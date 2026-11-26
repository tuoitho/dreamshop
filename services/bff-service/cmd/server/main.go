package main

import (
	"log"
	"net/http"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/config"
	internalhttp "ecommerce.local/bff-service/internal/http"
	"ecommerce.local/bff-service/internal/inventory"
	"ecommerce.local/bff-service/internal/order"
	"ecommerce.local/bff-service/internal/payment"
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

	orderClient, err := order.NewClient(cfg.OrderGRPCAddr, cfg.OrderGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create order client: %v", err)
	}
	defer orderClient.Close()

	paymentClient, err := payment.NewClient(cfg.PaymentGRPCAddr, cfg.PaymentGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create payment client: %v", err)
	}
	defer paymentClient.Close()

	mux := buildMux(cartClient, catalogClient, inventoryClient, orderClient, paymentClient, cfg.StorefrontBaseURL)

	log.Printf(
		"bff-service listening on %s; catalog=%s (%s), inventory=%s (%s), cart=%s (%s), order=%s (%s), payment=%s (%s)",
		cfg.HTTPAddr,
		cfg.CatalogGRPCAddr,
		cfg.CatalogGRPCTimeout,
		cfg.InventoryGRPCAddr,
		cfg.InventoryGRPCTimeout,
		cfg.CartGRPCAddr,
		cfg.CartGRPCTimeout,
		cfg.OrderGRPCAddr,
		cfg.OrderGRPCTimeout,
		cfg.PaymentGRPCAddr,
		cfg.PaymentGRPCTimeout,
	)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func buildMux(
	cartClient cart.Client,
	catalogClient catalog.Client,
	inventoryClient inventory.Client,
	orderClient order.Client,
	paymentClient payment.Client,
	storefrontBaseURL string,
) *http.ServeMux {
	healthHandler := internalhttp.NewHealthHandler("bff-service")
	productsHandler := internalhttp.NewProductsHandler(catalogClient, inventoryClient)
	cartHandler := internalhttp.NewCartHandler(cartClient)
	favoritesHandler := internalhttp.NewFavoritesHandler(cartClient)
	ordersHandler := internalhttp.NewOrdersHandler(orderClient)
	paymentsHandler := internalhttp.NewPaymentsHandler(paymentClient, storefrontBaseURL)
	adminProductsHandler := internalhttp.NewAdminProductsHandler(catalogClient, inventoryClient)
	adminInventoryHandler := internalhttp.NewAdminInventoryHandler(catalogClient, inventoryClient)
	adminOrdersHandler := internalhttp.NewAdminOrdersHandler(orderClient)
	reviewsHandler := internalhttp.NewReviewsHandler(catalogClient)
	adminReviewsHandler := internalhttp.NewAdminReviewsHandler(catalogClient)

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler)
	productsHandler.Register(mux)
	reviewsHandler.Register(mux)
	cartHandler.Register(mux)
	favoritesHandler.Register(mux)
	ordersHandler.Register(mux)
	paymentsHandler.Register(mux)
	adminProductsHandler.Register(mux)
	adminInventoryHandler.Register(mux)
	adminOrdersHandler.Register(mux)
	adminReviewsHandler.Register(mux)
	return mux
}
