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
	if cfg.MomoRefundEndpoint != "https://test-payment.momo.vn/v2/gateway/api/refund" {
		t.Fatalf("unexpected default MomoRefundEndpoint: %q", cfg.MomoRefundEndpoint)
	}
	if cfg.MomoRequestType != "captureWallet" {
		t.Fatalf("unexpected default MomoRequestType: %q", cfg.MomoRequestType)
	}
	if cfg.RefundMaxAutoAttempts != 3 || cfg.RefundPollInterval != 2*time.Second || cfg.RefundRetryBackoff != 10*time.Second {
		t.Fatalf("unexpected refund defaults: max=%d poll=%s backoff=%s", cfg.RefundMaxAutoAttempts, cfg.RefundPollInterval, cfg.RefundRetryBackoff)
	}
}

func TestLoadRefundEndpointDerivationAndValidation(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/payment_db?sslmode=disable")
	t.Setenv("MOMO_API_ENDPOINT", "https://payment.momo.vn/v2/gateway/api/create")
	t.Setenv("MOMO_REFUND_ENDPOINT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load derived refund endpoint: %v", err)
	}
	if cfg.MomoRefundEndpoint != "https://payment.momo.vn/v2/gateway/api/refund" {
		t.Fatalf("expected derived refund endpoint, got %q", cfg.MomoRefundEndpoint)
	}

	t.Setenv("MOMO_REFUND_ENDPOINT", "https://custom.momo.vn/v2/gateway/api/refund")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load explicit refund endpoint: %v", err)
	}
	if cfg.MomoRefundEndpoint != "https://custom.momo.vn/v2/gateway/api/refund" {
		t.Fatalf("expected explicit refund endpoint, got %q", cfg.MomoRefundEndpoint)
	}

	t.Setenv("PAYMENT_REFUND_MAX_AUTO_ATTEMPTS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for PAYMENT_REFUND_MAX_AUTO_ATTEMPTS=0")
	}
	t.Setenv("PAYMENT_REFUND_MAX_AUTO_ATTEMPTS", "3")

	t.Setenv("PAYMENT_REFUND_POLL_INTERVAL", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for PAYMENT_REFUND_POLL_INTERVAL=0s")
	}
	t.Setenv("PAYMENT_REFUND_POLL_INTERVAL", "2s")

	t.Setenv("PAYMENT_REFUND_RETRY_BACKOFF", "-1s")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for PAYMENT_REFUND_RETRY_BACKOFF=-1s")
	}
}
