package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"ecommerce.local/cart-service/internal/config"
	internalhttp "ecommerce.local/cart-service/internal/http"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestOpenPoolDoesNotExposeDatabaseURLOnStartupError(t *testing.T) {
	secret := "super-secret-password"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := openPool(ctx, "postgres://user:"+secret+"@%gh&")
	if err == nil {
		t.Fatal("openPool() error = nil, want malformed DATABASE_URL error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("openPool() error exposes DATABASE_URL secret: %v", err)
	}
}

func TestServeWithListenersShutsDownBothServersOnContextCancellation(t *testing.T) {
	grpcListener := testListener(t)
	httpListener := testListener(t)
	grpcServer := grpc.NewServer()
	httpServer := &http.Server{Handler: http.NewServeMux()}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serveWithListeners(ctx, grpcServer, httpServer, grpcListener, httpListener, ready)
	}()

	<-ready
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveWithListeners() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveWithListeners() did not stop after context cancellation")
	}

	assertListenerClosed(t, grpcListener.Addr().String())
	assertListenerClosed(t, httpListener.Addr().String())
}

func TestApplicationShutsDownBothServersOnSIGTERM(t *testing.T) {
	cfg := &config.Config{
		GRPCAddr: "127.0.0.1:0",
		HTTPAddr: ":0",
	}
	addresses := make(chan [2]string, 1)
	done := make(chan error, 1)
	terminated := false
	t.Cleanup(func() {
		if terminated {
			return
		}
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("application did not stop during test cleanup")
		}
	})
	go func() {
		done <- runWithSignal(cfg, func(context.Context, *config.Config) (cartv1.CartServiceServer, func(), error) {
			return signalTestCartService{}, func() {}, nil
		}, func(grpcAddress, httpAddress string) {
			addresses <- [2]string{grpcAddress, httpAddress}
		})
	}()

	var serverAddresses [2]string
	select {
	case serverAddresses = <-addresses:
	case <-time.After(2 * time.Second):
		t.Fatal("application did not publish listener addresses")
	}

	client := &http.Client{Timeout: 100 * time.Millisecond}
	waitForReady(t, func() bool {
		healthResponse, err := client.Get("http://" + serverAddresses[1] + "/healthz")
		if err != nil {
			return false
		}
		_ = healthResponse.Body.Close()
		if healthResponse.StatusCode != http.StatusOK {
			return false
		}

		conn, err := grpc.NewClient("passthrough:///"+serverAddresses[0], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return false
		}
		defer conn.Close()
		callCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err = cartv1.NewCartServiceClient(conn).GetCart(callCtx, &cartv1.GetCartRequest{UserId: "signal-test"})
		return err == nil
	})

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	terminated = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() after SIGTERM = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("application did not stop after SIGTERM")
	}

	assertClosedEventually(t, serverAddresses[0])
	assertClosedEventually(t, serverAddresses[1])
}

type signalTestCartService struct {
	cartv1.UnimplementedCartServiceServer
}

func (signalTestCartService) GetCart(context.Context, *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	return &cartv1.GetCartResponse{Cart: &cartv1.Cart{}}, nil
}

func waitForReady(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		<-poll.C
	}
	t.Fatal("application did not become ready before deadline")
}

func assertClosedEventually(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		<-poll.C
	}
	t.Fatalf("listener %s remained open after shutdown", address)
}

func testListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
}

func assertListenerClosed(t *testing.T, address string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("listener %s still accepts connections after shutdown", address)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("listener %s did not close deterministically: %v", address, err)
	}
}

var _ = http.ErrServerClosed
var _ = internalhttp.NewHealthHandler
