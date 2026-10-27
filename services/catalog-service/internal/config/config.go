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
	HTTPAddr             string
	GRPCAddr             string
	DatabaseURL          string
	IdentityGRPCAddr     string
	IdentityGRPCTimeout  time.Duration
	SeedDemoData         bool
	MinIOEndpoint        string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOUseSSL          bool
	MinIOBucket          string
	MinIOPublicBaseURL   string
	MediaPresignTTL      time.Duration
	MediaOrphanGrace     time.Duration
	MediaCleanupInterval time.Duration
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

	minioUseSSL, err := loadBool("MINIO_USE_SSL", false)
	if err != nil {
		return nil, err
	}

	presignTTL, err := loadDuration("MEDIA_PRESIGN_TTL", "5m")
	if err != nil {
		return nil, err
	}

	orphanGrace, err := loadDuration("MEDIA_ORPHAN_GRACE", "24h")
	if err != nil {
		return nil, err
	}

	cleanupInterval, err := loadDuration("MEDIA_CLEANUP_INTERVAL", "15m")
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr:             valueOrDefault("HTTP_ADDR", ":8081"),
		GRPCAddr:             valueOrDefault("GRPC_ADDR", "127.0.0.1:50051"),
		DatabaseURL:          databaseURL,
		IdentityGRPCAddr:     valueOrDefault("IDENTITY_GRPC_ADDR", "127.0.0.1:50054"),
		IdentityGRPCTimeout:  identityTimeout,
		SeedDemoData:         seedDemoData,
		MinIOEndpoint:        valueOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:       strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY")),
		MinIOSecretKey:       strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY")),
		MinIOUseSSL:          minioUseSSL,
		MinIOBucket:          valueOrDefault("MINIO_BUCKET", "product-media"),
		MinIOPublicBaseURL:   valueOrDefault("MINIO_PUBLIC_BASE_URL", "http://localhost:9000/product-media"),
		MediaPresignTTL:      presignTTL,
		MediaOrphanGrace:     orphanGrace,
		MediaCleanupInterval: cleanupInterval,
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
