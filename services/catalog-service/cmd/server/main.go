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

	"ecommerce.local/catalog-service/internal/authorization"
	"ecommerce.local/catalog-service/internal/config"
	cataloggrpc "ecommerce.local/catalog-service/internal/grpc"
	internalhttp "ecommerce.local/catalog-service/internal/http"
	"ecommerce.local/catalog-service/internal/media"
	"ecommerce.local/catalog-service/internal/store"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
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
		log.Fatalf("failed to connect to PostgreSQL at %s: %v", cfg.DatabaseURL, err)
	}

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	if err := seedIfEnabled(ctx, cfg.SeedDemoData, func(ctx context.Context) error {
		return store.Seed(ctx, pool)
	}); err != nil {
		log.Fatalf("failed to seed catalog: %v", err)
	}

	repo := store.New(pool)

	authClient, err := authorization.NewClient(cfg.IdentityGRPCAddr, cfg.IdentityGRPCTimeout)
	if err != nil {
		log.Fatalf("failed to create identity authorization client: %v", err)
	}
	defer func() { _ = authClient.Close() }()

	var serverOpts []cataloggrpc.ServerOption
	if cfg.MinIOAccessKey != "" && cfg.MinIOSecretKey != "" {
		minioClient, err := media.NewMinIOClient(media.Config{
			Endpoint:      cfg.MinIOEndpoint,
			AccessKey:     cfg.MinIOAccessKey,
			SecretKey:     cfg.MinIOSecretKey,
			Bucket:        cfg.MinIOBucket,
			PublicBaseURL: cfg.MinIOPublicBaseURL,
			UseSSL:        cfg.MinIOUseSSL,
		})
		if err != nil {
			log.Fatalf("failed to configure MinIO media client: %v", err)
		}
		if err := minioClient.EnsureBucket(ctx); err != nil {
			log.Printf("warning: failed to bootstrap MinIO bucket %q on startup: %v", cfg.MinIOBucket, err)
		}
		serverOpts = append(serverOpts, cataloggrpc.WithMediaSigner(minioClient, cfg.MediaPresignTTL, cfg.MediaOrphanGrace))

		sweeper := media.NewSweeper(repo, minioClient, cfg.MediaCleanupInterval, 50)
		go sweeper.Start(ctx)
	} else {
		log.Printf("MinIO credentials not set; presigned product media uploads are disabled")
	}

	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, cataloggrpc.NewServer(repo, authClient, serverOpts...))

	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", internalhttp.NewHealthHandler("catalog-service"))
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: healthMux}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen on %s: %w", cfg.GRPCAddr, err)
			return
		}
		log.Printf("catalog-service gRPC listening on %s", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()

	go func() {
		log.Printf("catalog-service HTTP listening on %s", cfg.HTTPAddr)
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
