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
	lastTraceID   string
	lastAdminList *orderv1.AdminListOrdersRequest
	createFn      func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
}

func (s *fakeOrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if vals := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(vals) > 0 {
		s.lastTraceID = vals[0]
	}
	return s.createFn(ctx, req)
}

func (s *fakeOrderServer) AdminListOrders(_ context.Context, req *orderv1.AdminListOrdersRequest) (*orderv1.AdminListOrdersResponse, error) {
	s.lastAdminList = req
	return &orderv1.AdminListOrdersResponse{TotalItems: 7}, nil
}

func (s *fakeOrderServer) AdminUpdateOrderStatus(_ context.Context, req *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error) {
	return &orderv1.AdminUpdateOrderStatusResponse{
		Order:               &orderv1.OrderDetail{OrderId: req.GetOrderId(), Status: req.GetTargetStatus()},
		AllowedNextStatuses: []string{"DELIVERED"},
	}, nil
}

func TestClientForwardsAdminOrderCalls(t *testing.T) {
	srv := &fakeOrderServer{}
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

	list, err := c.AdminListOrders(context.Background(), "admin-1", "SHIPPING", "ORD", 2, 30)
	if err != nil || list.GetTotalItems() != 7 {
		t.Fatalf("AdminListOrders: resp=%v err=%v", list, err)
	}
	if got := srv.lastAdminList; got.GetUserId() != "admin-1" || got.GetStatus() != "SHIPPING" || got.GetQuery() != "ORD" ||
		got.GetPage() != 2 || got.GetPageSize() != 30 {
		t.Fatalf("unexpected AdminListOrdersRequest: %+v", got)
	}

	updated, err := c.AdminUpdateOrderStatus(context.Background(), &orderv1.AdminUpdateOrderStatusRequest{
		UserId: "admin-1", OrderId: "ord-1", ExpectedStatus: "PREPARING", TargetStatus: "SHIPPING",
	})
	if err != nil || updated.GetOrder().GetStatus() != "SHIPPING" || len(updated.GetAllowedNextStatuses()) != 1 {
		t.Fatalf("AdminUpdateOrderStatus: resp=%v err=%v", updated, err)
	}
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
