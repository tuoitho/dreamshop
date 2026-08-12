package main

import (
	"context"
	"errors"
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

	pool, err := openPool(ctx, cfg.DatabaseURL)
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

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen for gRPC on %s", cfg.GRPCAddr)
	}
	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		_ = grpcListener.Close()
		log.Fatalf("failed to listen for HTTP on %s", cfg.HTTPAddr)
	}

	log.Printf("cart-service gRPC listening on %s", cfg.GRPCAddr)
	log.Printf("cart-service HTTP listening on %s", cfg.HTTPAddr)
	if err := serveWithListeners(ctx, grpcServer, httpServer, grpcListener, httpListener, nil); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}
}

func openPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("unable to create database pool")
	}
	return pool, nil
}

func serveWithListeners(
	ctx context.Context,
	grpcServer *grpc.Server,
	httpServer *http.Server,
	grpcListener net.Listener,
	httpListener net.Listener,
	ready chan<- struct{},
) error {
	defer grpcListener.Close()
	defer httpListener.Close()

	errCh := make(chan error, 2)
	go func() {
		errCh <- grpcServer.Serve(grpcListener)
	}()
	go func() {
		err := httpServer.Serve(httpListener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	if ready != nil {
		close(ready)
	}

	select {
	case <-ctx.Done():
		grpcServer.GracefulStop()
		if err := httpServer.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		grpcServer.Stop()
		_ = httpServer.Close()
		return err
	}
}
