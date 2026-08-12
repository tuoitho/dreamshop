package config

import (
	"strings"
	"testing"
	"time"
)

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"HTTP_ADDR", "GRPC_ADDR", "DATABASE_URL", "CATALOG_GRPC_ADDR",
		"CATALOG_GRPC_TIMEOUT", "INVENTORY_GRPC_ADDR", "INVENTORY_GRPC_TIMEOUT",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	clearConfigEnvironment(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL in error, got %v", err)
	}
}

func TestLoadUsesCartDefaults(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/cart_db?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":8084" {
		t.Fatalf("expected HTTPAddr :8084, got %s", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != "127.0.0.1:50053" {
		t.Fatalf("expected GRPCAddr 127.0.0.1:50053, got %s", cfg.GRPCAddr)
	}
	if cfg.CatalogGRPCAddr != "127.0.0.1:50051" {
		t.Fatalf("expected CatalogGRPCAddr 127.0.0.1:50051, got %s", cfg.CatalogGRPCAddr)
	}
	if cfg.InventoryGRPCAddr != "127.0.0.1:50052" {
		t.Fatalf("expected InventoryGRPCAddr 127.0.0.1:50052, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.CatalogGRPCTimeout <= 0 || cfg.InventoryGRPCTimeout <= 0 {
		t.Fatalf("expected positive dependency timeouts, got catalog=%s inventory=%s", cfg.CatalogGRPCTimeout, cfg.InventoryGRPCTimeout)
	}
}

func TestLoadReadsEnvironmentAndValidatesDurations(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/cart_db")
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("GRPC_ADDR", "0.0.0.0:60000")
	t.Setenv("CATALOG_GRPC_ADDR", "catalog:50051")
	t.Setenv("CATALOG_GRPC_TIMEOUT", "1500ms")
	t.Setenv("INVENTORY_GRPC_ADDR", "inventory:50052")
	t.Setenv("INVENTORY_GRPC_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9999" || cfg.GRPCAddr != "0.0.0.0:60000" {
		t.Fatalf("unexpected server addresses: %+v", cfg)
	}
	if cfg.CatalogGRPCAddr != "catalog:50051" || cfg.InventoryGRPCAddr != "inventory:50052" {
		t.Fatalf("unexpected dependency addresses: %+v", cfg)
	}
	if cfg.CatalogGRPCTimeout != 1500*time.Millisecond || cfg.InventoryGRPCTimeout != 2*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
}

func TestLoadRejectsNonPositiveDuration(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/cart_db")
	t.Setenv("CATALOG_GRPC_TIMEOUT", "0s")

	_, err := Load()
	if err == nil {
		t.Fatal("expected non-positive duration validation error")
	}
	if !strings.Contains(err.Error(), "CATALOG_GRPC_TIMEOUT") {
		t.Fatalf("expected catalog timeout in error, got %v", err)
	}
}
