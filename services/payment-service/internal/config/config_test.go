package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesSafeDefaultsWithoutKongPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/payment_db?sslmode=disable")
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("MOMO_REDIRECT_URL", "")
	t.Setenv("MOMO_IPN_URL", "")
	t.Setenv("PAYMENT_MOMO_MOCK_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8086" || cfg.GRPCAddr != "127.0.0.1:50056" {
		t.Fatalf("unexpected listen defaults: %+v", cfg)
	}
	if cfg.MomoConnectTimeout != 3*time.Second || cfg.MomoReadTimeout != 10*time.Second {
		t.Fatalf("unexpected momo timeouts: connect=%s read=%s", cfg.MomoConnectTimeout, cfg.MomoReadTimeout)
	}
	if strings.Contains(cfg.MomoRedirectURL, ":8000") || strings.Contains(cfg.MomoIPNURL, ":8000") {
		t.Fatalf("default callback URLs must not reference gateway port: redirect=%q ipn=%q", cfg.MomoRedirectURL, cfg.MomoIPNURL)
	}
}
