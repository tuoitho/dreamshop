package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ecommerce.local/cart-service/internal/catalog"
	"ecommerce.local/cart-service/internal/config"
	cartgrpc "ecommerce.local/cart-service/internal/grpc"
	internalhttp "ecommerce.local/cart-service/internal/http"
	"ecommerce.local/cart-service/internal/inventory"
	"ecommerce.local/cart-service/internal/store"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to parse database configuration")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to connect to cart database")
	}
	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run cart migrations: %v", err)
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

	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, cartgrpc.NewServer(store.New(pool), catalogClient, inventoryClient))
	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", internalhttp.NewHealthHandler("cart-service"))
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: healthMux}

	errCh := make(chan error, 2)
	go func() {
		listener, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen on %s: %w", cfg.GRPCAddr, err)
			return
		}
		log.Printf("cart-service gRPC listening on %s", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(listener)
	}()
	go func() {
		log.Printf("cart-service HTTP listening on %s", cfg.HTTPAddr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received; stopping servers")
		grpcServer.GracefulStop()
		_ = httpServer.Close()
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}
}
