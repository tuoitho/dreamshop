package inventory

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	block       bool
	gotSkuIDs   []string
	returnItems []*inventoryv1.AvailabilityItem
}

func (s *testInventoryServer) BatchGetAvailability(ctx context.Context, req *inventoryv1.BatchGetAvailabilityRequest) (*inventoryv1.BatchGetAvailabilityResponse, error) {
	s.gotSkuIDs = append([]string(nil), req.GetSkuIds()...)
	if s.block {
		<-ctx.Done()
		return nil, status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return &inventoryv1.BatchGetAvailabilityResponse{Items: s.returnItems}, nil
}

func startInventoryTestServer(t *testing.T, svc inventoryv1.InventoryServiceServer) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(server, svc)
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func TestBatchGetAvailabilityForwardsSkuIDsAndReturnsItems(t *testing.T) {
	svc := &testInventoryServer{
		returnItems: []*inventoryv1.AvailabilityItem{
			{SkuId: "dt-1-v1", ProductId: "dt-1", AvailableQuantity: 5},
			{SkuId: "dt-1-v2", ProductId: "dt-1", AvailableQuantity: 0},
		},
	}
	target, stop := startInventoryTestServer(t, svc)
	defer stop()

	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	items, err := client.BatchGetAvailability(context.Background(), []string{"dt-1-v1", "dt-1-v2"})
	if err != nil {
		t.Fatalf("BatchGetAvailability() error = %v", err)
	}
	if !reflect.DeepEqual(svc.gotSkuIDs, []string{"dt-1-v1", "dt-1-v2"}) {
		t.Fatalf("gotSkuIDs = %v, want [dt-1-v1 dt-1-v2]", svc.gotSkuIDs)
	}
	if len(items) != 2 || items[0].GetSkuId() != "dt-1-v1" || items[0].GetAvailableQuantity() != 5 || items[1].GetSkuId() != "dt-1-v2" {
		t.Fatalf("items = %+v", items)
	}
}

func TestClientUsesPerCallDeadline(t *testing.T) {
	target, stop := startInventoryTestServer(t, &testInventoryServer{block: true})
	defer stop()
	client, err := NewClient(target, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	_, err = client.BatchGetAvailability(context.Background(), []string{"dt-1-v1"})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("BatchGetAvailability() code = %v, want DeadlineExceeded", status.Code(err))
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
	_, err = client.BatchGetAvailability(context.Background(), []string{"dt-1-v1"})
	if err == nil {
		t.Fatal("BatchGetAvailability() after Close() returned nil error")
	}
}
