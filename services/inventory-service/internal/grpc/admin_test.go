package grpc

import (
	"context"
	"net"
	"testing"

	identityv1 "ecommerce.local/contracts/go/identity/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/authorization"
	"ecommerce.local/inventory-service/internal/domain"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"time"
)

type fakeAuth struct {
	perms map[string]bool
	err   error
	calls int
}

func (f *fakeAuth) CheckPermission(_ context.Context, _ string, perm string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.perms[perm], nil
}

func newAdminInventoryClient(t *testing.T, repo domain.Repository, auth authorizationClient) inventoryv1.InventoryServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(srv, NewServerWithAuth(repo, auth))
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///inventory-admin-test",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return inventoryv1.NewInventoryServiceClient(conn)
}

func assertInventoryErrorReason(t *testing.T, err error, wantCode codes.Code, wantReason string) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != wantCode {
		t.Fatalf("status code = %v, want %v", st.Code(), wantCode)
	}
	for _, detail := range st.Proto().GetDetails() {
		if detail.MessageIs(&errdetails.ErrorInfo{}) {
			info := &errdetails.ErrorInfo{}
			if err := detail.UnmarshalTo(info); err != nil {
				t.Fatalf("unmarshal ErrorInfo: %v", err)
			}
			if info.GetDomain() != "ecommerce.inventory.v1" || info.GetReason() != wantReason {
				t.Fatalf("ErrorInfo = domain=%q reason=%q, want ecommerce.inventory.v1 / %q", info.GetDomain(), info.GetReason(), wantReason)
			}
			return
		}
	}
	t.Fatalf("expected google.rpc.ErrorInfo detail %q on %v", wantReason, err)
}

func TestAdminInventoryRPCsAuthorizationAndMutations(t *testing.T) {
	repo := &memoryRepo{stocks: map[string]domain.Stock{
		"dt-1": {ProductID: "dt-1", Quantity: 10, Reserved: 3},
	}}
	auth := &fakeAuth{perms: map[string]bool{
		"inventory.view":   true,
		"inventory.update": true,
	}}
	client := newAdminInventoryClient(t, repo, auth)
	ctx := context.Background()

	// GetAvailability remains public (no user_id required, 0 auth calls)
	avail, err := client.GetAvailability(ctx, &inventoryv1.GetAvailabilityRequest{ProductId: "dt-1"})
	if err != nil || avail.GetAvailableQuantity() != 7 || auth.calls != 0 {
		t.Fatalf("GetAvailability: err=%v avail=%+v authCalls=%d", err, avail, auth.calls)
	}

	getResp, err := client.GetStock(ctx, &inventoryv1.GetStockRequest{UserId: "admin-1", ProductId: "dt-1"})
	if err != nil || getResp.GetStock().GetAvailableQuantity() != 7 {
		t.Fatalf("GetStock: %v, %+v", err, getResp)
	}

	batchResp, err := client.BatchGetStock(ctx, &inventoryv1.BatchGetStockRequest{
		UserId:     "admin-1",
		ProductIds: []string{"dt-1", "missing"},
	})
	if err != nil || len(batchResp.GetItems()) != 2 || batchResp.GetItems()[1].GetQuantity() != 0 {
		t.Fatalf("BatchGetStock: %v, %+v", err, batchResp)
	}

	listResp, err := client.ListStocks(ctx, &inventoryv1.ListStocksRequest{UserId: "admin-1"})
	if err != nil || len(listResp.GetItems()) != 1 {
		t.Fatalf("ListStocks: %v, %+v", err, listResp)
	}

	// SetStock below reserved (2 < 3) -> FailedPrecondition STOCK_BELOW_RESERVED
	_, err = client.SetStock(ctx, &inventoryv1.SetStockRequest{
		UserId:    "admin-1",
		ProductId: "dt-1",
		Quantity:  2,
	})
	assertInventoryErrorReason(t, err, codes.FailedPrecondition, "STOCK_BELOW_RESERVED")

	// SetStock valid
	setResp, err := client.SetStock(ctx, &inventoryv1.SetStockRequest{
		UserId:    "admin-1",
		ProductId: "dt-1",
		Quantity:  15,
	})
	if err != nil || setResp.GetStock().GetQuantity() != 15 || setResp.GetStock().GetAvailableQuantity() != 12 {
		t.Fatalf("SetStock: %v, %+v", err, setResp)
	}

	// AdjustStock valid
	adjResp, err := client.AdjustStock(ctx, &inventoryv1.AdjustStockRequest{
		UserId:    "admin-1",
		ProductId: "dt-1",
		Delta:     -5,
	})
	if err != nil || adjResp.GetStock().GetQuantity() != 10 {
		t.Fatalf("AdjustStock: %v, %+v", err, adjResp)
	}

	// Permission denied
	deniedClient := newAdminInventoryClient(t, repo, &fakeAuth{perms: map[string]bool{"inventory.view": true}})
	_, err = deniedClient.SetStock(ctx, &inventoryv1.SetStockRequest{
		UserId:    "viewer-1",
		ProductId: "dt-1",
		Quantity:  20,
	})
	assertInventoryErrorReason(t, err, codes.PermissionDenied, "INVENTORY_PERMISSION_DENIED")
}

type traceRecordingAuthServer struct {
	identityv1.UnimplementedAuthorizationServiceServer
	traceID string
}

func (s *traceRecordingAuthServer) CheckPermission(ctx context.Context, _ *identityv1.CheckPermissionRequest) (*identityv1.CheckPermissionResponse, error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(values) == 1 {
		s.traceID = values[0]
	}
	return nil, status.Error(codes.Unavailable, "identity down")
}

func TestInventoryAuthorizationForwardsTraceIDAndHandlesOutage(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	idSrv := &traceRecordingAuthServer{}
	grpcSrv := grpc.NewServer()
	identityv1.RegisterAuthorizationServiceServer(grpcSrv, idSrv)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = lis.Close()
	})

	authClient, err := authorization.NewClient(lis.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = authClient.Close() })

	client := newAdminInventoryClient(t, &memoryRepo{stocks: map[string]domain.Stock{}}, authClient)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-trace-id", "inv-trace-99")
	_, err = client.ListStocks(ctx, &inventoryv1.ListStocksRequest{UserId: "admin-1"})
	assertInventoryErrorReason(t, err, codes.Unavailable, "AUTHORIZATION_UNAVAILABLE")
	if idSrv.traceID != "inv-trace-99" {
		t.Fatalf("traceID = %q, want inv-trace-99", idSrv.traceID)
	}
}
