package inventory

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingInventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	lastRequest      *inventoryv1.GetAvailabilityRequest
	lastBatchRequest *inventoryv1.BatchGetAvailabilityRequest
	lastSetRequest   *inventoryv1.SetStockRequest
	lastDeleteReq    *inventoryv1.DeleteStocksRequest
	quantity         int32
	block            bool
	err              error
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
		SkuId:             req.GetSkuId(),
		ProductId:         "dt-1",
		AvailableQuantity: s.quantity,
	}, nil
}

func (s *recordingInventoryServer) BatchGetAvailability(ctx context.Context, req *inventoryv1.BatchGetAvailabilityRequest) (*inventoryv1.BatchGetAvailabilityResponse, error) {
	s.lastBatchRequest = req
	if s.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.err != nil {
		return nil, s.err
	}
	items := make([]*inventoryv1.AvailabilityItem, 0, len(req.GetSkuIds()))
	for _, skuID := range req.GetSkuIds() {
		items = append(items, &inventoryv1.AvailabilityItem{
			SkuId:             skuID,
			ProductId:         "dt-1",
			AvailableQuantity: s.quantity,
		})
	}
	return &inventoryv1.BatchGetAvailabilityResponse{Items: items}, nil
}

func (s *recordingInventoryServer) SetStock(_ context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.SetStockResponse, error) {
	s.lastSetRequest = req
	if s.err != nil {
		return nil, s.err
	}
	return &inventoryv1.SetStockResponse{
		Stock: &inventoryv1.StockItem{
			SkuId:             req.GetSkuId(),
			ProductId:         req.GetProductId(),
			Quantity:          req.GetQuantity(),
			AvailableQuantity: req.GetQuantity(),
		},
	}, nil
}

func (s *recordingInventoryServer) DeleteStocks(_ context.Context, req *inventoryv1.DeleteStocksRequest) (*inventoryv1.DeleteStocksResponse, error) {
	s.lastDeleteReq = req
	if s.err != nil {
		return nil, s.err
	}
	return &inventoryv1.DeleteStocksResponse{}, nil
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

func TestClientForwardsSkuIDsAndReturnsAvailability(t *testing.T) {
	server := &recordingInventoryServer{quantity: 12}
	c := newRecordingInventoryClient(t, server, time.Second)

	resp, err := c.GetAvailability(context.Background(), "dt-1-v1")
	if err != nil {
		t.Fatalf("GetAvailability() error = %v", err)
	}
	if got := server.lastRequest.GetSkuId(); got != "dt-1-v1" {
		t.Errorf("request sku_id = %q, want dt-1-v1", got)
	}
	if resp.GetSkuId() != "dt-1-v1" || resp.GetAvailableQuantity() != 12 {
		t.Errorf("response = %+v, want sku_id=dt-1-v1 available_quantity=12", resp)
	}

	items, err := c.BatchGetAvailability(context.Background(), []string{"dt-1-v1", "dt-1-v2"})
	if err != nil {
		t.Fatalf("BatchGetAvailability() error = %v", err)
	}
	if !reflect.DeepEqual(server.lastBatchRequest.GetSkuIds(), []string{"dt-1-v1", "dt-1-v2"}) {
		t.Fatalf("batch sku_ids = %v", server.lastBatchRequest.GetSkuIds())
	}
	if len(items) != 2 || items[0].GetSkuId() != "dt-1-v1" {
		t.Fatalf("batch items = %+v", items)
	}
}

func TestClientAppliesPerCallTimeout(t *testing.T) {
	c := newRecordingInventoryClient(t, &recordingInventoryServer{block: true}, 20*time.Millisecond)

	started := time.Now()
	_, err := c.GetAvailability(context.Background(), "dt-1-v1")
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

			_, err := c.GetAvailability(context.Background(), "dt-1-v1")
			if status.Code(err) != code {
				t.Fatalf("GetAvailability() code = %s, want %s; error = %v", status.Code(err), code, err)
			}
		})
	}
}

func TestClientMapsInventoryAdminAndDeleteStocksErrors(t *testing.T) {
	belowSt, _ := status.New(codes.FailedPrecondition, "below reserved").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.inventory.v1",
		Reason: "STOCK_BELOW_RESERVED",
	})
	cBelow := newRecordingInventoryClient(t, &recordingInventoryServer{err: belowSt.Err()}, time.Second)
	if _, err := cBelow.SetStock(context.Background(), "admin-1", "dt-1-v1", "dt-1", 1); !errors.Is(err, ErrStockBelowReserved) {
		t.Fatalf("SetStock(below) = %v, want ErrStockBelowReserved", err)
	}

	reservedSt, _ := status.New(codes.FailedPrecondition, "variant reserved").WithDetails(&errdetails.ErrorInfo{
		Domain:   "ecommerce.inventory.v1",
		Reason:   "VARIANT_HAS_RESERVED_STOCK",
		Metadata: map[string]string{"sku_id": "dt-1-v2"},
	})
	recReserved := &recordingInventoryServer{err: reservedSt.Err()}
	cReserved := newRecordingInventoryClient(t, recReserved, time.Second)
	delErr := cReserved.DeleteStocks(context.Background(), "admin-1", []string{"dt-1-v2"}, true)
	if !errors.Is(delErr, ErrVariantHasReservedStock) {
		t.Fatalf("DeleteStocks(reserved) = %v, want ErrVariantHasReservedStock", delErr)
	}
	var reservedDetail *VariantHasReservedStockError
	if !errors.As(delErr, &reservedDetail) || reservedDetail.SkuID != "dt-1-v2" {
		t.Fatalf("DeleteStocks(reserved) detail = %+v, want sku_id dt-1-v2", reservedDetail)
	}
	if !recReserved.lastDeleteReq.GetDryRun() || recReserved.lastDeleteReq.GetUserId() != "admin-1" {
		t.Fatalf("DeleteStocks request = %+v", recReserved.lastDeleteReq)
	}

	permSt, _ := status.New(codes.PermissionDenied, "denied").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.inventory.v1",
		Reason: "INVENTORY_PERMISSION_DENIED",
	})
	cPerm := newRecordingInventoryClient(t, &recordingInventoryServer{err: permSt.Err()}, time.Second)
	if _, err := cPerm.SetStock(context.Background(), "viewer-1", "dt-1-v1", "dt-1", 10); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("SetStock(denied) = %v, want ErrPermissionDenied", err)
	}

	authSt, _ := status.New(codes.Unavailable, "auth down").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.inventory.v1",
		Reason: "AUTHORIZATION_UNAVAILABLE",
	})
	cAuth := newRecordingInventoryClient(t, &recordingInventoryServer{err: authSt.Err()}, time.Second)
	if _, err := cAuth.SetStock(context.Background(), "admin-1", "dt-1-v1", "dt-1", 10); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("SetStock(auth down) = %v, want ErrAuthorizationUnavailable", err)
	}
}
