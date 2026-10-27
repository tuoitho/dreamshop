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

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/authorization"
	"ecommerce.local/inventory-service/internal/config"
	inventorygrpc "ecommerce.local/inventory-service/internal/grpc"
	internalhttp "ecommerce.local/inventory-service/internal/http"
	"ecommerce.local/inventory-service/internal/store"
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
		log.Fatalf("failed to parse DATABASE_URL: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to connect to inventory database: %v", err)
	}

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	if err := seedIfEnabled(ctx, cfg.SeedDemoData, func(ctx context.Context) error {
		return store.Seed(ctx, pool)
	}); err != nil {
		log.Fatalf("failed to seed inventory: %v", err)
	}

	authClient, err := authorization.NewClient(cfg.IdentityGRPCAddr, cfg.IdentityGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create identity authorization client: %v", err)
	}
	defer func() { _ = authClient.Close() }()

	repo := store.New(pool)
	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, inventorygrpc.NewServerWithAuth(repo, authClient))

	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", internalhttp.NewHealthHandler("inventory-service"))
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: healthMux}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen on %s: %w", cfg.GRPCAddr, err)
			return
		}
		log.Printf("inventory-service gRPC listening on %s", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()

	go func() {
		log.Printf("inventory-service HTTP listening on %s", cfg.HTTPAddr)
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

func seedIfEnabled(ctx context.Context, enabled bool, seed func(context.Context) error) error {
	if !enabled {
		return nil
	}
	return seed(ctx)
}
