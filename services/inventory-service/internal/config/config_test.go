package config

import (
	"testing"
)

func TestLoadRequiresHTTPAddr(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected HTTP_ADDR validation error")
	}

	t.Setenv("HTTP_ADDR", ":8082")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8082" {
		t.Fatalf("expected HTTPAddr :8082, got %s", cfg.HTTPAddr)
	}
}
