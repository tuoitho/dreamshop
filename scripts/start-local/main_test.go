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
	code := runWithDeps([]string{"-root", root, "-service", "identity-service"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
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
	if got.Env["SERVER_PORT"] != "8083" ||
		got.Env["MANAGEMENT_SERVER_PORT"] != "8093" ||
		got.Env["ORDER_GRPC_PORT"] != "50055" ||
		got.Env["PAYMENT_GRPC_ADDR"] != "127.0.0.1:50056" {
		t.Fatalf("unexpected order env: %v", got.Env)
	}

	// Payment service
	code = runWithDeps([]string{"-root", root, "-service", "payment"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("payment-service exit %d: %s", code, errOut.String())
	}
	if got.Dir != filepath.Join(root, "services", "payment-service") {
		t.Fatalf("unexpected payment dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected payment command: %s %v", got.Name, got.Args)
	}
	if got.Env["HTTP_ADDR"] != ":8086" ||
		got.Env["GRPC_ADDR"] != "127.0.0.1:50056" ||
		got.Env["DATABASE_URL"] != "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/payment_db?sslmode=disable" ||
		got.Env["RABBITMQ_URL"] != "amqp://ecommerce:change-me-local-only@127.0.0.1:5672/" ||
		got.Env["MOMO_PARTNER_CODE"] != "MOMO_LOCAL" ||
		got.Env["MOMO_ACCESS_KEY"] != "local-access-key" ||
		got.Env["MOMO_SECRET_KEY"] != "local-secret-key-change-me" ||
		got.Env["PAYMENT_MOMO_MOCK_ENABLED"] != "true" ||
		got.Env["MOMO_REDIRECT_URL"] != "http://localhost:8000/api/v1/payments/momo-redirect" ||
		got.Env["MOMO_REFUND_ENDPOINT"] != "https://test-payment.momo.vn/v2/gateway/api/refund" ||
		got.Env["PAYMENT_REFUND_MAX_AUTO_ATTEMPTS"] != "3" {
		t.Fatalf("unexpected payment env: %v", got.Env)
	}

	// Cart service
	code = runWithDeps([]string{"-root", root, "-service", "cart-service"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("cart-service exit %d", code)
	}
	if got.Dir != filepath.Join(root, "services", "cart-service") {
		t.Fatalf("unexpected cart dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected cart command: %s %v", got.Name, got.Args)
	}
	if got.Env["HTTP_ADDR"] != ":8084" || got.Env["IDENTITY_GRPC_TIMEOUT"] != "500ms" || got.Env["RABBITMQ_URL"] != "amqp://ecommerce:change-me-local-only@127.0.0.1:5672/" {
		t.Fatalf("unexpected cart env: %v", got.Env)
	}

	// BFF service sets local defaults directly without setting ENV_FILE
	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "bff-service"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("bff-service exit %d: %s", code, errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("expected no stderr output for bff-service, got %q", errOut.String())
	}
	if got.Dir != filepath.Join(root, "services", "bff-service") {
		t.Fatalf("unexpected bff dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected bff command: %s %v", got.Name, got.Args)
	}
	if got.Env["BFF_HTTP_ADDR"] != ":8081" ||
		got.Env["BFF_CATALOG_GRPC_ADDR"] != "127.0.0.1:50051" ||
		got.Env["BFF_CATALOG_GRPC_TIMEOUT"] != "3s" ||
		got.Env["BFF_INVENTORY_GRPC_ADDR"] != "127.0.0.1:50052" ||
		got.Env["BFF_INVENTORY_GRPC_TIMEOUT"] != "3s" ||
		got.Env["BFF_CART_GRPC_ADDR"] != "127.0.0.1:50053" ||
		got.Env["BFF_CART_GRPC_TIMEOUT"] != "3s" ||
		got.Env["BFF_ORDER_GRPC_ADDR"] != "127.0.0.1:50055" ||
		got.Env["BFF_ORDER_GRPC_TIMEOUT"] != "15s" ||
		got.Env["BFF_PAYMENT_GRPC_ADDR"] != "127.0.0.1:50056" ||
		got.Env["BFF_PAYMENT_GRPC_TIMEOUT"] != "5s" ||
		got.Env["BFF_STOREFRONT_BASE_URL"] != "http://localhost:5173" {
		t.Fatalf("unexpected bff env: %v", got.Env)
	}
	if _, hasEnvFile := got.Env["ENV_FILE"]; hasEnvFile {
		t.Fatalf("expected BFF launch not to set ENV_FILE, got %q", got.Env["ENV_FILE"])
	}

	// Frontend (fe / ecommerce-fe) sets default VITE_API_BASE_URL and runs Vite dev server
	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "fe"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("fe exit %d: %s", code, errOut.String())
	}
	if got.Dir != root {
		t.Fatalf("unexpected fe dir: %s", got.Dir)
	}
	if got.Name != "npm" || !reflect.DeepEqual(got.Args, []string{"--prefix", "ecommerce-fe", "run", "dev"}) {
		t.Fatalf("unexpected fe command: %s %v", got.Name, got.Args)
	}
	if got.Env["VITE_API_BASE_URL"] != "http://localhost:8000/api/v1" {
		t.Fatalf("unexpected fe VITE_API_BASE_URL: %q", got.Env["VITE_API_BASE_URL"])
	}

	// Admin frontend (admin / admin-fe) sets default VITE_API_BASE_URL and runs Vite dev server
	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "admin-fe"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("admin-fe exit %d: %s", code, errOut.String())
	}
	if got.Dir != root {
		t.Fatalf("unexpected admin-fe dir: %s", got.Dir)
	}
	if got.Name != "npm" || !reflect.DeepEqual(got.Args, []string{"--prefix", "admin-fe", "run", "dev"}) {
		t.Fatalf("unexpected admin-fe command: %s %v", got.Name, got.Args)
	}
	if got.Env["VITE_API_BASE_URL"] != "http://localhost:8000/api/v1" {
		t.Fatalf("unexpected admin-fe VITE_API_BASE_URL: %q", got.Env["VITE_API_BASE_URL"])
	}
}

func TestStartLocalCatalogAndInventoryDefaults(t *testing.T) {
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
	code := runWithDeps([]string{"-root", root, "-service", "catalog"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("catalog exit %d: %s", code, errOut.String())
	}
	if got.Dir != filepath.Join(root, "services", "catalog-service") {
		t.Fatalf("unexpected catalog dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected catalog command: %s %v", got.Name, got.Args)
	}
	if got.Env["HTTP_ADDR"] != ":8085" ||
		got.Env["GRPC_ADDR"] != "127.0.0.1:50051" ||
		got.Env["DATABASE_URL"] != "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/catalog_db?sslmode=disable" ||
		got.Env["IDENTITY_GRPC_ADDR"] != "127.0.0.1:50054" ||
		got.Env["IDENTITY_GRPC_TIMEOUT"] != "500ms" ||
		got.Env["MINIO_ENDPOINT"] != "localhost:9000" ||
		got.Env["MINIO_ACCESS_KEY"] != "minioadmin" ||
		got.Env["MINIO_SECRET_KEY"] != "minioadmin" ||
		got.Env["MINIO_BUCKET"] != "product-media" ||
		got.Env["MINIO_PUBLIC_BASE_URL"] != "http://localhost:9000/product-media" ||
		got.Env["SEED_DEMO_DATA"] != "true" {
		t.Fatalf("unexpected catalog env: %v", got.Env)
	}

	errOut.Reset()
	code = runWithDeps([]string{"-root", root, "-service", "inventory"}, strings.NewReader(""), &out, &errOut, []string{}, launcher)
	if code != 0 {
		t.Fatalf("inventory exit %d: %s", code, errOut.String())
	}
	if got.Dir != filepath.Join(root, "services", "inventory-service") {
		t.Fatalf("unexpected inventory dir: %s", got.Dir)
	}
	if got.Name != "go" || !reflect.DeepEqual(got.Args, []string{"run", "./cmd/server"}) {
		t.Fatalf("unexpected inventory command: %s %v", got.Name, got.Args)
	}
	if got.Env["HTTP_ADDR"] != ":8082" ||
		got.Env["GRPC_ADDR"] != "127.0.0.1:50052" ||
		got.Env["DATABASE_URL"] != "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/inventory_db?sslmode=disable" ||
		got.Env["RABBITMQ_URL"] != "amqp://ecommerce:change-me-local-only@127.0.0.1:5672/" ||
		got.Env["IDENTITY_GRPC_ADDR"] != "127.0.0.1:50054" ||
		got.Env["IDENTITY_GRPC_TIMEOUT"] != "500ms" ||
		got.Env["SEED_DEMO_DATA"] != "true" {
		t.Fatalf("unexpected inventory env: %v", got.Env)
	}
}

func TestStartLocalLoadsRootDotenvWithProcessEnvPrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("# comment\nMOMO_ACCESS_KEY=dotenv-access\nMOMO_SECRET_KEY=\"dotenv-secret\"\nHTTP_ADDR=:18086\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	var got recordedLaunch
	launcher := func(dir string, env []string, name string, args ...string) int {
		got = recordedLaunch{Dir: dir, Env: envSliceToMap(env), Name: name, Args: args}
		return 0
	}
	var out, errOut bytes.Buffer
	code := runWithDeps([]string{"-root", root, "-service", "payment-service"}, strings.NewReader(""), &out, &errOut, []string{"MOMO_ACCESS_KEY=caller-access"}, launcher)
	if code != 0 {
		t.Fatalf("payment-service with .env exit %d: %s", code, errOut.String())
	}
	if got.Env["MOMO_ACCESS_KEY"] != "caller-access" || got.Env["MOMO_SECRET_KEY"] != "dotenv-secret" || got.Env["HTTP_ADDR"] != ":18086" {
		t.Fatalf("unexpected .env precedence result: %v", got.Env)
	}
}

func TestStartLocalRespectsEnvOverrides(t *testing.T) {
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
	idEnv := []string{
		"SERVER_PORT=18080",
		"IDENTITY_JWT_ISSUER=http://localhost:18080",
	}
	code := runWithDeps([]string{"-root", root, "-service", "identity"}, strings.NewReader(""), &out, &errOut, idEnv, launcher)
	if code != 0 {
		t.Fatalf("identity override exit %d: %s", code, errOut.String())
	}
	if got.Env["SERVER_PORT"] != "18080" || got.Env["IDENTITY_JWT_ISSUER"] != "http://localhost:18080" {
		t.Fatalf("expected identity env overrides to be preserved, got %v", got.Env)
	}

	errOut.Reset()
	customEnv := []string{
		"HTTP_ADDR=:9085",
		"DATABASE_URL=postgres://custom:secret@127.0.0.1:5432/catalog_db?sslmode=disable",
	}
	code = runWithDeps([]string{"-root", root, "-service", "catalog-service"}, strings.NewReader(""), &out, &errOut, customEnv, launcher)
	if code != 0 {
		t.Fatalf("catalog-service override exit %d: %s", code, errOut.String())
	}
	if got.Env["HTTP_ADDR"] != ":9085" || got.Env["DATABASE_URL"] != "postgres://custom:secret@127.0.0.1:5432/catalog_db?sslmode=disable" {
		t.Fatalf("expected catalog env overrides to be preserved, got %v", got.Env)
	}

	errOut.Reset()
	invEnv := []string{
		"HTTP_ADDR=:9082",
		"DATABASE_URL=postgres://custom:secret@127.0.0.1:5432/inventory_db?sslmode=disable",
	}
	code = runWithDeps([]string{"-root", root, "-service", "inventory-service"}, strings.NewReader(""), &out, &errOut, invEnv, launcher)
	if code != 0 {
		t.Fatalf("inventory-service override exit %d: %s", code, errOut.String())
	}
	if got.Env["HTTP_ADDR"] != ":9082" || got.Env["DATABASE_URL"] != "postgres://custom:secret@127.0.0.1:5432/inventory_db?sslmode=disable" {
		t.Fatalf("expected inventory env overrides to be preserved, got %v", got.Env)
	}

	errOut.Reset()
	bffEnv := []string{"BFF_HTTP_ADDR=:9081"}
	code = runWithDeps([]string{"-root", root, "-service", "bff"}, strings.NewReader(""), &out, &errOut, bffEnv, launcher)
	if code != 0 {
		t.Fatalf("bff override exit %d: %s", code, errOut.String())
	}
	if got.Env["BFF_HTTP_ADDR"] != ":9081" {
		t.Fatalf("expected BFF_HTTP_ADDR override :9081, got %q", got.Env["BFF_HTTP_ADDR"])
	}

	errOut.Reset()
	feEnv := []string{"VITE_API_BASE_URL=http://localhost:9000/api/v1"}
	for _, svcAlias := range []string{"ecommerce-fe", "frontend"} {
		code = runWithDeps([]string{"-root", root, "-service", svcAlias}, strings.NewReader(""), &out, &errOut, feEnv, launcher)
		if code != 0 {
			t.Fatalf("%s override exit %d: %s", svcAlias, code, errOut.String())
		}
		if got.Env["VITE_API_BASE_URL"] != "http://localhost:9000/api/v1" {
			t.Fatalf("expected %s VITE_API_BASE_URL override, got %q", svcAlias, got.Env["VITE_API_BASE_URL"])
		}
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
