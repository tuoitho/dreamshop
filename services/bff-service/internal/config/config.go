package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	HTTPAddr        string
	CatalogGRPCAddr string
}

func Load() (*Config, error) {
	if err := loadDotenv(); err != nil {
		return nil, err
	}

	settings := viper.New()
	settings.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	settings.SetDefault("http.addr", ":8080")
	settings.SetDefault("catalog.grpc.addr", "127.0.0.1:50051")

	if err := settings.BindEnv("http.addr", "BFF_HTTP_ADDR", "HTTP_ADDR"); err != nil {
		return nil, fmt.Errorf("bind HTTP address: %w", err)
	}
	if err := settings.BindEnv("catalog.grpc.addr", "BFF_CATALOG_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Catalog gRPC address: %w", err)
	}

	return &Config{
		HTTPAddr:        settings.GetString("http.addr"),
		CatalogGRPCAddr: settings.GetString("catalog.grpc.addr"),
	}, nil
}

func loadDotenv() error {
	path := os.Getenv("ENV_FILE")
	if path == "" {
		path = ".env"
	}

	if err := godotenv.Load(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("load dotenv file %q: %w", path, err)
	}
	return nil
}
