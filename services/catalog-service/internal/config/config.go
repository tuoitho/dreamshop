package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr string
}

func Load() (*Config, error) {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		return nil, errors.New("HTTP_ADDR environment variable is required")
	}
	return &Config{
		HTTPAddr: addr,
	}, nil
}
