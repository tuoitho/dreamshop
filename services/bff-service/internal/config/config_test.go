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
	t.Setenv("BFF_INVENTORY_GRPC_ADDR", "")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", "")
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
	if cfg.InventoryGRPCAddr != "127.0.0.1:50052" {
		t.Fatalf("expected default InventoryGRPCAddr 127.0.0.1:50052, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.InventoryGRPCTimeout != 3*time.Second {
		t.Fatalf("expected default InventoryGRPCTimeout 3s, got %s", cfg.InventoryGRPCTimeout)
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
	t.Setenv("BFF_INVENTORY_GRPC_ADDR", "inventory.local:50052")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", "4s")
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
	if cfg.InventoryGRPCAddr != "inventory.local:50052" {
		t.Fatalf("expected BFF_INVENTORY_GRPC_ADDR inventory.local:50052, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.InventoryGRPCTimeout != 4*time.Second {
		t.Fatalf("expected BFF_INVENTORY_GRPC_TIMEOUT 4s, got %s", cfg.InventoryGRPCTimeout)
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

func TestLoadRejectsBlankInventoryAddress(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_INVENTORY_GRPC_ADDR", "   ")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", "")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for blank Inventory gRPC address, got nil")
	}
}

func TestLoadRejectsNonPositiveInventoryTimeout(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")
	t.Setenv("BFF_INVENTORY_GRPC_ADDR", "")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", "0s")
	t.Setenv("BFF_CART_GRPC_ADDR", "")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive Inventory gRPC timeout, got nil")
	}
}

func clearBFFConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"BFF_HTTP_ADDR",
		"HTTP_ADDR",
		"BFF_CATALOG_GRPC_ADDR",
		"BFF_CATALOG_GRPC_TIMEOUT",
		"BFF_INVENTORY_GRPC_ADDR",
		"BFF_INVENTORY_GRPC_TIMEOUT",
		"BFF_CART_GRPC_ADDR",
		"BFF_CART_GRPC_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadRejectsMalformedTimeouts(t *testing.T) {
	envKeys := []string{
		"BFF_CATALOG_GRPC_TIMEOUT",
		"BFF_INVENTORY_GRPC_TIMEOUT",
		"BFF_CART_GRPC_TIMEOUT",
	}
	malformedValues := []string{"3000", "invalid"}

	for _, envKey := range envKeys {
		for _, value := range malformedValues {
			t.Run(envKey+"="+value, func(t *testing.T) {
				clearBFFConfigEnv(t)
				t.Setenv(envKey, value)

				_, err := Load()
				if err == nil {
					t.Fatalf("expected error for %s=%q, got nil", envKey, value)
				}
				if !strings.Contains(err.Error(), envKey) || !strings.Contains(err.Error(), "valid duration") {
					t.Fatalf("Load() error = %q; want %s valid duration error", err.Error(), envKey)
				}
			})
		}
	}
}

func TestLoadRejectsNegativeTimeouts(t *testing.T) {
	for _, envKey := range []string{
		"BFF_CATALOG_GRPC_TIMEOUT",
		"BFF_INVENTORY_GRPC_TIMEOUT",
		"BFF_CART_GRPC_TIMEOUT",
	} {
		t.Run(envKey, func(t *testing.T) {
			clearBFFConfigEnv(t)
			t.Setenv(envKey, "-1s")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for %s=-1s, got nil", envKey)
			}
			if !strings.Contains(err.Error(), envKey) || !strings.Contains(err.Error(), "must be positive") {
				t.Fatalf("Load() error = %q; want %s positive duration error", err.Error(), envKey)
			}
		})
	}
}

func TestLoadDefaultsWhitespaceOnlyTimeouts(t *testing.T) {
	clearBFFConfigEnv(t)
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "   ")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", "   ")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", "   ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error for whitespace-only timeouts: %v", err)
	}
	if cfg.CatalogGRPCTimeout != 3*time.Second {
		t.Fatalf("CatalogGRPCTimeout = %s, want 3s", cfg.CatalogGRPCTimeout)
	}
	if cfg.InventoryGRPCTimeout != 3*time.Second {
		t.Fatalf("InventoryGRPCTimeout = %s, want 3s", cfg.InventoryGRPCTimeout)
	}
	if cfg.CartGRPCTimeout != 3*time.Second {
		t.Fatalf("CartGRPCTimeout = %s, want 3s", cfg.CartGRPCTimeout)
	}
}

func TestLoadTrimsSurroundingWhitespaceOnTimeouts(t *testing.T) {
	clearBFFConfigEnv(t)
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", " 1500ms ")
	t.Setenv("BFF_INVENTORY_GRPC_TIMEOUT", " 2500ms ")
	t.Setenv("BFF_CART_GRPC_TIMEOUT", " 3500ms ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error for padded timeouts: %v", err)
	}
	if cfg.CatalogGRPCTimeout != 1500*time.Millisecond {
		t.Fatalf("CatalogGRPCTimeout = %s, want 1500ms", cfg.CatalogGRPCTimeout)
	}
	if cfg.InventoryGRPCTimeout != 2500*time.Millisecond {
		t.Fatalf("InventoryGRPCTimeout = %s, want 2500ms", cfg.InventoryGRPCTimeout)
	}
	if cfg.CartGRPCTimeout != 3500*time.Millisecond {
		t.Fatalf("CartGRPCTimeout = %s, want 3500ms", cfg.CartGRPCTimeout)
	}
}

func TestLoadIgnoresDotenvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("BFF_HTTP_ADDR=:9180\nBFF_CATALOG_GRPC_ADDR=dotenv.local:50051\nBFF_INVENTORY_GRPC_ADDR=dotenv.local:50052\nBFF_CART_GRPC_ADDR=dotenv.local:50053\n"), 0o600); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	for _, key := range []string{"BFF_HTTP_ADDR", "HTTP_ADDR", "BFF_CATALOG_GRPC_ADDR", "BFF_CATALOG_GRPC_TIMEOUT", "BFF_INVENTORY_GRPC_ADDR", "BFF_INVENTORY_GRPC_TIMEOUT", "BFF_CART_GRPC_ADDR", "BFF_CART_GRPC_TIMEOUT"} {
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
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default HTTP address :8080 when .env present, got %s", cfg.HTTPAddr)
	}
	if cfg.CatalogGRPCAddr != "127.0.0.1:50051" {
		t.Fatalf("expected default Catalog gRPC address 127.0.0.1:50051, got %s", cfg.CatalogGRPCAddr)
	}
	if cfg.InventoryGRPCAddr != "127.0.0.1:50052" {
		t.Fatalf("expected default Inventory gRPC address 127.0.0.1:50052, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.CartGRPCAddr != "127.0.0.1:50053" {
		t.Fatalf("expected default Cart gRPC address 127.0.0.1:50053, got %s", cfg.CartGRPCAddr)
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

	var cartRouteFound, adminProductsRouteFound bool
	for _, service := range config.Services {
		if service.Name != "bff-service" || service.URL != "http://bff-service:8080" {
			continue
		}
		for _, route := range service.Routes {
			switch route.Name {
			case "cart-api":
				cartRouteFound = true
				if len(route.Paths) != 1 || route.Paths[0] != "/api/v1/cart" || route.StripPath {
					t.Fatalf("production cart route path/strip_path = %#v/%t, want [/api/v1/cart]/false", route.Paths, route.StripPath)
				}
				if len(route.Plugins) != 1 || route.Plugins[0].Name != "caller-context" {
					t.Fatalf("production cart route plugins = %#v, want caller-context only", route.Plugins)
				}
				callerContext := route.Plugins[0].Config
				if callerContext.JWKSURI != "http://identity-service:8080/.well-known/jwks.json" || callerContext.Issuer != "http://identity-service:8080" || callerContext.Audience != "ecommerce-api" {
					t.Fatalf("production cart caller-context config = %#v", callerContext)
				}
			case "admin-products-api":
				adminProductsRouteFound = true
				if len(route.Paths) != 1 || route.Paths[0] != "/api/v1/admin/products" || route.StripPath {
					t.Fatalf("production admin-products-api path/strip_path = %#v/%t, want [/api/v1/admin/products]/false", route.Paths, route.StripPath)
				}
				if len(route.Plugins) != 1 || route.Plugins[0].Name != "caller-context" {
					t.Fatalf("production admin-products-api plugins = %#v, want caller-context only", route.Plugins)
				}
				callerContext := route.Plugins[0].Config
				if callerContext.JWKSURI != "http://identity-service:8080/.well-known/jwks.json" || callerContext.Issuer != "http://identity-service:8080" || callerContext.Audience != "ecommerce-api" {
					t.Fatalf("production admin-products-api caller-context config = %#v", callerContext)
				}
			}
		}
	}
	if !cartRouteFound {
		t.Fatal("production Kong config is missing the protected cart-api route on bff-service")
	}
	if !adminProductsRouteFound {
		t.Fatal("production Kong config is missing the protected admin-products-api route on bff-service")
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
	if !strings.Contains(content, `ENV BFF_INVENTORY_GRPC_ADDR="inventory-service:50052"`) {
		t.Fatal("BFF production image does not declare the Inventory gRPC service DNS target")
	}
	if !strings.Contains(content, `ENV BFF_INVENTORY_GRPC_TIMEOUT="3s"`) {
		t.Fatal("BFF production image does not declare the Inventory gRPC timeout")
	}
}
