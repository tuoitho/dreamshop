package main

import (
	"log"
	"net/http"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/config"
	internalhttp "ecommerce.local/bff-service/internal/http"
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

	cartClient, err := cart.NewClient(cfg.CartGRPCAddr, cfg.CartGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create cart client: %v", err)
	}
	defer cartClient.Close()

	mux := buildMux(cartClient, catalogClient)

	log.Printf("bff-service listening on %s; catalog gRPC target %s (timeout %s)", cfg.HTTPAddr, cfg.CatalogGRPCAddr, cfg.CatalogGRPCTimeout)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func buildMux(cartClient cart.Client, catalogClient catalog.Client) *http.ServeMux {
	healthHandler := internalhttp.NewHealthHandler("bff-service")
	productsHandler := internalhttp.NewProductsHandler(catalogClient)
	cartHandler := internalhttp.NewCartHandler(cartClient)
	adminHandler := internalhttp.NewAdminProductsHandler(catalogClient)

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler)
	productsHandler.Register(mux)
	cartHandler.Register(mux)
	adminHandler.Register(mux)
	return mux
}
