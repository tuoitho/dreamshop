package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr            string
	GRPCAddr            string
	DatabaseURL         string
	IdentityGRPCAddr    string
	IdentityGRPCTimeout time.Duration
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

	return &Config{
		HTTPAddr:            valueOrDefault("HTTP_ADDR", ":8081"),
		GRPCAddr:            valueOrDefault("GRPC_ADDR", "127.0.0.1:50051"),
		DatabaseURL:         databaseURL,
		IdentityGRPCAddr:    valueOrDefault("IDENTITY_GRPC_ADDR", "127.0.0.1:50054"),
		IdentityGRPCTimeout: identityTimeout,
	}, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func loadDuration(name, fallback string) (time.Duration, error) {
	value := valueOrDefault(name, fallback)
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return duration, nil
}
