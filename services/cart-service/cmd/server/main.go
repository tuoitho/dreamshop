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

	"ecommerce.local/cart-service/internal/authorization"
	"ecommerce.local/cart-service/internal/catalog"
	"ecommerce.local/cart-service/internal/config"
	"ecommerce.local/cart-service/internal/events"
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

	if err := runWithSignal(cfg, buildProductionService, func(grpcAddress, httpAddress string) {
		log.Printf("cart-service gRPC listening on %s", grpcAddress)
		log.Printf("cart-service HTTP listening on %s", httpAddress)
	}); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}
}

type serviceBuilder func(context.Context, *config.Config) (cartv1.CartServiceServer, func(), error)

func runWithSignal(cfg *config.Config, build serviceBuilder, onReady func(string, string)) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg, build, onReady)
}

func buildProductionService(ctx context.Context, cfg *config.Config) (cartv1.CartServiceServer, func(), error) {
	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, errors.New("failed to parse database configuration")
	}
	cleanupPool := func() { pool.Close() }
	if err := pool.Ping(ctx); err != nil {
		cleanupPool()
		return nil, nil, errors.New("failed to connect to cart database")
	}
	if err := store.Migrate(ctx, pool); err != nil {
		cleanupPool()
		return nil, nil, errors.New("failed to run cart migrations")
	}

	catalogClient, err := catalog.NewClient(cfg.CatalogGRPCAddr, cfg.CatalogGRPCTimeout)
	if err != nil {
		cleanupPool()
		return nil, nil, errors.New("failed to create catalog client")
	}
	inventoryClient, err := inventory.NewClient(cfg.InventoryGRPCAddr, cfg.InventoryGRPCTimeout)
	if err != nil {
		_ = catalogClient.Close()
		cleanupPool()
		return nil, nil, errors.New("failed to create inventory client")
	}
	authorizationClient, err := authorization.NewClient(cfg.IdentityGRPCAddr, cfg.IdentityGRPCTimeout)
	if err != nil {
		_ = inventoryClient.Close()
		_ = catalogClient.Close()
		cleanupPool()
		return nil, nil, errors.New("failed to create identity authorization client")
	}
	cartStore := store.New(pool)
	stopConsumer := func() {}
	if cfg.RabbitMQURL != "" {
		consumer, err := events.NewConsumer(cfg.RabbitMQURL, cartStore)
		if err != nil {
			_ = authorizationClient.Close()
			_ = inventoryClient.Close()
			_ = catalogClient.Close()
			cleanupPool()
			return nil, nil, errors.New("failed to create cart event consumer")
		}
		consumerCtx, cancelConsumer := context.WithCancel(ctx)
		stopConsumer = cancelConsumer
		go func() {
			if err := consumer.Run(consumerCtx); err != nil {
				log.Printf("cart event consumer stopped: %v", err)
			}
		}()
	}
	return cartgrpc.NewServer(cartStore, catalogClient, inventoryClient, authorizationClient), func() {
		stopConsumer()
		_ = authorizationClient.Close()
		_ = catalogClient.Close()
		_ = inventoryClient.Close()
		cleanupPool()
	}, nil
}

func run(ctx context.Context, cfg *config.Config, build serviceBuilder, onReady func(string, string)) error {
	service, cleanup, err := build(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, service)
	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", internalhttp.NewHealthHandler("cart-service"))
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: healthMux}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return errors.New("failed to listen for gRPC")
	}
	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		_ = grpcListener.Close()
		return errors.New("failed to listen for HTTP")
	}
	if onReady != nil {
		onReady(grpcListener.Addr().String(), httpListener.Addr().String())
	}
	return serveWithListeners(ctx, grpcServer, httpServer, grpcListener, httpListener, nil)
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
