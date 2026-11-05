package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	commonv1 "ecommerce.local/contracts/go/common/v1"
	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"ecommerce.local/payment-service/internal/domain"
	"ecommerce.local/payment-service/internal/momo"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type spyPaymentStore struct {
	callbackCalls int
}

func (s *spyPaymentStore) BeginCreateSession(_ context.Context, in domain.CreateSessionInput) (*domain.BeginSessionOutcome, error) {
	return &domain.BeginSessionOutcome{ProceedToGateway: true, MomoOrderID: in.OrderID, MomoRequestID: in.MomoRequestID}, nil
}

func (s *spyPaymentStore) CompleteCreateSession(_ context.Context, _, payURL string, _ int32, _ string, gatewayErr error) (string, error) {
	if gatewayErr != nil {
		return "", gatewayErr
	}
	return payURL, nil
}

func (s *spyPaymentStore) RecordMomoCallback(_ context.Context, in domain.CallbackInput) (*domain.CallbackOutcome, error) {
	s.callbackCalls++
	return &domain.CallbackOutcome{
		OrderID:        in.MomoOrderID,
		Status:         "SUCCEEDED",
		OutboxInserted: true,
	}, nil
}

func TestHandleMomoCallbackRejectsForgedSignatureBeforeStoreAccess(t *testing.T) {
	store := &spyPaymentStore{}
	momoClient := momo.NewClient(momo.ClientConfig{
		PartnerCode: "MOMO",
		AccessKey:   "ACCESS_1",
		SecretKey:   "SECRET_1",
		RedirectURL: "http://localhost:8081/api/v1/payments/momo-redirect",
		IPNURL:      "http://localhost:8081/api/v1/payments/momo-ipn",
		MockEnabled: true,
	})
	srv := NewServer(store, momoClient, nil)

	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///payment-grpc-test",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := paymentv1.NewPaymentServiceClient(conn)

	for _, forgedCode := range []int32{0, 1006} {
		_, err := client.HandleMomoCallback(context.Background(), &paymentv1.HandleMomoCallbackRequest{
			PartnerCode:  "MOMO",
			OrderId:      "ord-victim",
			RequestId:    "req-victim",
			Amount:       150000,
			OrderInfo:    "Order",
			OrderType:    "momo_wallet",
			TransId:      "999",
			ResultCode:   forgedCode,
			Message:      "Forged",
			PayType:      "qr",
			ResponseTime: time.Now().UnixMilli(),
			Signature:    "0000000000000000000000000000000000000000000000000000000000000000",
			Source:       "REDIRECT",
		})
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Fatalf("resultCode=%d: expected InvalidArgument, got %v", forgedCode, err)
		}
		var reason string
		for _, d := range st.Proto().GetDetails() {
			if d.MessageIs(&errdetails.ErrorInfo{}) {
				info := &errdetails.ErrorInfo{}
				if err := d.UnmarshalTo(info); err == nil {
					reason = info.GetReason()
				}
			}
		}
		if reason != "INVALID_MOMO_SIGNATURE" {
			t.Fatalf("resultCode=%d: expected INVALID_MOMO_SIGNATURE, got %q", forgedCode, reason)
		}
	}

	if store.callbackCalls != 0 {
		t.Fatalf("expected 0 store calls on forged signatures, got %d", store.callbackCalls)
	}

	createResp, err := client.CreatePaymentSession(context.Background(), &paymentv1.CreatePaymentSessionRequest{
		OrderId:   "ord-create-1",
		OrderCode: "ORD-CREATE-1",
		UserId:    "user-1",
		Amount:    &commonv1.Money{Currency: "VND", AmountMinor: 150000},
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339),
	})
	if err != nil || createResp.GetPayUrl() == "" || createResp.GetStatus() != "PENDING" {
		t.Fatalf("CreatePaymentSession: resp=%+v err=%v", createResp, err)
	}
}
