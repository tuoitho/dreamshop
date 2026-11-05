package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ecommerce.local/scripts/internal/repo"
)

var requiredFoundationFiles = []string{
	"buf.yaml",
	"buf.gen.yaml",
	"deployments/docker/compose.foundation.yml",
	"deployments/kong/kong.yaml",
	"contracts/proto/common/v1/common.proto",
	"services/identity-service/pom.xml",
	"services/order-service/pom.xml",
	"services/bff-service/go.mod",
	"services/catalog-service/go.mod",
	"services/inventory-service/go.mod",
	"services/cart-service/go.mod",
	"services/payment-service/go.mod",
}

type cmdRunner func(dir, name string, args ...string) (string, error)

func main() {
	repo.EnsureGoBinInPath()
	os.Exit(runWithDeps(os.Args[1:], os.Stdout, os.Stderr, defaultRunner))
}

func defaultRunner(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func runWithDeps(args []string, out, errOut io.Writer, runner cmdRunner) int {
	fs := flag.NewFlagSet("verify-foundation", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var root, kongURL string
	fs.StringVar(&root, "root", "", "repository root directory")
	fs.StringVar(&kongURL, "kong-url", "http://localhost:8000", "Kong gateway base URL")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if root == "" {
		r, err := repo.Root()
		if err != nil {
			fmt.Fprintf(errOut, "error locating repository root: %v\n", err)
			return 1
		}
		root = r
	}

	var missing []string
	for _, rel := range requiredFoundationFiles {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(full); err != nil {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(errOut, "Missing foundation files: %s\n", strings.Join(missing, ", "))
		return 1
	}

	if cmdOut, err := runner(root, "docker", "compose", "-f", "deployments/docker/compose.foundation.yml", "config", "--quiet"); err != nil {
		if cmdOut != "" {
			fmt.Fprint(errOut, cmdOut)
		}
		fmt.Fprintf(errOut, "docker compose config failed: %v\n", err)
		return 1
	}

	if cmdOut, err := runner(root, "buf", "lint"); err != nil {
		if cmdOut != "" {
			fmt.Fprint(errOut, cmdOut)
		}
		fmt.Fprintf(errOut, "buf lint failed: %v\n", err)
		return 1
	}

	containers, err := runner(root, "docker", "compose", "-f", "deployments/docker/compose.foundation.yml", "ps", "-q")
	if err == nil && strings.TrimSpace(containers) != "" {
		fmt.Fprintln(out, "Checking public routes through Kong gateway...")
		client := &http.Client{Timeout: 5 * time.Second}
		base := strings.TrimRight(kongURL, "/")

		if err := checkPublicRoute(client, base+"/api/v1/storefront/products", "BFF"); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := checkPublicRoute(client, base+"/api/v1/auth/.well-known/jwks.json", "Identity"); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
	}

	fmt.Fprintln(out, "Foundation checks passed.")
	return 0
}

func checkPublicRoute(client *http.Client, url, target string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to reach %s through Kong: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to reach %s through Kong: %s public route returned status %d", target, target, resp.StatusCode)
	}
	return nil
}
