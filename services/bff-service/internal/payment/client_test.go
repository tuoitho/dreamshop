package payment

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakePaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
	callbackFn func(context.Context, *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error)
}

func (s *fakePaymentServer) HandleMomoCallback(ctx context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
	return s.callbackFn(ctx, req)
}

func TestClientUnpacksPaymentErrorInfo(t *testing.T) {
	srv := &fakePaymentServer{
		callbackFn: func(context.Context, *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
			st := status.New(codes.InvalidArgument, "invalid MoMo signature")
			withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
				Domain: "ecommerce.payment.v1",
				Reason: "INVALID_MOMO_SIGNATURE",
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
	paymentv1.RegisterPaymentServiceServer(grpcServer, srv)
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

	_, err = c.HandleMomoCallback(context.Background(), &paymentv1.HandleMomoCallbackRequest{
		OrderId:   "ord-1",
		Signature: "bad",
		Source:    "IPN",
	})
	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Reason != "INVALID_MOMO_SIGNATURE" || svcErr.GRPCStatus != codes.InvalidArgument {
		t.Fatalf("expected INVALID_MOMO_SIGNATURE ServiceError, got %v", err)
	}
}
