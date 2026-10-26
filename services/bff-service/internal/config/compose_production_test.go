package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeManifest struct {
	Name     string                           `yaml:"name"`
	Networks map[string]map[string]any        `yaml:"networks"`
	Volumes  map[string]any                   `yaml:"volumes"`
	Services map[string]composeServiceSection `yaml:"services"`
}

type composeServiceSection struct {
	Restart     string                            `yaml:"restart"`
	Ports       []string                          `yaml:"ports"`
	Networks    []string                          `yaml:"networks"`
	Volumes     []string                          `yaml:"volumes"`
	Environment map[string]string                 `yaml:"environment"`
	DependsOn   map[string]composeDependsOnConfig `yaml:"depends_on"`
	Healthcheck map[string]any                    `yaml:"healthcheck"`
}

type composeDependsOnConfig struct {
	Condition string `yaml:"condition"`
}

func assertHealthyDeps(t *testing.T, svcName string, deps map[string]composeDependsOnConfig, expected ...string) {
	t.Helper()
	for _, dep := range expected {
		cfg, ok := deps[dep]
		if !ok {
			t.Errorf("%s missing depends_on %q", svcName, dep)
			continue
		}
		if cfg.Condition != "service_healthy" {
			t.Errorf("%s depends_on %q condition = %q, want service_healthy", svcName, dep, cfg.Condition)
		}
	}
}

func TestProductionComposeEnforcesIsolationSecretsAndStartupOrder(t *testing.T) {
	root := repoRoot(t)
	raw := readRepoFile(t, root, "deployments", "docker", "compose.production.yml")

	var compose composeManifest
	if err := yaml.Unmarshal([]byte(raw), &compose); err != nil {
		t.Fatalf("parse compose.production.yml: %v", err)
	}

	if compose.Name != "ecommerce-prod" {
		t.Errorf("compose name = %q, want ecommerce-prod", compose.Name)
	}
	if _, ok := compose.Networks["ecommerce-prod-net"]; !ok {
		t.Error("compose missing ecommerce-prod-net network")
	}
	for _, vol := range []string{"postgres_data", "redis_data", "rabbitmq_data"} {
		if _, ok := compose.Volumes[vol]; !ok {
			t.Errorf("compose missing named volume %q", vol)
		}
	}

	expectedServices := []string{
		"postgres",
		"redis",
		"rabbitmq",
		"kong",
		"identity-service",
		"bff-service",
		"catalog-service",
		"inventory-service",
		"cart-service",
		"order-service",
		"ecommerce-fe",
	}

	for _, name := range expectedServices {
		svc, ok := compose.Services[name]
		if !ok {
			t.Errorf("compose missing service %q", name)
			continue
		}
		if svc.Restart != "unless-stopped" {
			t.Errorf("service %q restart = %q, want unless-stopped", name, svc.Restart)
		}
		if !slices.Contains(svc.Networks, "ecommerce-prod-net") {
			t.Errorf("service %q must attach to ecommerce-prod-net", name)
		}
		if len(svc.Healthcheck) == 0 {
			t.Errorf("service %q missing healthcheck", name)
		}
		if name == "kong" {
			if len(svc.Ports) != 1 || svc.Ports[0] != "${PUBLIC_HTTP_PORT:-80}:8000" {
				t.Errorf("kong ports = %v, want [${PUBLIC_HTTP_PORT:-80}:8000]", svc.Ports)
			}
		} else if len(svc.Ports) > 0 {
			t.Errorf("internal service %q must not publish host ports, got %v", name, svc.Ports)
		}
	}

	kong := compose.Services["kong"]
	if kong.Environment["KONG_ADMIN_LISTEN"] != "127.0.0.1:8001" {
		t.Errorf("kong KONG_ADMIN_LISTEN = %q, want 127.0.0.1:8001", kong.Environment["KONG_ADMIN_LISTEN"])
	}
	if !slices.Contains(kong.Volumes, "../kong/kong-production.yaml:/usr/local/kong/declarative/kong.yml:ro") {
		t.Errorf("kong volumes = %v, missing kong-production.yaml read-only mount", kong.Volumes)
	}
	assertHealthyDeps(t, "kong", kong.DependsOn, "identity-service", "bff-service", "ecommerce-fe")

	if !strings.HasPrefix(compose.Services["postgres"].Environment["POSTGRES_PASSWORD"], "${POSTGRES_PASSWORD:?") {
		t.Errorf("postgres POSTGRES_PASSWORD must fail fast via :?, got %q", compose.Services["postgres"].Environment["POSTGRES_PASSWORD"])
	}
	if !strings.HasPrefix(compose.Services["rabbitmq"].Environment["RABBITMQ_DEFAULT_PASS"], "${RABBITMQ_DEFAULT_PASS:?") {
		t.Errorf("rabbitmq RABBITMQ_DEFAULT_PASS must fail fast via :?, got %q", compose.Services["rabbitmq"].Environment["RABBITMQ_DEFAULT_PASS"])
	}

	identity := compose.Services["identity-service"]
	if identity.Environment["IDENTITY_GRPC_HOST"] != "0.0.0.0" {
		t.Errorf("identity-service IDENTITY_GRPC_HOST = %q, want 0.0.0.0", identity.Environment["IDENTITY_GRPC_HOST"])
	}
	if identity.Environment["IDENTITY_JWT_ISSUER"] != "http://identity-service:8080" {
		t.Errorf("identity-service IDENTITY_JWT_ISSUER = %q, want http://identity-service:8080", identity.Environment["IDENTITY_JWT_ISSUER"])
	}
	if identity.Environment["IDENTITY_SESSION_COOKIE_SECURE"] != "${IDENTITY_SESSION_COOKIE_SECURE:-true}" {
		t.Errorf("identity-service IDENTITY_SESSION_COOKIE_SECURE = %q, want ${IDENTITY_SESSION_COOKIE_SECURE:-true}", identity.Environment["IDENTITY_SESSION_COOKIE_SECURE"])
	}
	for _, bootstrapKey := range []string{
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL",
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD",
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME",
	} {
		expected := "${" + bootstrapKey + ":-}"
		if identity.Environment[bootstrapKey] != expected {
			t.Errorf("identity-service %s = %q, want %q", bootstrapKey, identity.Environment[bootstrapKey], expected)
		}
	}
	assertHealthyDeps(t, "identity-service", identity.DependsOn, "postgres")

	if compose.Services["catalog-service"].Environment["GRPC_ADDR"] != "0.0.0.0:50051" {
		t.Errorf("catalog-service GRPC_ADDR = %q, want 0.0.0.0:50051", compose.Services["catalog-service"].Environment["GRPC_ADDR"])
	}
	if compose.Services["inventory-service"].Environment["GRPC_ADDR"] != "0.0.0.0:50052" {
		t.Errorf("inventory-service GRPC_ADDR = %q, want 0.0.0.0:50052", compose.Services["inventory-service"].Environment["GRPC_ADDR"])
	}

	cart := compose.Services["cart-service"]
	if cart.Environment["GRPC_ADDR"] != "0.0.0.0:50053" {
		t.Errorf("cart-service GRPC_ADDR = %q, want 0.0.0.0:50053", cart.Environment["GRPC_ADDR"])
	}
	assertHealthyDeps(t, "cart-service", cart.DependsOn, "postgres", "identity-service", "catalog-service", "inventory-service")

	bff := compose.Services["bff-service"]
	if bff.Environment["BFF_CATALOG_GRPC_ADDR"] != "catalog-service:50051" || bff.Environment["BFF_CART_GRPC_ADDR"] != "cart-service:50053" {
		t.Errorf("bff-service gRPC targets = catalog=%q cart=%q", bff.Environment["BFF_CATALOG_GRPC_ADDR"], bff.Environment["BFF_CART_GRPC_ADDR"])
	}
	assertHealthyDeps(t, "bff-service", bff.DependsOn, "catalog-service", "cart-service")

	envExample := readRepoFile(t, root, ".env.production.example")
	for _, key := range []string{
		"PUBLIC_HTTP_PORT=",
		"POSTGRES_USER=",
		"POSTGRES_PASSWORD=",
		"RABBITMQ_DEFAULT_USER=",
		"RABBITMQ_DEFAULT_PASS=",
		"IDENTITY_KEYS_DIR=",
		"IDENTITY_SESSION_COOKIE_SECURE=true",
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL=",
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD=",
		"IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME=",
	} {
		if !strings.Contains(envExample, key) {
			t.Errorf(".env.production.example missing %q", key)
		}
	}
	if strings.Contains(envExample, "change-me-local-only") || strings.Contains(envExample, "AdminPassword123!") {
		t.Error(".env.production.example must not contain local fallback passwords")
	}
}

func TestProductionRunbookDocumentsSecretDirectoryPermissionsAndTLSHeaders(t *testing.T) {
	root := repoRoot(t)
	runbook := readRepoFile(t, root, "docs", "instructions", "production-deployment.md")
	for _, required := range []string{
		"chmod 0755 /etc/ecommerce",
		"X-Forwarded-Proto",
	} {
		if !strings.Contains(runbook, required) {
			t.Errorf("production-deployment.md missing %q", required)
		}
	}
}
