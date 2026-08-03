package main

import (
	"log"
	"net/http"

	"ecommerce.local/bff-service/internal/config"
	internalhttp "ecommerce.local/bff-service/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	handler := internalhttp.NewHealthHandler("bff-service")
	mux := http.NewServeMux()
	mux.Handle("/healthz", handler)

	log.Printf("bff-service listening on %s; catalog gRPC target %s", cfg.HTTPAddr, cfg.CatalogGRPCAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
