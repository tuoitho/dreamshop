package cart

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingCartServer struct {
	cartv1.UnimplementedCartServiceServer
	getRequest    *cartv1.GetCartRequest
	addRequest    *cartv1.AddItemRequest
	updateRequest *cartv1.UpdateItemQuantityRequest
	removeRequest *cartv1.RemoveItemRequest
	clearRequest  *cartv1.ClearCartRequest
	getBlock      bool
	getErr        error
}

func (s *recordingCartServer) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	s.getRequest = req
	if s.getBlock {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &cartv1.GetCartResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) AddItem(_ context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	s.addRequest = req
	return &cartv1.AddItemResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) UpdateItemQuantity(_ context.Context, req *cartv1.UpdateItemQuantityRequest) (*cartv1.UpdateItemQuantityResponse, error) {
	s.updateRequest = req
	return &cartv1.UpdateItemQuantityResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) RemoveItem(_ context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	s.removeRequest = req
	return &cartv1.RemoveItemResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) ClearCart(_ context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	s.clearRequest = req
	return &cartv1.ClearCartResponse{Cart: &cartv1.Cart{}}, nil
}

func newRecordingCartClient(t *testing.T, server cartv1.CartServiceServer, timeout time.Duration) *client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	client, err := NewClient(listener.Addr().String(), timeout)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientForwardsEveryCartRequestField(t *testing.T) {
	server := &recordingCartServer{}
	client := newRecordingCartClient(t, server, time.Second)

	if _, err := client.GetCart(context.Background(), "user-1"); err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if _, err := client.AddItem(context.Background(), "user-1", "product-1", "red", 2); err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	if _, err := client.UpdateItemQuantity(context.Background(), "user-1", "item-1", 3); err != nil {
		t.Fatalf("UpdateItemQuantity() error = %v", err)
	}
	if _, err := client.RemoveItem(context.Background(), "user-1", "item-1"); err != nil {
		t.Fatalf("RemoveItem() error = %v", err)
	}
	if _, err := client.ClearCart(context.Background(), "user-1"); err != nil {
		t.Fatalf("ClearCart() error = %v", err)
	}

	if got := server.getRequest.GetUserId(); got != "user-1" {
		t.Errorf("GetCart user_id = %q, want user-1", got)
	}
	if got := server.addRequest; got.GetUserId() != "user-1" || got.GetProductId() != "product-1" || got.GetVariantLabel() != "red" || got.GetQuantity() != 2 {
		t.Errorf("AddItem request = %v", got)
	}
	if got := server.updateRequest; got.GetUserId() != "user-1" || got.GetItemId() != "item-1" || got.GetQuantity() != 3 {
		t.Errorf("UpdateItemQuantity request = %v", got)
	}
	if got := server.removeRequest; got.GetUserId() != "user-1" || got.GetItemId() != "item-1" {
		t.Errorf("RemoveItem request = %v", got)
	}
	if got := server.clearRequest.GetUserId(); got != "user-1" {
		t.Errorf("ClearCart user_id = %q, want user-1", got)
	}
}

func TestClientAppliesPerCallTimeout(t *testing.T) {
	client := newRecordingCartClient(t, &recordingCartServer{getBlock: true}, 20*time.Millisecond)

	started := time.Now()
	_, err := client.GetCart(context.Background(), "user-1")
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("GetCart() code = %s, want DeadlineExceeded; error = %v", status.Code(err), err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("GetCart() exceeded deterministic timeout window: %s", elapsed)
	}
}

func TestClientMapsNotFoundToErrCartItemNotFound(t *testing.T) {
	client := newRecordingCartClient(t, &recordingCartServer{getErr: status.Error(codes.NotFound, "missing")}, time.Second)

	_, err := client.GetCart(context.Background(), "user-1")
	if !errors.Is(err, ErrCartItemNotFound) {
		t.Fatalf("GetCart() error = %v, want ErrCartItemNotFound", err)
	}
}

func TestClientPreservesNonNotFoundGRPCStatusErrors(t *testing.T) {
	for _, code := range []codes.Code{codes.InvalidArgument, codes.Internal, codes.Unavailable} {
		t.Run(code.String(), func(t *testing.T) {
			client := newRecordingCartClient(t, &recordingCartServer{getErr: status.Error(code, "cart failure")}, time.Second)

			_, err := client.GetCart(context.Background(), "user-1")
			if status.Code(err) != code {
				t.Fatalf("GetCart() code = %s, want %s; error = %v", status.Code(err), code, err)
			}
			if errors.Is(err, ErrCartItemNotFound) {
				t.Fatalf("GetCart() error unexpectedly mapped to ErrCartItemNotFound: %v", err)
			}
		})
	}
}
