package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"ecommerce.local/scripts/internal/boundaries"
	"ecommerce.local/scripts/internal/repo"
)

type execFunc func(dir, name string, args ...string) (string, int)

func main() {
	repo.EnsureGoBinInPath()
	os.Exit(runWithDeps(os.Args[1:], os.Stdout, runtime.GOOS, defaultExec(os.Stdout)))
}

func defaultExec(out io.Writer) execFunc {
	return func(dir, name string, args ...string) (string, int) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		var captured bytes.Buffer
		mw := io.MultiWriter(out, &captured)
		cmd.Stdout = mw
		cmd.Stderr = mw
		err := cmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return captured.String(), exitErr.ExitCode()
			}
			return captured.String(), 1
		}
		return captured.String(), 0
	}
}

func runWithDeps(args []string, out io.Writer, goos string, runCmd execFunc) int {
	fs := flag.NewFlagSet("verify-service", flag.ContinueOnError)
	fs.SetOutput(out)
	var service, baseBranch, root string
	fs.StringVar(&service, "service", "", "surface to verify")
	fs.StringVar(&service, "Service", "", "surface to verify")
	fs.StringVar(&baseBranch, "base-branch", "main", "base branch for buf breaking")
	fs.StringVar(&baseBranch, "BaseBranch", "main", "base branch for buf breaking")
	fs.StringVar(&root, "root", "", "repository root directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if service == "" && fs.NArg() > 0 {
		service = fs.Arg(0)
	}
	if service == "" {
		fmt.Fprintln(out, "missing required -service flag")
		return 2
	}

	if root == "" {
		r, err := repo.Root()
		if err != nil {
			fmt.Fprintf(out, "error locating repository root: %v\n", err)
			return 1
		}
		root = r
	}

	fmt.Fprintln(out, "==> boundary check")
	if code := boundaries.Check(root, out); code != 0 {
		fmt.Fprintf(out, "FAILED: boundary check (exit %d)\n", code)
		return code
	}

	step := func(name, bin string, cmdArgs ...string) int {
		fmt.Fprintf(out, "==> %s\n", name)
		_, code := runCmd(root, bin, cmdArgs...)
		if code != 0 {
			fmt.Fprintf(out, "FAILED: %s (exit %d)\n", name, code)
			return code
		}
		return 0
	}

	serviceRel := "services/" + service
	serviceAbs := filepath.Join(root, "services", service)

	switch {
	case service == "contracts":
		if goos == "windows" {
			ensureGitUsrBinInPath()
		}
		gitDirOut, code := runCmd(root, "git", "rev-parse", "--git-common-dir")
		if code != 0 {
			fmt.Fprintf(out, "FAILED: git rev-parse (exit %d)\n", code)
			return code
		}
		gitDir := strings.TrimSpace(gitDirOut)

		if c := step("buf format", "buf", "format", "--diff", "--exit-code"); c != 0 {
			return c
		}
		if c := step("buf lint", "buf", "lint"); c != 0 {
			return c
		}
		if c := step("buf breaking against "+baseBranch, "buf", "breaking", "--against", fmt.Sprintf("%s#branch=%s", gitDir, baseBranch)); c != 0 {
			return c
		}
		if c := step("buf generate", "buf", "generate"); c != 0 {
			return c
		}
		if c := step("generated output is synchronized", "git", "diff", "--exit-code", "--", "contracts/go", "contracts/java/src/main/generated"); c != 0 {
			return c
		}
		untracked, code := runCmd(root, "git", "ls-files", "--others", "--exclude-standard", "--", "contracts/go", "contracts/java/src/main/generated")
		if code != 0 {
			return code
		}
		if strings.TrimSpace(untracked) != "" {
			fmt.Fprintln(out, "FAILED: generated contract files are not tracked")
			return 1
		}

	case service == "ecommerce-fe":
		if c := step("frontend tests", "npm", "--prefix", "ecommerce-fe", "test", "--", "--run"); c != 0 {
			return c
		}
		if c := step("frontend lint", "npm", "--prefix", "ecommerce-fe", "run", "lint"); c != 0 {
			return c
		}
		if c := step("frontend build", "npm", "--prefix", "ecommerce-fe", "run", "build"); c != 0 {
			return c
		}

	case service == "admin-fe":
		if c := step("admin frontend tests", "npm", "--prefix", "admin-fe", "test", "--", "--run"); c != 0 {
			return c
		}
		if c := step("admin frontend lint", "npm", "--prefix", "admin-fe", "run", "lint"); c != 0 {
			return c
		}
		if c := step("admin frontend build", "npm", "--prefix", "admin-fe", "run", "build"); c != 0 {
			return c
		}

	case fileExists(filepath.Join(serviceAbs, "pom.xml")):
		if c := step("maven verify "+service, "mvn", "--batch-mode", "--no-transfer-progress", "-pl", serviceRel, "-am", "verify"); c != 0 {
			return c
		}

	case fileExists(filepath.Join(serviceAbs, "go.mod")):
		testScope := "./" + serviceRel + "/..."
		if goos == "windows" && service == "cart-service" {
			testScope = "./" + serviceRel + "/internal/..."
			fmt.Fprintf(out, "NOTE: Windows runs %s only; cart-service cmd/server tests run on Linux/CI.\n", testScope)
		}
		if c := step("go build "+service, "go", "build", "./"+serviceRel+"/..."); c != 0 {
			return c
		}
		if c := step("go test "+service, "go", "test", testScope); c != 0 {
			return c
		}

	default:
		surfaces := listSurfaces(root)
		fmt.Fprintf(out, "Unknown surface '%s'. Expected one of: %s\n", service, strings.Join(surfaces, ", "))
		return 2
	}

	fmt.Fprintf(out, "PASSED: %s\n", service)
	return 0
}

func ensureGitUsrBinInPath() {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return
	}
	gitRoot := filepath.Dir(filepath.Dir(gitPath))
	usrBin := filepath.Join(gitRoot, "usr", "bin")
	if info, err := os.Stat(usrBin); err == nil && info.IsDir() {
		_ = os.Setenv("PATH", usrBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
}

func listSurfaces(root string) []string {
	entries, _ := os.ReadDir(filepath.Join(root, "services"))
	var surfaces []string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), "-service") {
			surfaces = append(surfaces, e.Name())
		}
	}
	sort.Strings(surfaces)
	return append(surfaces, "ecommerce-fe", "admin-fe", "contracts")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
