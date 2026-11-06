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

	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"ecommerce.local/payment-service/internal/config"
	"ecommerce.local/payment-service/internal/events"
	paymentgrpc "ecommerce.local/payment-service/internal/grpc"
	internalhttp "ecommerce.local/payment-service/internal/http"
	"ecommerce.local/payment-service/internal/momo"
	"ecommerce.local/payment-service/internal/store"
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
		log.Fatalf("failed to connect to payment database: %v", err)
	}

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run payment migrations: %v", err)
	}

	repo := store.New(pool)
	momoClient := momo.NewClient(momo.ClientConfig{
		PartnerCode:    cfg.MomoPartnerCode,
		AccessKey:      cfg.MomoAccessKey,
		SecretKey:      cfg.MomoSecretKey,
		APIEndpoint:    cfg.MomoAPIEndpoint,
		RedirectURL:    cfg.MomoRedirectURL,
		IPNURL:         cfg.MomoIPNURL,
		ConnectTimeout: cfg.MomoConnectTimeout,
		ReadTimeout:    cfg.MomoReadTimeout,
		MockEnabled:    cfg.MomoMockEnabled,
	})

	var relay *events.OutboxRelay
	if cfg.RabbitMQURL != "" {
		relay = events.NewOutboxRelay(repo, cfg.RabbitMQURL, cfg.OutboxPollInterval)
		go func() {
			if err := relay.Run(ctx); err != nil {
				log.Printf("payment outbox relay stopped: %v", err)
			}
		}()

		consumer, err := events.NewConsumer(cfg.RabbitMQURL, repo)
		if err != nil {
			log.Fatalf("failed to create payment event consumer: %v", err)
		}
		go func() {
			if err := consumer.Run(ctx); err != nil {
				log.Printf("payment event consumer stopped: %v", err)
			}
		}()
	}

	grpcServer := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcServer, paymentgrpc.NewServer(repo, momoClient, relay))

	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", internalhttp.NewHealthHandler("payment-service"))
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: healthMux}

	errCh := make(chan error, 2)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen on %s: %w", cfg.GRPCAddr, err)
			return
		}
		log.Printf("payment-service gRPC listening on %s", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()

	go func() {
		log.Printf("payment-service HTTP listening on %s", cfg.HTTPAddr)
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
