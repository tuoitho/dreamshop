package domain

import (
	"errors"
	"time"
)

const (
	StatusInitiating     = "INITIATING"
	StatusPending        = "PENDING"
	StatusSucceeded      = "SUCCEEDED"
	StatusFailed         = "FAILED"
	StatusCancelled      = "CANCELLED"
	StatusRefundRequired = "REFUND_REQUIRED"
)

var (
	ErrPaymentNotFound          = errors.New("payment not found")
	ErrPaymentAmountMismatch    = errors.New("payment amount mismatch")
	ErrPaymentAlreadyCancelled  = errors.New("payment already cancelled")
	ErrPaymentSessionInProgress = errors.New("payment session creation in progress")
	ErrPaymentGatewayRejected   = errors.New("payment gateway rejected session")
)

type Payment struct {
	OrderID       string
	OrderCode     string
	UserID        string
	Method        string
	Status        string
	AmountMinor   int64
	Currency      string
	MomoOrderID   string
	MomoRequestID string
	MomoTransID   string
	PayURL        string
	ResultCode    *int32
	ResultMessage string
	CancelReason  string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateSessionInput struct {
	OrderID       string
	OrderCode     string
	UserID        string
	AmountMinor   int64
	Currency      string
	MomoRequestID string
	ExpiresAt     time.Time
}

type BeginSessionOutcome struct {
	ProceedToGateway bool
	Status           string
	PayURL           string
	MomoOrderID      string
	MomoRequestID    string
}

type CallbackInput struct {
	MomoOrderID   string
	MomoRequestID string
	MomoTransID   string
	AmountMinor   int64
	ResultCode    int32
	ResultMessage string
	CorrelationID string
}

type CallbackOutcome struct {
	OrderID        string
	Status         string
	OutboxInserted bool
}

type OutboxEvent struct {
	ID          string
	EventType   string
	RoutingKey  string
	AggregateID string
	Payload     []byte
}
