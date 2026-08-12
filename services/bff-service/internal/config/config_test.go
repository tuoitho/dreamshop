package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")

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
}

func TestLoadUsesPrefixedEnvironment(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", ":9080")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "catalog.local:50051")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "5s")

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
}

func TestLoadKeepsLegacyHTTPAddrFallback(t *testing.T) {
	t.Setenv("BFF_HTTP_ADDR", "")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("BFF_CATALOG_GRPC_ADDR", "")
	t.Setenv("BFF_CATALOG_GRPC_TIMEOUT", "")

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

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive Catalog gRPC timeout, got nil")
	}
}

func TestLoadReadsDotenvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("BFF_HTTP_ADDR=:9180\nBFF_CATALOG_GRPC_ADDR=localhost:50051\nBFF_CATALOG_GRPC_TIMEOUT=2s\n"), 0o600); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}

	for _, key := range []string{"BFF_HTTP_ADDR", "HTTP_ADDR", "BFF_CATALOG_GRPC_ADDR", "BFF_CATALOG_GRPC_TIMEOUT"} {
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
}
