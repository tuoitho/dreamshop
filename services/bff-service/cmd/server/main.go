package main

import (
	"log"
	"net/http"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/config"
	internalhttp "ecommerce.local/bff-service/internal/http"
	"ecommerce.local/bff-service/internal/inventory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	catalogClient, err := catalog.NewClient(cfg.CatalogGRPCAddr, cfg.CatalogGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create catalog client: %v", err)
	}
	defer catalogClient.Close()

	inventoryClient, err := inventory.NewClient(cfg.InventoryGRPCAddr, cfg.InventoryGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create inventory client: %v", err)
	}
	defer inventoryClient.Close()

	cartClient, err := cart.NewClient(cfg.CartGRPCAddr, cfg.CartGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create cart client: %v", err)
	}
	defer cartClient.Close()

	mux := buildMux(cartClient, catalogClient, inventoryClient)

	log.Printf(
		"bff-service listening on %s; catalog gRPC %s (%s), inventory gRPC %s (%s)",
		cfg.HTTPAddr,
		cfg.CatalogGRPCAddr,
		cfg.CatalogGRPCTimeout,
		cfg.InventoryGRPCAddr,
		cfg.InventoryGRPCTimeout,
	)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func buildMux(cartClient cart.Client, catalogClient catalog.Client, inventoryClient inventory.Client) *http.ServeMux {
	healthHandler := internalhttp.NewHealthHandler("bff-service")
	productsHandler := internalhttp.NewProductsHandler(catalogClient, inventoryClient)
	cartHandler := internalhttp.NewCartHandler(cartClient)
	adminHandler := internalhttp.NewAdminProductsHandler(catalogClient)

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler)
	productsHandler.Register(mux)
	cartHandler.Register(mux)
	adminHandler.Register(mux)
	return mux
}
