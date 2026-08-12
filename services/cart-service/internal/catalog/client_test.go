package catalog

import (
	"context"
	"net"
	"testing"
	"time"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type blockingCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
}

func (blockingCatalogServer) GetProduct(ctx context.Context, _ *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	<-ctx.Done()
	return nil, status.Error(codes.DeadlineExceeded, "request deadline exceeded")
}

func startCatalogTestServer(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(server, blockingCatalogServer{})
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func TestClientUsesPerCallDeadline(t *testing.T) {
	target, stop := startCatalogTestServer(t)
	defer stop()
	client, err := NewClient(target, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	_, err = client.GetProduct(context.Background(), "p-1")
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("GetProduct() code = %v, want DeadlineExceeded", status.Code(err))
	}
}

func TestClientConnectionIsLazyAndCloseEndsLifecycle(t *testing.T) {
	client, err := NewClient("127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatalf("NewClient() should not dial eagerly: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	_, err = client.GetProduct(context.Background(), "p-1")
	if err == nil {
		t.Fatal("GetProduct() after Close() returned nil error")
	}
}
