package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr     string
	GRPCAddr     string
	DatabaseURL  string
	SeedDemoData bool
}

func Load() (*Config, error) {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8082"
	}

	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = "127.0.0.1:50052"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}

	seedDemoData, err := loadBool("SEED_DEMO_DATA", false)
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr:     httpAddr,
		GRPCAddr:     grpcAddr,
		DatabaseURL:  databaseURL,
		SeedDemoData: seedDemoData,
	}, nil
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
