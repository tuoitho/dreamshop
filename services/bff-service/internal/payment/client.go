// Package payment provides the BFF client adapter for the payment-service private gRPC API.
package payment

import (
	"context"
	"fmt"
	"time"

	paymentv1 "ecommerce.local/contracts/go/payment/v1"
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
		return fmt.Sprintf("payment-service %s (%s): %s", e.Reason, e.GRPCStatus, e.Message)
	}
	return fmt.Sprintf("payment-service (%s): %s", e.GRPCStatus, e.Message)
}

type Client interface {
	HandleMomoCallback(ctx context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error)
}

type client struct {
	conn    *grpc.ClientConn
	svc     paymentv1.PaymentServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to payment-service at %s: %w", target, err)
	}
	return &client{
		conn:    conn,
		svc:     paymentv1.NewPaymentServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) HandleMomoCallback(ctx context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.HandleMomoCallback(ctx, req)
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
		if !ok || info.GetDomain() != "ecommerce.payment.v1" {
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
