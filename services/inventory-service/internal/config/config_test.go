package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GRPC_ADDR", "")
	t.Setenv("IDENTITY_GRPC_ADDR", "")
	t.Setenv("IDENTITY_GRPC_TIMEOUT", "")
	t.Setenv("SEED_DEMO_DATA", "")
	t.Setenv("DATABASE_URL", "postgres://ecommerce:secret@localhost:5432/inventory_db?sslmode=disable")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8082" {
		t.Fatalf("expected HTTPAddr :8082, got %s", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != "127.0.0.1:50052" {
		t.Fatalf("expected GRPCAddr 127.0.0.1:50052, got %s", cfg.GRPCAddr)
	}
	if cfg.IdentityGRPCAddr != "127.0.0.1:50054" {
		t.Fatalf("expected IdentityGRPCAddr 127.0.0.1:50054, got %s", cfg.IdentityGRPCAddr)
	}
	if cfg.IdentityGRPCTimeout != 500*time.Millisecond {
		t.Fatalf("expected IdentityGRPCTimeout 500ms, got %s", cfg.IdentityGRPCTimeout)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected DatabaseURL to be read from the environment")
	}
	if cfg.SeedDemoData {
		t.Fatal("expected SeedDemoData to default to false")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("GRPC_ADDR", "0.0.0.0:60000")
	t.Setenv("IDENTITY_GRPC_ADDR", "identity-service:50054")
	t.Setenv("IDENTITY_GRPC_TIMEOUT", "750ms")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/inventory_db")
	t.Setenv("SEED_DEMO_DATA", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9999" || cfg.GRPCAddr != "0.0.0.0:60000" {
		t.Fatalf("unexpected addresses: %+v", cfg)
	}
	if cfg.IdentityGRPCAddr != "identity-service:50054" || cfg.IdentityGRPCTimeout != 750*time.Millisecond {
		t.Fatalf("unexpected identity gRPC settings: %+v", cfg)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/inventory_db" {
		t.Fatalf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if !cfg.SeedDemoData {
		t.Fatal("expected SeedDemoData to be true when SEED_DEMO_DATA=true")
	}
}

func TestLoadRejectsInvalidSeedDemoData(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/inventory_db")
	t.Setenv("SEED_DEMO_DATA", "not-a-bool")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SEED_DEMO_DATA") {
		t.Fatalf("Load() error = %v; want SEED_DEMO_DATA validation error", err)
	}
}

func TestLoadRejectsInvalidIdentityDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/inventory_db")
	t.Setenv("IDENTITY_GRPC_TIMEOUT", "0s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "IDENTITY_GRPC_TIMEOUT") {
		t.Fatalf("Load() error = %v; want IDENTITY_GRPC_TIMEOUT validation error", err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8082")
	t.Setenv("GRPC_ADDR", "127.0.0.1:50052")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}
