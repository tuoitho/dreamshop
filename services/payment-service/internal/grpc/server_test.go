package grpc

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
	callbackCalls          int
	beginManualOutcome     *domain.ManualRefundBeginOutcome
	beginManualErr         error
	completeManualCalls    int
	lastCompleteManualRes  domain.RefundAttemptResult
	completeManualOutbox   bool
	completeManualErr      error
}

type spyNotifier struct {
	count int
}

func (n *spyNotifier) Notify() {
	n.count++
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

func (s *spyPaymentStore) BeginManualRefund(_ context.Context, _, _ string) (*domain.ManualRefundBeginOutcome, error) {
	if s.beginManualErr != nil {
		return nil, s.beginManualErr
	}
	return s.beginManualOutcome, nil
}

func (s *spyPaymentStore) CompleteManualRefund(_ context.Context, _ string, res domain.RefundAttemptResult) (bool, error) {
	s.completeManualCalls++
	s.lastCompleteManualRes = res
	return s.completeManualOutbox, s.completeManualErr
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

func extractErrorReason(err error) (codes.Code, string) {
	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown, ""
	}
	for _, d := range st.Proto().GetDetails() {
		if d.MessageIs(&errdetails.ErrorInfo{}) {
			info := &errdetails.ErrorInfo{}
			if err := d.UnmarshalTo(info); err == nil {
				return st.Code(), info.GetReason()
			}
		}
	}
	return st.Code(), ""
}

func TestRetryRefundSucceedsAndNotifiesOutbox(t *testing.T) {
	store := &spyPaymentStore{
		beginManualOutcome: &domain.ManualRefundBeginOutcome{
			Job: &domain.RefundJob{
				OrderID:         "ord-manual-1",
				MomoTransID:     "mock-123",
				RefundOrderID:   "REFUNDMANUAL1",
				RefundRequestID: "REFREQMANUAL1",
				Description:     "Hoan tien don hang ORD-M1",
				AmountMinor:     150000,
			},
		},
		completeManualOutbox: true,
	}
	notifier := &spyNotifier{}
	momoClient := momo.NewClient(momo.ClientConfig{PartnerCode: "MOMO", MockEnabled: true})
	srv := NewServer(store, momoClient, notifier)

	resp, err := srv.RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{
		OrderId: "ord-manual-1",
		Reason:  "Admin manual retry",
	})
	if err != nil {
		t.Fatalf("RetryRefund: %v", err)
	}
	if resp.GetPaymentStatus() != domain.StatusRefunded || resp.GetRefundTransId() == "" {
		t.Fatalf("unexpected RetryRefund response: %+v", resp)
	}
	if store.completeManualCalls != 1 || !store.lastCompleteManualRes.Succeeded {
		t.Fatalf("expected CompleteManualRefund(Succeeded=true), calls=%d res=%+v", store.completeManualCalls, store.lastCompleteManualRes)
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier.Notify() once, got %d", notifier.count)
	}
}

func TestRetryRefundMapsPreconditionAndGatewayErrors(t *testing.T) {
	momoMock := momo.NewClient(momo.ClientConfig{PartnerCode: "MOMO", MockEnabled: true})

	cases := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantReason string
	}{
		{"not found", domain.ErrRefundNotFound, codes.NotFound, "PAYMENT_REFUND_NOT_FOUND"},
		{"auto retry in progress", domain.ErrRefundAutoRetryInProgress, codes.FailedPrecondition, "REFUND_AUTO_RETRY_IN_PROGRESS"},
		{"in progress", domain.ErrRefundInProgress, codes.Aborted, "REFUND_IN_PROGRESS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := NewServer(&spyPaymentStore{beginManualErr: tc.err}, momoMock, nil)
			_, err := srv.RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-1"})
			code, reason := extractErrorReason(err)
			if code != tc.wantCode || reason != tc.wantReason {
				t.Fatalf("got (%v, %q), want (%v, %q)", code, reason, tc.wantCode, tc.wantReason)
			}
		})
	}

	// Already refunded returns idempotent REFUNDED without calling CompleteManualRefund.
	alreadyStore := &spyPaymentStore{
		beginManualOutcome: &domain.ManualRefundBeginOutcome{
			AlreadyRefunded: true,
			RefundTransID:   "momo-already-1",
		},
	}
	srvAlready := NewServer(alreadyStore, momoMock, nil)
	resp, err := srvAlready.RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-already"})
	if err != nil || resp.GetPaymentStatus() != domain.StatusRefunded || resp.GetRefundTransId() != "momo-already-1" {
		t.Fatalf("unexpected already refunded response: %+v err=%v", resp, err)
	}
	if alreadyStore.completeManualCalls != 0 {
		t.Fatalf("expected 0 CompleteManualRefund calls for already refunded, got %d", alreadyStore.completeManualCalls)
	}

	// Gateway 503 -> persists failure and returns Unavailable / PAYMENT_GATEWAY_UNAVAILABLE.
	gw503 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer gw503.Close()

	store503 := &spyPaymentStore{
		beginManualOutcome: &domain.ManualRefundBeginOutcome{
			Job: &domain.RefundJob{
				OrderID: "ord-503", MomoTransID: "123456", RefundOrderID: "REF503", RefundRequestID: "REQ503",
				Description: "Hoan tien", AmountMinor: 100000,
			},
		},
	}
	client503 := momo.NewClient(momo.ClientConfig{
		PartnerCode: "MOMO", AccessKey: "A", SecretKey: "S", RefundEndpoint: gw503.URL,
	})
	_, err = NewServer(store503, client503, nil).RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-503"})
	code, reason := extractErrorReason(err)
	if code != codes.Unavailable || reason != "PAYMENT_GATEWAY_UNAVAILABLE" {
		t.Fatalf("503: got (%v, %q), want (Unavailable, PAYMENT_GATEWAY_UNAVAILABLE)", code, reason)
	}
	if store503.completeManualCalls != 1 || store503.lastCompleteManualRes.Succeeded || store503.lastCompleteManualRes.RotateIDsOnNextAttempt {
		t.Fatalf("expected persisted ambiguous failure on 503, got calls=%d res=%+v", store503.completeManualCalls, store503.lastCompleteManualRes)
	}

	// Gateway definite rejection -> persists failure with RotateIDsOnNextAttempt=true and returns FailedPrecondition / PAYMENT_GATEWAY_REJECTED.
	gwRej := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCode":1080,"message":"Refund rejected"}`))
	}))
	defer gwRej.Close()

	storeRej := &spyPaymentStore{
		beginManualOutcome: &domain.ManualRefundBeginOutcome{
			Job: &domain.RefundJob{
				OrderID: "ord-rej", MomoTransID: "123456", RefundOrderID: "REFREJ", RefundRequestID: "REQREJ",
				Description: "Hoan tien", AmountMinor: 100000,
			},
		},
	}
	clientRej := momo.NewClient(momo.ClientConfig{
		PartnerCode: "MOMO", AccessKey: "A", SecretKey: "S", RefundEndpoint: gwRej.URL,
	})
	_, err = NewServer(storeRej, clientRej, nil).RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-rej"})
	code, reason = extractErrorReason(err)
	if code != codes.FailedPrecondition || reason != "PAYMENT_GATEWAY_REJECTED" {
		t.Fatalf("reject: got (%v, %q), want (FailedPrecondition, PAYMENT_GATEWAY_REJECTED)", code, reason)
	}
	if storeRej.completeManualCalls != 1 || storeRej.lastCompleteManualRes.Succeeded || !storeRej.lastCompleteManualRes.RotateIDsOnNextAttempt {
		t.Fatalf("expected persisted definite rejection with RotateIDsOnNextAttempt=true, got calls=%d res=%+v", storeRej.completeManualCalls, storeRej.lastCompleteManualRes)
	}

	// Gateway 503 and internal DB errors must not leak internal URLs or SQL error strings in gRPC status message.
	_, err = NewServer(store503, client503, nil).RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-503"})
	st503, _ := status.FromError(err)
	if st503.Message() != "MoMo payment gateway is temporarily unavailable" {
		t.Fatalf("expected sanitized gateway unavailable message, got %q", st503.Message())
	}

	storeInternal := &spyPaymentStore{
		beginManualErr: status.Error(codes.Internal, "pq: relation payment_refunds does not exist"),
	}
	_, err = NewServer(storeInternal, momoMock, nil).RetryRefund(context.Background(), &paymentv1.RetryRefundRequest{OrderId: "ord-int"})
	stInt, _ := status.FromError(err)
	if stInt.Code() != codes.Internal || stInt.Message() != "payment operation failed" {
		t.Fatalf("expected sanitized Internal error without raw DB text, got code=%v msg=%q", stInt.Code(), stInt.Message())
	}
}
