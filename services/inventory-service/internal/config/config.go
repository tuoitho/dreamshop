package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr            string
	GRPCAddr            string
	DatabaseURL         string
	RabbitMQURL         string
	IdentityGRPCAddr    string
	IdentityGRPCTimeout time.Duration
	SeedDemoData        bool
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}

	identityTimeout, err := loadDuration("IDENTITY_GRPC_TIMEOUT", "500ms")
	if err != nil {
		return nil, err
	}

	seedDemoData, err := loadBool("SEED_DEMO_DATA", false)
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr:            valueOrDefault("HTTP_ADDR", ":8082"),
		GRPCAddr:            valueOrDefault("GRPC_ADDR", "127.0.0.1:50052"),
		DatabaseURL:         databaseURL,
		RabbitMQURL:         strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		IdentityGRPCAddr:    valueOrDefault("IDENTITY_GRPC_ADDR", "127.0.0.1:50054"),
		IdentityGRPCTimeout: identityTimeout,
		SeedDemoData:        seedDemoData,
	}, nil
}

func valueOrDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func loadDuration(name, fallback string) (time.Duration, error) {
	raw := valueOrDefault(name, fallback)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return d, nil
}

func loadBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", name, err)
	}
	return parsed, nil
}
