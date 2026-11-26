package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func createSurfaceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"services/identity-service/pom.xml",
		"services/bff-service/go.mod",
		"services/cart-service/go.mod",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestVerifyServiceGoOnWindowsAndLinux(t *testing.T) {
	root := createSurfaceRepo(t)

	var recorded []string
	execMock := func(dir, name string, args ...string) (string, int) {
		recorded = append(recorded, name+" "+strings.Join(args, " "))
		return "", 0
	}

	var out bytes.Buffer
	code := runWithDeps([]string{"-root", root, "-service", "cart-service"}, &out, "windows", execMock)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; out:\n%s", code, out.String())
	}
	wantWin := []string{
		"go build ./services/cart-service/...",
		"go test ./services/cart-service/internal/...",
	}
	if !reflect.DeepEqual(recorded, wantWin) {
		t.Fatalf("windows commands = %v, want %v", recorded, wantWin)
	}
	if !strings.Contains(out.String(), "PASSED: cart-service") {
		t.Fatalf("expected PASSED output, got:\n%s", out.String())
	}

	recorded = nil
	out.Reset()
	code = runWithDeps([]string{"-root", root, "-Service", "cart-service"}, &out, "linux", execMock)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	wantLinux := []string{
		"go build ./services/cart-service/...",
		"go test ./services/cart-service/...",
	}
	if !reflect.DeepEqual(recorded, wantLinux) {
		t.Fatalf("linux commands = %v, want %v", recorded, wantLinux)
	}
}

func TestVerifyServiceJavaAndFrontendAndContracts(t *testing.T) {
	root := createSurfaceRepo(t)

	var recorded []string
	execMock := func(dir, name string, args ...string) (string, int) {
		cmd := name + " " + strings.Join(args, " ")
		recorded = append(recorded, cmd)
		if cmd == "git rev-parse --git-common-dir" {
			return ".git\n", 0
		}
		return "", 0
	}

	var out bytes.Buffer
	if code := runWithDeps([]string{"-root", root, "-service", "identity-service"}, &out, "linux", execMock); code != 0 {
		t.Fatalf("identity-service failed: %d", code)
	}
	if code := runWithDeps([]string{"-root", root, "-service", "ecommerce-fe"}, &out, "linux", execMock); code != 0 {
		t.Fatalf("ecommerce-fe failed: %d", code)
	}
	if code := runWithDeps([]string{"-root", root, "-service", "admin-fe"}, &out, "linux", execMock); code != 0 {
		t.Fatalf("admin-fe failed: %d; out:\n%s", code, out.String())
	}
	if code := runWithDeps([]string{"-root", root, "-service", "e2e"}, &out, "linux", execMock); code != 0 {
		t.Fatalf("e2e failed: %d; out:\n%s", code, out.String())
	}
	if code := runWithDeps([]string{"-root", root, "-service", "contracts"}, &out, "linux", execMock); code != 0 {
		t.Fatalf("contracts failed: %d", code)
	}
	if code := runWithDeps([]string{"-root", root, "-service", "contracts", "-base-branch", "develop"}, &out, "linux", execMock); code == 0 {
		t.Fatal("expected removed -base-branch flag to fail flag parsing")
	}

	want := []string{
		"mvn --batch-mode --no-transfer-progress -pl services/identity-service -am verify",
		"npm --prefix ecommerce-fe test -- --run",
		"npm --prefix ecommerce-fe run lint",
		"npm --prefix ecommerce-fe run build",
		"npm --prefix admin-fe test -- --run",
		"npm --prefix admin-fe run lint",
		"npm --prefix admin-fe run build",
		"npm --prefix e2e run typecheck",
		"npm --prefix e2e run test:mode2",
		"git rev-parse --git-common-dir",
		"buf format --diff --exit-code",
		"buf lint",
		"buf breaking --against .git#branch=main",
		"buf generate",
		"git diff --exit-code -- contracts/go contracts/java/src/main/generated",
		"git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated",
	}
	if !reflect.DeepEqual(recorded, want) {
		t.Fatalf("commands =\n%v\nwant:\n%v", recorded, want)
	}
}

func TestVerifyServiceUnknownSurfaceExits2(t *testing.T) {
	root := createSurfaceRepo(t)
	var out bytes.Buffer
	execMock := func(dir, name string, args ...string) (string, int) { return "", 0 }

	code := runWithDeps([]string{"-root", root, "-service", "nonexistent"}, &out, "linux", execMock)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; out:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "Unknown surface 'nonexistent'.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
