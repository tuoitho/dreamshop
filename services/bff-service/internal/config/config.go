package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTPAddr             string
	CatalogGRPCAddr      string
	CatalogGRPCTimeout   time.Duration
	InventoryGRPCAddr    string
	InventoryGRPCTimeout time.Duration
	CartGRPCAddr         string
	CartGRPCTimeout      time.Duration
	OrderGRPCAddr        string
	OrderGRPCTimeout     time.Duration
	PaymentGRPCAddr      string
	PaymentGRPCTimeout   time.Duration
	StorefrontBaseURL    string
}

func Load() (*Config, error) {
	settings := viper.New()
	settings.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	settings.SetDefault("http.addr", ":8080")
	settings.SetDefault("catalog.grpc.addr", "127.0.0.1:50051")
	settings.SetDefault("inventory.grpc.addr", "127.0.0.1:50052")
	settings.SetDefault("cart.grpc.addr", "127.0.0.1:50053")
	settings.SetDefault("order.grpc.addr", "127.0.0.1:50055")
	settings.SetDefault("payment.grpc.addr", "127.0.0.1:50056")
	settings.SetDefault("storefront.base.url", "http://localhost:5173")

	if err := settings.BindEnv("http.addr", "BFF_HTTP_ADDR", "HTTP_ADDR"); err != nil {
		return nil, fmt.Errorf("bind HTTP address: %w", err)
	}
	if err := settings.BindEnv("catalog.grpc.addr", "BFF_CATALOG_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Catalog gRPC address: %w", err)
	}
	if err := settings.BindEnv("catalog.grpc.timeout", "BFF_CATALOG_GRPC_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("bind Catalog gRPC timeout: %w", err)
	}
	if err := settings.BindEnv("inventory.grpc.addr", "BFF_INVENTORY_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Inventory gRPC address: %w", err)
	}
	if err := settings.BindEnv("inventory.grpc.timeout", "BFF_INVENTORY_GRPC_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("bind Inventory gRPC timeout: %w", err)
	}
	if err := settings.BindEnv("cart.grpc.addr", "BFF_CART_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Cart gRPC address: %w", err)
	}
	if err := settings.BindEnv("cart.grpc.timeout", "BFF_CART_GRPC_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("bind Cart gRPC timeout: %w", err)
	}
	if err := settings.BindEnv("order.grpc.addr", "BFF_ORDER_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Order gRPC address: %w", err)
	}
	if err := settings.BindEnv("order.grpc.timeout", "BFF_ORDER_GRPC_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("bind Order gRPC timeout: %w", err)
	}
	if err := settings.BindEnv("payment.grpc.addr", "BFF_PAYMENT_GRPC_ADDR"); err != nil {
		return nil, fmt.Errorf("bind Payment gRPC address: %w", err)
	}
	if err := settings.BindEnv("payment.grpc.timeout", "BFF_PAYMENT_GRPC_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("bind Payment gRPC timeout: %w", err)
	}
	if err := settings.BindEnv("storefront.base.url", "BFF_STOREFRONT_BASE_URL"); err != nil {
		return nil, fmt.Errorf("bind Storefront base URL: %w", err)
	}

	catalogTimeout, err := loadDuration(settings, "catalog.grpc.timeout", "BFF_CATALOG_GRPC_TIMEOUT", 3*time.Second)
	if err != nil {
		return nil, err
	}
	inventoryTimeout, err := loadDuration(settings, "inventory.grpc.timeout", "BFF_INVENTORY_GRPC_TIMEOUT", 3*time.Second)
	if err != nil {
		return nil, err
	}
	cartTimeout, err := loadDuration(settings, "cart.grpc.timeout", "BFF_CART_GRPC_TIMEOUT", 3*time.Second)
	if err != nil {
		return nil, err
	}
	orderTimeout, err := loadDuration(settings, "order.grpc.timeout", "BFF_ORDER_GRPC_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	paymentTimeout, err := loadDuration(settings, "payment.grpc.timeout", "BFF_PAYMENT_GRPC_TIMEOUT", 5*time.Second)
	if err != nil {
		return nil, err
	}

	storefrontBaseURL := settings.GetString("storefront.base.url")
	if raw, ok := os.LookupEnv("BFF_STOREFRONT_BASE_URL"); ok && strings.TrimSpace(raw) == "/" {
		storefrontBaseURL = ""
	}

	cfg := &Config{
		HTTPAddr:             settings.GetString("http.addr"),
		CatalogGRPCAddr:      settings.GetString("catalog.grpc.addr"),
		CatalogGRPCTimeout:   catalogTimeout,
		InventoryGRPCAddr:    settings.GetString("inventory.grpc.addr"),
		InventoryGRPCTimeout: inventoryTimeout,
		CartGRPCAddr:         settings.GetString("cart.grpc.addr"),
		CartGRPCTimeout:      cartTimeout,
		OrderGRPCAddr:        settings.GetString("order.grpc.addr"),
		OrderGRPCTimeout:     orderTimeout,
		PaymentGRPCAddr:      settings.GetString("payment.grpc.addr"),
		PaymentGRPCTimeout:   paymentTimeout,
		StorefrontBaseURL:    strings.TrimRight(strings.TrimSpace(storefrontBaseURL), "/"),
	}

	if cfg.HTTPAddr == "" {
		return nil, errors.New("HTTP address must not be empty")
	}
	if cfg.CatalogGRPCAddr == "" {
		return nil, errors.New("Catalog gRPC address must not be empty")
	}
	if strings.TrimSpace(cfg.InventoryGRPCAddr) == "" {
		return nil, errors.New("Inventory gRPC address must not be empty")
	}
	if strings.TrimSpace(cfg.CartGRPCAddr) == "" {
		return nil, errors.New("Cart gRPC address must not be empty")
	}
	if strings.TrimSpace(cfg.OrderGRPCAddr) == "" {
		return nil, errors.New("Order gRPC address must not be empty")
	}
	if strings.TrimSpace(cfg.PaymentGRPCAddr) == "" {
		return nil, errors.New("Payment gRPC address must not be empty")
	}

	return cfg, nil
}

func loadDuration(settings *viper.Viper, key, envName string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(settings.GetString(key))
	if raw == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", envName, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", envName)
	}
	return duration, nil
}
