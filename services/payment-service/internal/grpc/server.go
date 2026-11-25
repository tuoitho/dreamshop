package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"ecommerce.local/payment-service/internal/domain"
	"ecommerce.local/payment-service/internal/momo"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const errorDomain = "ecommerce.payment.v1"

type PaymentStore interface {
	BeginCreateSession(ctx context.Context, in domain.CreateSessionInput) (*domain.BeginSessionOutcome, error)
	CompleteCreateSession(ctx context.Context, orderID, payURL string, resultCode int32, resultMessage string, gatewayErr error) (string, error)
	RecordMomoCallback(ctx context.Context, in domain.CallbackInput) (*domain.CallbackOutcome, error)
	BeginManualRefund(ctx context.Context, orderID, reason string) (*domain.ManualRefundBeginOutcome, error)
	CompleteManualRefund(ctx context.Context, orderID string, res domain.RefundAttemptResult) (bool, error)
}

type OutboxNotifier interface {
	Notify()
}

type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
	store      PaymentStore
	momoClient *momo.Client
	notifier   OutboxNotifier
}

func NewServer(store PaymentStore, momoClient *momo.Client, notifier OutboxNotifier) *Server {
	return &Server{
		store:      store,
		momoClient: momoClient,
		notifier:   notifier,
	}
}

func (s *Server) CreatePaymentSession(
	ctx context.Context,
	req *paymentv1.CreatePaymentSessionRequest,
) (*paymentv1.CreatePaymentSessionResponse, error) {
	orderID := strings.TrimSpace(req.GetOrderId())
	userID := strings.TrimSpace(req.GetUserId())
	if orderID == "" || userID == "" || req.GetAmount() == nil || req.GetAmount().GetAmountMinor() <= 0 {
		return nil, statusWithReason(codes.InvalidArgument, "INVALID_PARAMETER", "order_id, user_id, and positive amount are required")
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if rawExp := strings.TrimSpace(req.GetExpiresAt()); rawExp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, rawExp)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, rawExp)
		}
		if err != nil {
			return nil, statusWithReason(codes.InvalidArgument, "INVALID_PARAMETER", "expires_at must be RFC3339")
		}
		expiresAt = parsed.UTC()
	}

	reqID, err := randomRequestID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate request id: %v", err)
	}

	currency := strings.TrimSpace(req.GetAmount().GetCurrency())
	if currency == "" {
		currency = "VND"
	}

	begin, err := s.store.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID:       orderID,
		OrderCode:     strings.TrimSpace(req.GetOrderCode()),
		UserID:        userID,
		AmountMinor:   req.GetAmount().GetAmountMinor(),
		Currency:      currency,
		MomoRequestID: reqID,
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		return nil, mapPaymentError(err)
	}
	if !begin.ProceedToGateway {
		return &paymentv1.CreatePaymentSessionResponse{
			OrderId: orderID,
			Status:  begin.Status,
			PayUrl:  begin.PayURL,
		}, nil
	}

	payURL, gwErr := s.momoClient.CreateSession(ctx, momo.CreateSessionRequest{
		OrderID:     begin.MomoOrderID,
		OrderCode:   strings.TrimSpace(req.GetOrderCode()),
		RequestID:   begin.MomoRequestID,
		AmountMinor: req.GetAmount().GetAmountMinor(),
	})

	var resCode int32
	resMsg := "Successful."
	if gwErr != nil {
		var rejErr *momo.GatewayRejectedError
		if errors.As(gwErr, &rejErr) {
			resCode = rejErr.ResultCode
			resMsg = rejErr.Message
		} else {
			resCode = -1
			resMsg = gwErr.Error()
		}
	}

	completeCtx, cancelComplete := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancelComplete()
	savedURL, completeErr := s.store.CompleteCreateSession(completeCtx, orderID, payURL, resCode, resMsg, gwErr)
	if completeErr != nil {
		return nil, mapPaymentError(completeErr)
	}

	return &paymentv1.CreatePaymentSessionResponse{
		OrderId: orderID,
		Status:  domain.StatusPending,
		PayUrl:  savedURL,
	}, nil
}

func (s *Server) HandleMomoCallback(
	ctx context.Context,
	req *paymentv1.HandleMomoCallbackRequest,
) (*paymentv1.HandleMomoCallbackResponse, error) {
	if s.momoClient == nil || !s.momoClient.VerifyCallback(momo.CallbackSignParams{
		Amount:       req.GetAmount(),
		ExtraData:    req.GetExtraData(),
		Message:      req.GetMessage(),
		OrderID:      req.GetOrderId(),
		OrderInfo:    req.GetOrderInfo(),
		OrderType:    req.GetOrderType(),
		PartnerCode:  req.GetPartnerCode(),
		PayType:      req.GetPayType(),
		RequestID:    req.GetRequestId(),
		ResponseTime: req.GetResponseTime(),
		ResultCode:   req.GetResultCode(),
		TransID:      req.GetTransId(),
	}, req.GetSignature()) {
		return nil, statusWithReason(codes.InvalidArgument, "INVALID_MOMO_SIGNATURE", "invalid MoMo callback signature")
	}

	outcome, err := s.store.RecordMomoCallback(ctx, domain.CallbackInput{
		MomoOrderID:   strings.TrimSpace(req.GetOrderId()),
		MomoRequestID: strings.TrimSpace(req.GetRequestId()),
		MomoTransID:   strings.TrimSpace(req.GetTransId()),
		AmountMinor:   req.GetAmount(),
		ResultCode:    req.GetResultCode(),
		ResultMessage: req.GetMessage(),
		CorrelationID: extractTraceID(ctx),
	})
	if err != nil {
		return nil, mapPaymentError(err)
	}

	if outcome.OutboxInserted && s.notifier != nil {
		s.notifier.Notify()
	}

	redirectResult := "failed"
	if outcome.Status == domain.StatusSucceeded {
		redirectResult = "success"
	}

	return &paymentv1.HandleMomoCallbackResponse{
		OrderId:        outcome.OrderID,
		PaymentStatus:  outcome.Status,
		RedirectResult: redirectResult,
	}, nil
}

func (s *Server) RetryRefund(
	ctx context.Context,
	req *paymentv1.RetryRefundRequest,
) (*paymentv1.RetryRefundResponse, error) {
	orderID := strings.TrimSpace(req.GetOrderId())
	if orderID == "" {
		return nil, statusWithReason(codes.InvalidArgument, "INVALID_PARAMETER", "order_id is required")
	}

	begin, err := s.store.BeginManualRefund(ctx, orderID, strings.TrimSpace(req.GetReason()))
	if err != nil {
		return nil, mapPaymentError(err)
	}
	if begin.AlreadyRefunded {
		return &paymentv1.RetryRefundResponse{
			OrderId:       orderID,
			PaymentStatus: domain.StatusRefunded,
			RefundTransId: begin.RefundTransID,
			Message:       "Already refunded",
		}, nil
	}

	job := begin.Job
	resp, callErr := s.momoClient.Refund(ctx, momo.RefundRequest{
		OrderID:     job.RefundOrderID,
		RequestID:   job.RefundRequestID,
		OrigTransID: job.MomoTransID,
		AmountMinor: job.AmountMinor,
		Description: job.Description,
	})

	var attemptRes domain.RefundAttemptResult
	if callErr == nil {
		attemptRes = domain.RefundAttemptResult{
			Succeeded:     true,
			RefundTransID: resp.RefundTransID,
			ResultCode:    resp.ResultCode,
			ResultMessage: resp.Message,
		}
	} else {
		attemptRes = domain.RefundAttemptResult{
			Succeeded:              false,
			ResultCode:             -1,
			LastError:              callErr.Error(),
			RotateIDsOnNextAttempt: !momo.IsAmbiguousRefundError(callErr),
		}
		var rej *momo.GatewayRejectedError
		if errors.As(callErr, &rej) {
			attemptRes.ResultCode = rej.ResultCode
			attemptRes.ResultMessage = rej.Message
		}
	}

	completeCtx, cancelComplete := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancelComplete()
	outboxInserted, completeErr := s.store.CompleteManualRefund(completeCtx, orderID, attemptRes)
	if completeErr != nil {
		return nil, mapPaymentError(completeErr)
	}
	if outboxInserted && s.notifier != nil {
		s.notifier.Notify()
	}

	if callErr != nil {
		return nil, mapPaymentError(callErr)
	}

	return &paymentv1.RetryRefundResponse{
		OrderId:       orderID,
		PaymentStatus: domain.StatusRefunded,
		RefundTransId: resp.RefundTransID,
		Message:       resp.Message,
	}, nil
}

func mapPaymentError(err error) error {
	var rejErr *momo.GatewayRejectedError
	switch {
	case errors.Is(err, domain.ErrPaymentNotFound):
		return statusWithReason(codes.NotFound, "PAYMENT_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrRefundNotFound):
		return statusWithReason(codes.NotFound, "PAYMENT_REFUND_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrRefundAutoRetryInProgress):
		return statusWithReason(codes.FailedPrecondition, "REFUND_AUTO_RETRY_IN_PROGRESS", err.Error())
	case errors.Is(err, domain.ErrRefundInProgress):
		return statusWithReason(codes.Aborted, "REFUND_IN_PROGRESS", err.Error())
	case errors.Is(err, domain.ErrPaymentAmountMismatch):
		return statusWithReason(codes.InvalidArgument, "PAYMENT_AMOUNT_MISMATCH", err.Error())
	case errors.Is(err, domain.ErrPaymentAlreadyCancelled):
		return statusWithReason(codes.FailedPrecondition, "PAYMENT_ALREADY_CANCELLED", err.Error())
	case errors.Is(err, domain.ErrPaymentSessionInProgress):
		return statusWithReason(codes.Aborted, "PAYMENT_SESSION_IN_PROGRESS", err.Error())
	case errors.Is(err, domain.ErrPaymentGatewayRejected) || errors.As(err, &rejErr):
		msg := "MoMo payment gateway rejected request"
		if rejErr != nil && strings.TrimSpace(rejErr.Message) != "" {
			msg = strings.TrimSpace(rejErr.Message)
		}
		return statusWithReason(codes.FailedPrecondition, "PAYMENT_GATEWAY_REJECTED", msg)
	case errors.Is(err, momo.ErrGatewayUnavailable):
		return statusWithReason(codes.Unavailable, "PAYMENT_GATEWAY_UNAVAILABLE", "MoMo payment gateway is temporarily unavailable")
	default:
		return status.Error(codes.Internal, "payment operation failed")
	}
}

func statusWithReason(code codes.Code, reason, message string) error {
	st := status.New(code, message)
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: reason,
		Domain: errorDomain,
	})
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}

func extractTraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{"x-trace-id", "x-request-id"} {
		if vals := md.Get(key); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}

func randomRequestID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "req-" + hex.EncodeToString(b[:]), nil
}
