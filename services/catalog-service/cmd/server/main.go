package main

import (
	"log"
	"net/http"
	"os"

	"ecommerce.local/catalog-service/internal/config"
	internalhttp "ecommerce.local/catalog-service/internal/http"
)

func main() {
	if os.Getenv("HTTP_ADDR") == "" {
		os.Setenv("HTTP_ADDR", ":8081")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	handler := internalhttp.NewHealthHandler("catalog-service")
	mux := http.NewServeMux()
	mux.Handle("/healthz", handler)

	log.Printf("catalog-service listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
