package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	internalhttp "ecommerce.local/cart-service/internal/http"
	"google.golang.org/grpc"
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
