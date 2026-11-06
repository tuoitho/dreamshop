package order

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	commonv1 "ecommerce.local/contracts/go/common/v1"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeOrderServer struct {
	orderv1.UnimplementedOrderServiceServer
	lastTraceID string
	createFn    func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
}

func (s *fakeOrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if vals := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(vals) > 0 {
		s.lastTraceID = vals[0]
	}
	return s.createFn(ctx, req)
}

func TestClientUnpacksOrderErrorInfoAndPropagatesTrace(t *testing.T) {
	srv := &fakeOrderServer{
		createFn: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			st := status.New(codes.FailedPrecondition, "Thông tin sản phẩm vừa thay đổi")
			withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
				Domain: "ecommerce.order.v1",
				Reason: "PRODUCT_SNAPSHOT_MISMATCH",
				Metadata: map[string]string{
					"sku-1": "[PRICE_CHANGED] Giá phân loại vừa thay đổi",
				},
			})
			if err != nil {
				return nil, st.Err()
			}
			return nil, withDetails.Err()
		},
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, srv)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	c, err := NewClient(lis.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-trace-id", "trace-ord-1"))
	_, err = c.CreateOrder(ctx, &orderv1.CreateOrderRequest{
		UserId:         "user-1",
		IdempotencyKey: "idem-1",
		PaymentMethod:  "COD",
		Items: []*orderv1.OrderItem{{
			ProductId: "p-1",
			SkuId:     "sku-1",
			Quantity:  1,
			UnitPrice: &commonv1.Money{Currency: "VND", AmountMinor: 100000},
		}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if srv.lastTraceID != "trace-ord-1" {
		t.Fatalf("expected trace-ord-1, got %q", srv.lastTraceID)
	}
	var svcErr *ServiceError
	if !errors.As(err, &svcErr) {
		t.Fatalf("expected *ServiceError, got %T: %v", err, err)
	}
	if svcErr.Reason != "PRODUCT_SNAPSHOT_MISMATCH" || svcErr.Metadata["sku-1"] != "[PRICE_CHANGED] Giá phân loại vừa thay đổi" {
		t.Fatalf("unexpected ServiceError: %+v", svcErr)
	}
}
