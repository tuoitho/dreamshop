package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr    string
	GRPCAddr    string
	DatabaseURL string
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

	return &Config{
		HTTPAddr:    httpAddr,
		GRPCAddr:    grpcAddr,
		DatabaseURL: databaseURL,
	}, nil
}
