package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type recordedLaunch struct {
	Dir  string
	Env  map[string]string
	Name string
	Args []string
}

func envSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}

func TestStartLocalIdentityOrderCartBff(t *testing.T) {
	root := t.TempDir()
	var got recordedLaunch
	launcher := func(dir string, env []string, name string, args ...string) int {
		got = recordedLaunch{
			Dir:  dir,
			Env:  envSliceToMap(env),
			Name: name,
			Args: args,
		}
		return 0
	}

	var out, errOut bytes.Buffer
	code := runWithDeps([]string{"-root", root, "-service", "identity-service"}, strings.NewReader(""), &out, &errOut, nil, launcher)
	if code != 0 {
		t.Fatalf("identity-service exit %d: %s", code, errOut.String())
	}
	if got.Name != "mvn" || !reflect.DeepEqual(got.Args, []string{"-f", "services/identity-service/pom.xml", "spring-boot:run"}) {
		t.Fatalf("unexpected identity command: %s %v", got.Name, got.Args)
	}
	if got.Env["SERVER_PORT"] != "8080" || got.Env["MANAGEMENT_SERVER_PORT"] != "8090" || got.Env["IDENTITY_GRPC_PORT"] != "50054" {
		t.Fatalf("unexpected identity env: %v", got.Env)
	}
	if !strings.Contains(got.Env["JDK_JAVA_OPTIONS"], "-Djava.net.preferIPv4Stack=true") {
		t.Fatalf("expected preferIPv4Stack in JDK_JAVA_OPTIONS, got %q", got.Env["JDK_JAVA_OPTIONS"])
	}

	// Order service
	code = runWithDeps([]string{"-root", root, "-service", "order"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("order-service exit %d", code)
	}
	if got.Name != "mvn" || !reflect.DeepEqual(got.Args, []string{"-f", "services/order-service/pom.xml", "spring-boot:run"}) {
		t.Fatalf("unexpected order command: %s %v", got.Name, got.Args)
	}
	if got.Env["SERVER_PORT"] != "8083" || got.Env["MANAGEMENT_SERVER_PORT"] != "8093" {
		t.Fatalf("unexpected order env: %v", got.Env)
	}

	// Cart service
	code = runWithDeps([]string{"-root", root, "-service", "cart-service"}, strings.NewReader(""), &out, &errOut, nil, launcher)
	if code != 0 {
		t.Fatalf("cart-service exit %d", code)
	}
	if got.Dir != filepath.Join(root, "services", "cart-service") {
		t.Fatalf("unexpected cart dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected cart command: %s %v", got.Name, got.Args)
	}
	if got.Env["HTTP_ADDR"] != ":8084" || got.Env["IDENTITY_GRPC_TIMEOUT"] != "500ms" {
		t.Fatalf("unexpected cart env: %v", got.Env)
	}

	// BFF service warns when .env is missing, then runs go run ./services/bff-service/cmd/server
	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "bff-service"}, strings.NewReader(""), &out, &errOut, nil, launcher)
	if code != 0 {
		t.Fatalf("bff-service exit %d", code)
	}
	if !strings.Contains(errOut.String(), "Local env file was not found") {
		t.Fatalf("expected missing .env warning, got %q", errOut.String())
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./services/bff-service/cmd/server"}) {
		t.Fatalf("unexpected bff command: %s %v", got.Name, got.Args)
	}

	// BFF service with existing .env sets ENV_FILE without warning
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("BFF_HTTP_ADDR=:8081\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "bff-service"}, strings.NewReader(""), &out, &errOut, nil, launcher)
	if code != 0 {
		t.Fatalf("bff-service with .env exit %d", code)
	}
	if errOut.String() != "" {
		t.Fatalf("expected no warning when .env exists, got %q", errOut.String())
	}
	if got.Env["ENV_FILE"] != envPath {
		t.Fatalf("expected ENV_FILE=%s, got %q", envPath, got.Env["ENV_FILE"])
	}
}

func TestStartLocalIdentityBootstrapSuperAdminValidation(t *testing.T) {
	root := t.TempDir()
	launcher := func(dir string, env []string, name string, args ...string) int { return 0 }

	var out, errOut bytes.Buffer
	// Missing full name fails
	code := runWithDeps([]string{"-root", root, "-service", "identity", "-bootstrap-super-admin-email", "admin@example.com"}, strings.NewReader(""), &out, &errOut, nil, launcher)
	if code != 1 || !strings.Contains(errOut.String(), "Bootstrap SUPER_ADMIN email and full name must be supplied together.") {
		t.Fatalf("expected missing full name failure, got code=%d err=%s", code, errOut.String())
	}

	// Short password from stdin fails
	errOut.Reset()
	code = runWithDeps(
		[]string{"-root", root, "-service", "identity", "-bootstrap-super-admin-email", "admin@example.com", "-bootstrap-super-admin-full-name", "Super Admin"},
		strings.NewReader("short\n"),
		&out, &errOut, nil, launcher,
	)
	if code != 1 || !strings.Contains(errOut.String(), "Bootstrap SUPER_ADMIN password must be 12 through 128 characters.") {
		t.Fatalf("expected password length failure, got code=%d err=%s", code, errOut.String())
	}

	// Valid password from stdin sets env vars
	var capturedEnv map[string]string
	launcherCapture := func(dir string, env []string, name string, args ...string) int {
		capturedEnv = envSliceToMap(env)
		return 0
	}
	errOut.Reset()
	_ = os.Unsetenv("IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD")
	code = runWithDeps(
		[]string{"-root", root, "-service", "identity", "-BootstrapSuperAdminEmail", "admin@example.com", "-BootstrapSuperAdminFullName", "Super Admin"},
		strings.NewReader("ValidSuperAdminPass123!\n"),
		&out, &errOut, nil, launcherCapture,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, errOut.String())
	}
	if capturedEnv["IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL"] != "admin@example.com" ||
		capturedEnv["IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME"] != "Super Admin" ||
		capturedEnv["IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD"] != "ValidSuperAdminPass123!" {
		t.Fatalf("unexpected bootstrap env: %v", capturedEnv)
	}
}
