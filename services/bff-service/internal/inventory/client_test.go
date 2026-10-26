package inventory

import (
	"context"
	"net"
	"testing"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	lastRequest *inventoryv1.GetAvailabilityRequest
	quantity    int32
	block       bool
	err         error
}

func (s *recordingInventoryServer) GetAvailability(ctx context.Context, req *inventoryv1.GetAvailabilityRequest) (*inventoryv1.GetAvailabilityResponse, error) {
	s.lastRequest = req
	if s.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.err != nil {
		return nil, s.err
	}
	return &inventoryv1.GetAvailabilityResponse{
		ProductId:         req.GetProductId(),
		AvailableQuantity: s.quantity,
	}, nil
}

func newRecordingInventoryClient(t *testing.T, server inventoryv1.InventoryServiceServer, timeout time.Duration) *client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	c, err := NewClient(listener.Addr().String(), timeout)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClientForwardsProductIDAndReturnsAvailability(t *testing.T) {
	server := &recordingInventoryServer{quantity: 12}
	c := newRecordingInventoryClient(t, server, time.Second)

	resp, err := c.GetAvailability(context.Background(), "dt-1")
	if err != nil {
		t.Fatalf("GetAvailability() error = %v", err)
	}
	if got := server.lastRequest.GetProductId(); got != "dt-1" {
		t.Errorf("request product_id = %q, want dt-1", got)
	}
	if resp.GetProductId() != "dt-1" || resp.GetAvailableQuantity() != 12 {
		t.Errorf("response = %+v, want product_id=dt-1 available_quantity=12", resp)
	}
}

func TestClientAppliesPerCallTimeout(t *testing.T) {
	c := newRecordingInventoryClient(t, &recordingInventoryServer{block: true}, 20*time.Millisecond)

	started := time.Now()
	_, err := c.GetAvailability(context.Background(), "dt-1")
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("GetAvailability() code = %s, want DeadlineExceeded; error = %v", status.Code(err), err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("GetAvailability() exceeded deterministic timeout window: %s", elapsed)
	}
}

func TestClientPropagatesGRPCStatusErrors(t *testing.T) {
	for _, code := range []codes.Code{codes.InvalidArgument, codes.Internal, codes.Unavailable} {
		t.Run(code.String(), func(t *testing.T) {
			c := newRecordingInventoryClient(t, &recordingInventoryServer{err: status.Error(code, "inventory failure")}, time.Second)

			_, err := c.GetAvailability(context.Background(), "dt-1")
			if status.Code(err) != code {
				t.Fatalf("GetAvailability() code = %s, want %s; error = %v", status.Code(err), code, err)
			}
		})
	}
}
