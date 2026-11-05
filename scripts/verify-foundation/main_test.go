package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func populateFoundationFiles(t *testing.T, root string) {
	t.Helper()
	for _, rel := range requiredFoundationFiles {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVerifyFoundationPassesWithRunningContainers(t *testing.T) {
	root := t.TempDir()
	populateFoundationFiles(t, root)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/storefront/products", "/api/v1/auth/.well-known/jwks.json":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	var commands []string
	runner := func(dir, name string, args ...string) (string, error) {
		cmdStr := name + " " + strings.Join(args, " ")
		commands = append(commands, cmdStr)
		if strings.HasSuffix(cmdStr, "ps -q") {
			return "abc123container\n", nil
		}
		return "", nil
	}

	var out, errOut bytes.Buffer
	code := runWithDeps([]string{"-root", root, "-kong-url", srv.URL}, &out, &errOut, runner)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Checking public routes through Kong gateway...") {
		t.Fatalf("expected Kong route check log, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Foundation checks passed.") {
		t.Fatalf("expected pass message, got: %s", out.String())
	}
}

func TestVerifyFoundationFailsWhenFilesMissing(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	runner := func(dir, name string, args ...string) (string, error) {
		return "", nil
	}
	code := runWithDeps([]string{"-root", root}, &out, &errOut, runner)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Missing foundation files:") {
		t.Fatalf("expected missing foundation files error, got: %s", errOut.String())
	}
}

func TestVerifyFoundationRequiresCartService(t *testing.T) {
	foundFile := false
	for _, f := range requiredFoundationFiles {
		if f == "services/cart-service/go.mod" {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatalf("expected requiredFoundationFiles to contain services/cart-service/go.mod")
	}
}

func TestVerifyFoundationRequiresPaymentService(t *testing.T) {
	foundFile := false
	for _, f := range requiredFoundationFiles {
		if f == "services/payment-service/go.mod" {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatalf("expected requiredFoundationFiles to contain services/payment-service/go.mod")
	}
}
