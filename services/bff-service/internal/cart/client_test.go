package cart

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingCartServer struct {
	cartv1.UnimplementedCartServiceServer
	getRequest            *cartv1.GetCartRequest
	addRequest            *cartv1.AddItemRequest
	updateRequest         *cartv1.UpdateItemQuantityRequest
	removeRequest         *cartv1.RemoveItemRequest
	clearRequest          *cartv1.ClearCartRequest
	getFavoritesRequest   *cartv1.GetFavoritesRequest
	addFavoriteRequest    *cartv1.AddFavoriteRequest
	removeFavoriteRequest *cartv1.RemoveFavoriteRequest
	clearFavoritesRequest *cartv1.ClearFavoritesRequest
	getBlock              bool
	getErr                error
	addErr                error
	updateErr             error
	removeErr             error
	clearErr              error
	favErr                error
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
	if s.addErr != nil {
		return nil, s.addErr
	}
	return &cartv1.AddItemResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) UpdateItemQuantity(_ context.Context, req *cartv1.UpdateItemQuantityRequest) (*cartv1.UpdateItemQuantityResponse, error) {
	s.updateRequest = req
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return &cartv1.UpdateItemQuantityResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) RemoveItem(_ context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	s.removeRequest = req
	if s.removeErr != nil {
		return nil, s.removeErr
	}
	return &cartv1.RemoveItemResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) ClearCart(_ context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	s.clearRequest = req
	if s.clearErr != nil {
		return nil, s.clearErr
	}
	return &cartv1.ClearCartResponse{Cart: &cartv1.Cart{}}, nil
}

func (s *recordingCartServer) GetFavorites(_ context.Context, req *cartv1.GetFavoritesRequest) (*cartv1.GetFavoritesResponse, error) {
	s.getFavoritesRequest = req
	if s.favErr != nil {
		return nil, s.favErr
	}
	return &cartv1.GetFavoritesResponse{Favorites: &cartv1.FavoriteList{}}, nil
}

func (s *recordingCartServer) AddFavorite(_ context.Context, req *cartv1.AddFavoriteRequest) (*cartv1.AddFavoriteResponse, error) {
	s.addFavoriteRequest = req
	if s.favErr != nil {
		return nil, s.favErr
	}
	return &cartv1.AddFavoriteResponse{Favorites: &cartv1.FavoriteList{}}, nil
}

func (s *recordingCartServer) RemoveFavorite(_ context.Context, req *cartv1.RemoveFavoriteRequest) (*cartv1.RemoveFavoriteResponse, error) {
	s.removeFavoriteRequest = req
	if s.favErr != nil {
		return nil, s.favErr
	}
	return &cartv1.RemoveFavoriteResponse{Favorites: &cartv1.FavoriteList{}}, nil
}

func (s *recordingCartServer) ClearFavorites(_ context.Context, req *cartv1.ClearFavoritesRequest) (*cartv1.ClearFavoritesResponse, error) {
	s.clearFavoritesRequest = req
	if s.favErr != nil {
		return nil, s.favErr
	}
	return &cartv1.ClearFavoritesResponse{Favorites: &cartv1.FavoriteList{}}, nil
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
	if _, err := client.AddItem(context.Background(), "user-1", "product-1", "product-1-v1", 2); err != nil {
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
	if got := server.addRequest; got.GetUserId() != "user-1" || got.GetProductId() != "product-1" || got.GetSkuId() != "product-1-v1" || got.GetQuantity() != 2 {
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

func TestClientMapsCartPermissionErrorInfoToErrPermissionDenied(t *testing.T) {
	err := status.New(codes.PermissionDenied, "permission denied")
	err, detailErr := err.WithDetails(&errdetails.ErrorInfo{Domain: "ecommerce.cart.v1", Reason: "CART_PERMISSION_DENIED"})
	if detailErr != nil {
		t.Fatalf("add error details: %v", detailErr)
	}
	deniedErr := err.Err()
	server := &recordingCartServer{
		getErr:    deniedErr,
		addErr:    deniedErr,
		updateErr: deniedErr,
		removeErr: deniedErr,
		clearErr:  deniedErr,
	}
	client := newRecordingCartClient(t, server, time.Second)

	calls := []struct {
		name string
		fn   func() error
	}{
		{"GetCart", func() error { _, e := client.GetCart(context.Background(), "user-1"); return e }},
		{"AddItem", func() error { _, e := client.AddItem(context.Background(), "user-1", "product-1", "red", 2); return e }},
		{"UpdateItemQuantity", func() error { _, e := client.UpdateItemQuantity(context.Background(), "user-1", "item-1", 2); return e }},
		{"RemoveItem", func() error { _, e := client.RemoveItem(context.Background(), "user-1", "item-1"); return e }},
		{"ClearCart", func() error { _, e := client.ClearCart(context.Background(), "user-1"); return e }},
	}
	for _, call := range calls {
		if got := call.fn(); !errors.Is(got, ErrPermissionDenied) {
			t.Fatalf("%s() error = %v, want ErrPermissionDenied", call.name, got)
		}
	}
}

func TestClientMapsCartAuthorizationUnavailableErrorInfoToErrAuthorizationUnavailable(t *testing.T) {
	err := status.New(codes.Unavailable, "authorization unavailable")
	err, detailErr := err.WithDetails(&errdetails.ErrorInfo{Domain: "ecommerce.cart.v1", Reason: "AUTHORIZATION_UNAVAILABLE"})
	if detailErr != nil {
		t.Fatalf("add error details: %v", detailErr)
	}
	unavailErr := err.Err()
	server := &recordingCartServer{
		getErr:    unavailErr,
		addErr:    unavailErr,
		updateErr: unavailErr,
		removeErr: unavailErr,
		clearErr:  unavailErr,
	}
	client := newRecordingCartClient(t, server, time.Second)

	calls := []struct {
		name string
		fn   func() error
	}{
		{"GetCart", func() error { _, e := client.GetCart(context.Background(), "user-1"); return e }},
		{"AddItem", func() error { _, e := client.AddItem(context.Background(), "user-1", "product-1", "red", 2); return e }},
		{"UpdateItemQuantity", func() error { _, e := client.UpdateItemQuantity(context.Background(), "user-1", "item-1", 2); return e }},
		{"RemoveItem", func() error { _, e := client.RemoveItem(context.Background(), "user-1", "item-1"); return e }},
		{"ClearCart", func() error { _, e := client.ClearCart(context.Background(), "user-1"); return e }},
	}
	for _, call := range calls {
		if got := call.fn(); !errors.Is(got, ErrAuthorizationUnavailable) {
			t.Fatalf("%s() error = %v, want ErrAuthorizationUnavailable", call.name, got)
		}
	}
}

func TestClientPreservesBareCartUnavailableError(t *testing.T) {
	client := newRecordingCartClient(t, &recordingCartServer{addErr: status.Error(codes.Unavailable, "cart down")}, time.Second)

	_, err := client.AddItem(context.Background(), "user-1", "product-1", "red", 2)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("error code = %s, want Unavailable; error = %v", status.Code(err), err)
	}
	if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("bare Cart outage unexpectedly matched authorization sentinel: %v", err)
	}
}

func TestClientForwardsFavoriteRequestsAndMapsFavoriteErrors(t *testing.T) {
	server := &recordingCartServer{}
	client := newRecordingCartClient(t, server, time.Second)

	if _, err := client.GetFavorites(context.Background(), "user-1"); err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	if _, err := client.AddFavorite(context.Background(), "user-1", "dt-1"); err != nil {
		t.Fatalf("AddFavorite() error = %v", err)
	}
	if _, err := client.RemoveFavorite(context.Background(), "user-1", "dt-1"); err != nil {
		t.Fatalf("RemoveFavorite() error = %v", err)
	}
	if _, err := client.ClearFavorites(context.Background(), "user-1"); err != nil {
		t.Fatalf("ClearFavorites() error = %v", err)
	}

	if server.getFavoritesRequest.GetUserId() != "user-1" {
		t.Fatalf("GetFavorites user_id = %q, want user-1", server.getFavoritesRequest.GetUserId())
	}
	if server.addFavoriteRequest.GetUserId() != "user-1" || server.addFavoriteRequest.GetProductId() != "dt-1" {
		t.Fatalf("AddFavorite request = %+v", server.addFavoriteRequest)
	}
	if server.removeFavoriteRequest.GetUserId() != "user-1" || server.removeFavoriteRequest.GetProductId() != "dt-1" {
		t.Fatalf("RemoveFavorite request = %+v", server.removeFavoriteRequest)
	}
	if server.clearFavoritesRequest.GetUserId() != "user-1" {
		t.Fatalf("ClearFavorites user_id = %q, want user-1", server.clearFavoritesRequest.GetUserId())
	}

	withReason := func(code codes.Code, reason string) error {
		st, err := status.New(code, reason).WithDetails(&errdetails.ErrorInfo{
			Domain: "ecommerce.cart.v1",
			Reason: reason,
		})
		if err != nil {
			t.Fatalf("with details: %v", err)
		}
		return st.Err()
	}

	cases := []struct {
		name string
		err  error
		want error
	}{
		{"permission denied", withReason(codes.PermissionDenied, "FAVORITE_PERMISSION_DENIED"), ErrPermissionDenied},
		{"authorization unavailable", withReason(codes.Unavailable, "AUTHORIZATION_UNAVAILABLE"), ErrAuthorizationUnavailable},
		{"product unavailable", withReason(codes.FailedPrecondition, "FAVORITE_PRODUCT_UNAVAILABLE"), ErrProductUnavailable},
		{"product not found", status.Error(codes.NotFound, "product not found"), ErrProductNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errClient := newRecordingCartClient(t, &recordingCartServer{favErr: tc.err}, time.Second)
			_, got := errClient.AddFavorite(context.Background(), "user-1", "dt-1")
			if !errors.Is(got, tc.want) {
				t.Fatalf("AddFavorite() error = %v, want %v", got, tc.want)
			}
		})
	}
}
