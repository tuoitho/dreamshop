package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GRPC_ADDR", "")
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
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("GRPC_ADDR", "0.0.0.0:60000")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/catalog_db")

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
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8081")
	t.Setenv("GRPC_ADDR", "127.0.0.1:50051")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}
