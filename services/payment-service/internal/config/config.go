package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr           string
	GRPCAddr           string
	DatabaseURL        string
	RabbitMQURL        string
	OutboxPollInterval time.Duration
	MomoPartnerCode    string
	MomoAccessKey      string
	MomoSecretKey      string
	MomoAPIEndpoint    string
	MomoConnectTimeout time.Duration
	MomoReadTimeout    time.Duration
	MomoRedirectURL    string
	MomoIPNURL         string
	MomoMockEnabled    bool
}

func Load() (Config, error) {
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	pollInterval, err := durationEnv("OUTBOX_POLL_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}
	connectTimeout, err := durationEnv("MOMO_CONNECT_TIMEOUT", 3*time.Second)
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := durationEnv("MOMO_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	mockEnabled, err := boolEnv("PAYMENT_MOMO_MOCK_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:           stringEnv("HTTP_ADDR", ":8086"),
		GRPCAddr:           stringEnv("GRPC_ADDR", "127.0.0.1:50056"),
		DatabaseURL:        dbURL,
		RabbitMQURL:        strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		OutboxPollInterval: pollInterval,
		MomoPartnerCode:    stringEnv("MOMO_PARTNER_CODE", "MOMO"),
		MomoAccessKey:      strings.TrimSpace(os.Getenv("MOMO_ACCESS_KEY")),
		MomoSecretKey:      strings.TrimSpace(os.Getenv("MOMO_SECRET_KEY")),
		MomoAPIEndpoint:    stringEnv("MOMO_API_ENDPOINT", "https://test-payment.momo.vn/v2/gateway/api/create"),
		MomoConnectTimeout: connectTimeout,
		MomoReadTimeout:    readTimeout,
		MomoRedirectURL:    stringEnv("MOMO_REDIRECT_URL", "http://localhost:8081/api/v1/payments/momo-redirect"),
		MomoIPNURL:         stringEnv("MOMO_IPN_URL", "http://localhost:8081/api/v1/payments/momo-ipn"),
		MomoMockEnabled:    mockEnabled,
	}, nil
}

func stringEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(val)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid %s=%q", key, val)
	}
	return d, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("invalid %s=%q: %w", key, val, err)
	}
	return b, nil
}
