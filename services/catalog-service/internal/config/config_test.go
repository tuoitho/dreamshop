package config

import (
	"strings"
	"testing"
	"time"
)

func clearCatalogConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_ADDR",
		"GRPC_ADDR",
		"DATABASE_URL",
		"IDENTITY_GRPC_ADDR",
		"IDENTITY_GRPC_TIMEOUT",
		"SEED_DEMO_DATA",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearCatalogConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://ecommerce:secret@localhost:5432/catalog_db?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8081" {
		t.Fatalf("expected HTTPAddr :8081, got %s", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != "127.0.0.1:50051" {
		t.Fatalf("expected GRPCAddr 127.0.0.1:50051, got %s", cfg.GRPCAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatalf("expected DatabaseURL to be read from the environment")
	}
	if cfg.IdentityGRPCAddr != "127.0.0.1:50054" {
		t.Fatalf("expected IdentityGRPCAddr 127.0.0.1:50054, got %s", cfg.IdentityGRPCAddr)
	}
	if cfg.IdentityGRPCTimeout != 500*time.Millisecond {
		t.Fatalf("expected IdentityGRPCTimeout 500ms, got %s", cfg.IdentityGRPCTimeout)
	}
	if cfg.SeedDemoData {
		t.Fatal("expected SeedDemoData to default to false")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	clearCatalogConfigEnv(t)
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("GRPC_ADDR", "0.0.0.0:60000")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/catalog_db")
	t.Setenv("IDENTITY_GRPC_ADDR", "identity-service:50054")
	t.Setenv("IDENTITY_GRPC_TIMEOUT", "750ms")
	t.Setenv("SEED_DEMO_DATA", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("expected HTTPAddr :9999, got %s", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != "0.0.0.0:60000" {
		t.Fatalf("expected GRPCAddr 0.0.0.0:60000, got %s", cfg.GRPCAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/catalog_db" {
		t.Fatalf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.IdentityGRPCAddr != "identity-service:50054" {
		t.Fatalf("expected IdentityGRPCAddr identity-service:50054, got %s", cfg.IdentityGRPCAddr)
	}
	if cfg.IdentityGRPCTimeout != 750*time.Millisecond {
		t.Fatalf("expected IdentityGRPCTimeout 750ms, got %s", cfg.IdentityGRPCTimeout)
	}
	if !cfg.SeedDemoData {
		t.Fatal("expected SeedDemoData to be true when SEED_DEMO_DATA=true")
	}
}

func TestLoadRejectsInvalidSeedDemoData(t *testing.T) {
	clearCatalogConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/catalog_db")
	t.Setenv("SEED_DEMO_DATA", "not-a-bool")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SEED_DEMO_DATA") {
		t.Fatalf("Load() error = %v; want SEED_DEMO_DATA validation error", err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	clearCatalogConfigEnv(t)
	t.Setenv("HTTP_ADDR", ":8081")
	t.Setenv("GRPC_ADDR", "127.0.0.1:50051")

	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}

func TestLoadRejectsInvalidIdentityDuration(t *testing.T) {
	for _, value := range []string{"invalid", "0s", "-1s"} {
		t.Run(value, func(t *testing.T) {
			clearCatalogConfigEnv(t)
			t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/catalog_db")
			t.Setenv("IDENTITY_GRPC_TIMEOUT", value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "IDENTITY_GRPC_TIMEOUT") {
				t.Fatalf("Load() error = %v; want IDENTITY_GRPC_TIMEOUT validation", err)
			}
		})
	}
}
