package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type kongProductionConfig struct {
	Services []struct {
		Name   string `yaml:"name"`
		URL    string `yaml:"url"`
		Routes []struct {
			Name      string   `yaml:"name"`
			Paths     []string `yaml:"paths"`
			StripPath bool     `yaml:"strip_path"`
			Plugins   []struct {
				Name   string `yaml:"name"`
				Config struct {
					JWKSURI string `yaml:"jwks_uri"`
					Issuer  string `yaml:"issuer"`
					Audience string `yaml:"audience"`
				} `yaml:"config"`
			} `yaml:"plugins"`
		} `yaml:"routes"`
	} `yaml:"services"`
}

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default HTTPAddr :8080, got %s", cfg.HTTPAddr)
	}
	if cfg.CatalogGRPCAddr != "127.0.0.1:50051" {
		t.Fatalf("expected default CatalogGRPCAddr 127.0.0.1:50051, got %s", cfg.CatalogGRPCAddr)
	}
	if cfg.CatalogGRPCTimeout != 3*time.Second {
		t.Fatalf("expected default CatalogGRPCTimeout 3s, got %s", cfg.CatalogGRPCTimeout)
	}
	if cfg.CartGRPCAddr != "127.0.0.1:50053" {
		t.Fatalf("expected default CartGRPCAddr 127.0.0.1:50053, got %s", cfg.CartGRPCAddr)
	}
	if cfg.CartGRPCTimeout != 3*time.Second {
		t.Fatalf("expected default CartGRPCTimeout 3s, got %s", cfg.CartGRPCTimeout)
	}
}

func TestLoadUsesPrefixedEnvironment(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", ":9080")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "catalog.local:50051")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "5s")
	t.Setenv("BFF_CART_GRPC_ADDR", "cart.local:50053")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "7s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9080" {
		t.Fatalf("expected BFF_HTTP_ADDR :9080, got %s", cfg.HTTPAddr)
	}
	if cfg.CatalogGRPCAddr != "catalog.local:50051" {
		t.Fatalf("expected BFF_CATALOG_GRPC_ADDR catalog.local:50051, got %s", cfg.CatalogGRPCAddr)
	}
	if cfg.CatalogGRPCTimeout != 5*time.Second {
		t.Fatalf("expected BFF_CATALOG_GRPC_TIMEOUT 5s, got %s", cfg.CatalogGRPCTimeout)
	}
	if cfg.CartGRPCAddr != "cart.local:50053" {
		t.Fatalf("expected BFF_CART_GRPC_ADDR cart.local:50053, got %s", cfg.CartGRPCAddr)
	}
	if cfg.CartGRPCTimeout != 7*time.Second {
		t.Fatalf("expected BFF_CART_GRPC_TIMEOUT 7s, got %s", cfg.CartGRPCTimeout)
	}
}

func TestLoadKeepsLegacyHTTPAddrFallback(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected legacy HTTP_ADDR :9090, got %s", cfg.HTTPAddr)
	}
}

func TestLoadRejectsNonPositiveCatalogTimeout(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "0s")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive Catalog gRPC timeout, got nil")
	}
}

func TestLoadRejectsBlankCartAddress(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_CART_GRPC_ADDR", "   ")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for blank Cart gRPC address, got nil")
	}
}

func TestLoadRejectsNonPositiveCartTimeout(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive Cart gRPC timeout, got nil")
	}
}

func TestLoadReadsDotenvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("BFF_HTTP_ADDR=:9180\nBFF_CATALOG_GRPC_ADDR=localhost:50051\nBFF_CATALOG_GRPC_TIMEOUT=2s\nBFF_CART_GRPC_ADDR=localhost:50053\nBFF_CART_GRPC_TIMEOUT=4s\n"), 0o600); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}

	for _, key := range []string{"BFF_HTTP_ADDR", "HTTP_ADDR", "BFF_CATALOG_GRPC_ADDR", "BFF_CATALOG_GRPC_TIMEOUT", "BFF_CART_GRPC_ADDR", "BFF_CART_GRPC_TIMEOUT"} {
		value, exists := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	t.Setenv("ENV_FILE", envFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9180" {
		t.Fatalf("expected dotenv HTTP address :9180, got %s", cfg.HTTPAddr)
	}
	if cfg.CatalogGRPCAddr != "localhost:50051" {
		t.Fatalf("expected dotenv Catalog gRPC address localhost:50051, got %s", cfg.CatalogGRPCAddr)
	}
	if cfg.CatalogGRPCTimeout != 2*time.Second {
		t.Fatalf("expected dotenv Catalog gRPC timeout 2s, got %s", cfg.CatalogGRPCTimeout)
	}
	if cfg.CartGRPCAddr != "localhost:50053" {
		t.Fatalf("expected dotenv Cart gRPC address localhost:50053, got %s", cfg.CartGRPCAddr)
	}
	if cfg.CartGRPCTimeout != 4*time.Second {
		t.Fatalf("expected dotenv Cart gRPC timeout 4s, got %s", cfg.CartGRPCTimeout)
	}
}

func TestProductionKongProtectsCartWithProductionCallerContext(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	configPath := filepath.Join(root, "deployments", "kong", "kong-production.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read production Kong config: %v", err)
	}
	var config kongProductionConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		t.Fatalf("parse production Kong config: %v", err)
	}

	var cartRouteFound bool
	for _, service := range config.Services {
		if service.Name != "bff-service" || service.URL != "http://bff-service:8080" {
			continue
		}
		for _, route := range service.Routes {
			if route.Name != "cart-api" {
				continue
			}
			cartRouteFound = true
			if len(route.Paths) != 1 || route.Paths[0] != "/api/v1/cart" || !route.StripPath {
				t.Fatalf("production cart route path/strip_path = %#v/%t", route.Paths, route.StripPath)
			}
			if len(route.Plugins) != 1 || route.Plugins[0].Name != "caller-context" {
				t.Fatalf("production cart route plugins = %#v, want caller-context only", route.Plugins)
			}
			callerContext := route.Plugins[0].Config
			if callerContext.JWKSURI != "http://identity-service:8080/.well-known/jwks.json" || callerContext.Issuer != "http://identity-service:8080" || callerContext.Audience != "ecommerce-api" {
				t.Fatalf("production cart caller-context config = %#v", callerContext)
			}
		}
	}
	if !cartRouteFound {
		t.Fatal("production Kong config is missing the protected cart-api route on bff-service")
	}
	if strings.Contains(string(content), "name: admin-products-api") {
		t.Fatal("unauthenticated development admin route must not be present in production")
	}
}

func TestBFFContainerDeclaresProductionCartGRPCTarget(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	dockerfile, err := os.ReadFile(filepath.Join(root, "services", "bff-service", "Dockerfile"))
	if err != nil {
		t.Fatalf("read BFF Dockerfile: %v", err)
	}
	content := string(dockerfile)
	if !strings.Contains(content, `ENV BFF_CART_GRPC_ADDR="cart-service:50053"`) {
		t.Fatal("BFF production image does not declare the Cart gRPC service DNS target")
	}
	if !strings.Contains(content, `ENV BFF_CART_GRPC_TIMEOUT="3s"`) {
		t.Fatal("BFF production image does not declare the Cart gRPC timeout")
	}
}
