// Package order provides the BFF client adapter for the order-service private gRPC API.
package order

import (
	"context"
	"fmt"
	"time"

	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type ServiceError struct {
	GRPCStatus codes.Code
	Reason     string
	Message    string
	Metadata   map[string]string
}

func (e *ServiceError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("order-service %s (%s): %s", e.Reason, e.GRPCStatus, e.Message)
	}
	return fmt.Sprintf("order-service (%s): %s", e.GRPCStatus, e.Message)
}

type Client interface {
	CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
	GetOrder(ctx context.Context, userID, orderID string) (*orderv1.GetOrderResponse, error)
	ListOrders(ctx context.Context, userID, statusFilter string, page, pageSize int32) (*orderv1.ListOrdersResponse, error)
	CancelOrder(ctx context.Context, userID, orderID, reason string) (*orderv1.CancelOrderResponse, error)
}

type client struct {
	conn    *grpc.ClientConn
	svc     orderv1.OrderServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to order-service at %s: %w", target, err)
	}
	return &client{
		conn:    conn,
		svc:     orderv1.NewOrderServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.CreateOrder(ctx, req)
	return resp, mapError(err)
}

func (c *client) GetOrder(ctx context.Context, userID, orderID string) (*orderv1.GetOrderResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.GetOrder(ctx, &orderv1.GetOrderRequest{
		UserId:  userID,
		OrderId: orderID,
	})
	return resp, mapError(err)
}

func (c *client) ListOrders(ctx context.Context, userID, statusFilter string, page, pageSize int32) (*orderv1.ListOrdersResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.ListOrders(ctx, &orderv1.ListOrdersRequest{
		UserId:   userID,
		Status:   statusFilter,
		Page:     page,
		PageSize: pageSize,
	})
	return resp, mapError(err)
}

func (c *client) CancelOrder(ctx context.Context, userID, orderID, reason string) (*orderv1.CancelOrderResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.CancelOrder(ctx, &orderv1.CancelOrderRequest{
		UserId:  userID,
		OrderId: orderID,
		Reason:  reason,
	})
	return resp, mapError(err)
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	svcErr := &ServiceError{
		GRPCStatus: st.Code(),
		Message:    st.Message(),
	}
	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetDomain() != "ecommerce.order.v1" {
			continue
		}
		svcErr.Reason = info.GetReason()
		if len(info.GetMetadata()) > 0 {
			svcErr.Metadata = make(map[string]string, len(info.GetMetadata()))
			for k, v := range info.GetMetadata() {
				svcErr.Metadata[k] = v
			}
		}
		break
	}
	return svcErr
}
