package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) BeginCreateSession(ctx context.Context, in domain.CreateSessionInput) (*domain.BeginSessionOutcome, error) {
	currency := in.Currency
	if currency == "" {
		currency = "VND"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin session tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO payments (
			order_id, order_code, user_id, method, status,
			amount_minor, currency, momo_order_id, momo_request_id, expires_at
		)
		VALUES ($1, $2, $3, 'MOMO', $4, $5, $6, $1, $7, $8)
		ON CONFLICT (order_id) DO NOTHING`,
		in.OrderID,
		in.OrderCode,
		in.UserID,
		domain.StatusInitiating,
		in.AmountMinor,
		currency,
		in.MomoRequestID,
		in.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert initiating payment: %w", err)
	}

	if tag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit initiating payment: %w", err)
		}
		return &domain.BeginSessionOutcome{
			ProceedToGateway: true,
			Status:           domain.StatusInitiating,
			MomoOrderID:      in.OrderID,
			MomoRequestID:    in.MomoRequestID,
		}, nil
	}

	var status, payURL, momoOrderID, momoReqID string
	if err := tx.QueryRow(ctx, `
		SELECT status, pay_url, momo_order_id, momo_request_id
		FROM payments
		WHERE order_id = $1
		FOR UPDATE`,
		in.OrderID,
	).Scan(&status, &payURL, &momoOrderID, &momoReqID); err != nil {
		return nil, fmt.Errorf("lock existing payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit existing payment read: %w", err)
	}

	switch status {
	case domain.StatusPending, domain.StatusSucceeded:
		if payURL != "" || status == domain.StatusSucceeded {
			return &domain.BeginSessionOutcome{
				ProceedToGateway: false,
				Status:           status,
				PayURL:           payURL,
				MomoOrderID:      momoOrderID,
				MomoRequestID:    momoReqID,
			}, nil
		}
		return nil, domain.ErrPaymentGatewayRejected
	case domain.StatusCancelled, domain.StatusRefundRequired, domain.StatusRefunded, domain.StatusRefundFailed:
		return nil, domain.ErrPaymentAlreadyCancelled
	case domain.StatusInitiating:
		return nil, domain.ErrPaymentSessionInProgress
	default:
		return nil, domain.ErrPaymentGatewayRejected
	}
}

func (s *Store) CompleteCreateSession(
	ctx context.Context,
	orderID, payURL string,
	resultCode int32,
	resultMessage string,
	gatewayErr error,
) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin complete session tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, existingPayURL string
	if err := tx.QueryRow(ctx, `
		SELECT status, pay_url
		FROM payments
		WHERE order_id = $1
		FOR UPDATE`,
		orderID,
	).Scan(&status, &existingPayURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrPaymentNotFound
		}
		return "", fmt.Errorf("lock payment for completion: %w", err)
	}

	if status != domain.StatusInitiating {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit non-initiating state: %w", err)
		}
		switch status {
		case domain.StatusPending, domain.StatusSucceeded:
			if existingPayURL != "" {
				return existingPayURL, nil
			}
			return "", domain.ErrPaymentGatewayRejected
		case domain.StatusCancelled, domain.StatusRefundRequired, domain.StatusRefunded, domain.StatusRefundFailed:
			return "", domain.ErrPaymentAlreadyCancelled
		default:
			return "", domain.ErrPaymentGatewayRejected
		}
	}

	if gatewayErr != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    result_code = $3,
			    result_message = $4,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.StatusFailed,
			resultCode,
			resultMessage,
		); err != nil {
			return "", fmt.Errorf("mark payment failed: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit failed payment: %w", err)
		}
		return "", gatewayErr
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $2,
		    pay_url = $3,
		    result_code = $4,
		    result_message = $5,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		domain.StatusPending,
		payURL,
		resultCode,
		resultMessage,
	); err != nil {
		return "", fmt.Errorf("mark payment pending: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit pending payment: %w", err)
	}
	return payURL, nil
}

func (s *Store) RecordMomoCallback(ctx context.Context, in domain.CallbackInput) (*domain.CallbackOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin callback tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID, orderCode, userID, status, currency, existingTransID, cancelReason string
	var amountMinor int64
	if err := tx.QueryRow(ctx, `
		SELECT order_id, order_code, user_id, status, amount_minor, currency, momo_trans_id, cancel_reason
		FROM payments
		WHERE momo_order_id = $1
		FOR UPDATE`,
		in.MomoOrderID,
	).Scan(&orderID, &orderCode, &userID, &status, &amountMinor, &currency, &existingTransID, &cancelReason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("lock payment by momo_order_id: %w", err)
	}

	if amountMinor > 0 && in.AmountMinor != amountMinor {
		return nil, domain.ErrPaymentAmountMismatch
	}
	effectiveAmount := amountMinor
	if effectiveAmount == 0 && in.AmountMinor > 0 {
		effectiveAmount = in.AmountMinor
	}

	var nextStatus string
	var eventType, routingKey string
	ensureRefundRow := false

	if in.ResultCode == 0 {
		switch status {
		case domain.StatusPending, domain.StatusInitiating:
			nextStatus = domain.StatusSucceeded
			eventType = "PaymentSucceeded"
			routingKey = "payment.succeeded"
		case domain.StatusCancelled, domain.StatusFailed:
			nextStatus = domain.StatusRefundRequired
			eventType = "PaymentSucceeded"
			routingKey = "payment.succeeded"
		case domain.StatusRefundRequired:
			if existingTransID == "" {
				nextStatus = domain.StatusRefundRequired
				eventType = "PaymentSucceeded"
				routingKey = "payment.succeeded"
				ensureRefundRow = true
			} else {
				if err := tx.Commit(ctx); err != nil {
					return nil, fmt.Errorf("commit idempotent success callback: %w", err)
				}
				return &domain.CallbackOutcome{
					OrderID:        orderID,
					Status:         status,
					OutboxInserted: false,
				}, nil
			}
		case domain.StatusSucceeded, domain.StatusRefunded, domain.StatusRefundFailed:
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit idempotent success callback: %w", err)
			}
			return &domain.CallbackOutcome{
				OrderID:        orderID,
				Status:         status,
				OutboxInserted: false,
			}, nil
		}
	} else {
		switch status {
		case domain.StatusPending, domain.StatusInitiating:
			nextStatus = domain.StatusFailed
			eventType = "PaymentFailed"
			routingKey = "payment.failed"
		default:
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit terminal callback no-op: %w", err)
			}
			return &domain.CallbackOutcome{
				OrderID:        orderID,
				Status:         status,
				OutboxInserted: false,
			}, nil
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $2,
		    amount_minor = CASE WHEN amount_minor = 0 AND $6 > 0 THEN $6 ELSE amount_minor END,
		    momo_trans_id = CASE WHEN $3 <> '' THEN $3 ELSE momo_trans_id END,
		    result_code = $4,
		    result_message = $5,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		nextStatus,
		in.MomoTransID,
		in.ResultCode,
		in.ResultMessage,
		effectiveAmount,
	); err != nil {
		return nil, fmt.Errorf("update payment callback status: %w", err)
	}

	if ensureRefundRow && effectiveAmount > 0 && in.MomoTransID != "" {
		if _, err := insertRefundJobTx(ctx, tx, orderID, orderCode, effectiveAmount, currency, in.MomoTransID, cancelReason, 3); err != nil {
			return nil, err
		}
	}

	eventID, err := newUUID()
	if err != nil {
		return nil, err
	}
	corrID := in.CorrelationID
	if corrID == "" {
		corrID = eventID
	}

	envelope := map[string]any{
		"eventId":       eventID,
		"eventType":     eventType,
		"eventVersion":  1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"aggregateId":   orderID,
		"correlationId": corrID,
		"data": map[string]any{
			"orderId":     orderID,
			"orderCode":   orderCode,
			"userId":      userID,
			"momoOrderId": in.MomoOrderID,
			"momoTransId": in.MomoTransID,
			"amountMinor": amountMinor,
			"currency":    currency,
			"resultCode":  in.ResultCode,
			"message":     in.ResultMessage,
		},
	}
	payloadBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal payment outbox event: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, routing_key, aggregate_id, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)`,
		eventID,
		eventType,
		routingKey,
		orderID,
		string(payloadBytes),
	); err != nil {
		return nil, fmt.Errorf("insert payment outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit callback tx: %w", err)
	}
	return &domain.CallbackOutcome{
		OrderID:        orderID,
		Status:         nextStatus,
		OutboxInserted: true,
	}, nil
}

func (s *Store) CancelPayment(ctx context.Context, orderID, reason string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin cancel payment tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO payments (order_id, status, momo_order_id, momo_request_id, cancel_reason)
		VALUES ($1, $2, $1, 'tombstone:' || $1, $3)
		ON CONFLICT (order_id) DO NOTHING`,
		orderID,
		domain.StatusCancelled,
		reason,
	)
	if err != nil {
		return "", fmt.Errorf("insert payment cancel tombstone: %w", err)
	}
	if tag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit payment cancel tombstone: %w", err)
		}
		return domain.StatusCancelled, nil
	}

	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status
		FROM payments
		WHERE order_id = $1
		FOR UPDATE`,
		orderID,
	).Scan(&status); err != nil {
		return "", fmt.Errorf("lock payment for cancel: %w", err)
	}

	nextStatus := status
	switch status {
	case domain.StatusInitiating, domain.StatusPending:
		nextStatus = domain.StatusCancelled
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    cancel_reason = $3,
			    pay_url = '',
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			nextStatus,
			reason,
		); err != nil {
			return "", fmt.Errorf("mark payment cancelled: %w", err)
		}
	case domain.StatusSucceeded:
		nextStatus = domain.StatusRefundRequired
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    cancel_reason = $3,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			nextStatus,
			reason,
		); err != nil {
			return "", fmt.Errorf("mark succeeded payment refund_required: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit cancel payment: %w", err)
	}
	return nextStatus, nil
}

func (s *Store) GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	var p domain.Payment
	err := s.pool.QueryRow(ctx, `
		SELECT order_id, order_code, user_id, method, status,
		       amount_minor, currency, momo_order_id, momo_request_id, momo_trans_id,
		       pay_url, result_code, result_message, cancel_reason,
		       expires_at, created_at, updated_at
		FROM payments
		WHERE order_id = $1`,
		orderID,
	).Scan(
		&p.OrderID,
		&p.OrderCode,
		&p.UserID,
		&p.Method,
		&p.Status,
		&p.AmountMinor,
		&p.Currency,
		&p.MomoOrderID,
		&p.MomoRequestID,
		&p.MomoTransID,
		&p.PayURL,
		&p.ResultCode,
		&p.ResultMessage,
		&p.CancelReason,
		&p.ExpiresAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("get payment by order_id: %w", err)
	}
	return &p, nil
}

func (s *Store) ClaimPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim outbox tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		WITH due AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND next_attempt_at <= now()
			ORDER BY next_attempt_at ASC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events o
		SET attempts = o.attempts + 1,
		    next_attempt_at = now() + interval '15 seconds'
		FROM due
		WHERE o.id = due.id
		RETURNING o.id, o.event_type, o.routing_key, o.aggregate_id, o.payload::text`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim due outbox rows: %w", err)
	}
	var events []domain.OutboxEvent
	for rows.Next() {
		var ev domain.OutboxEvent
		var payloadText string
		if err := rows.Scan(&ev.ID, &ev.EventType, &ev.RoutingKey, &ev.AggregateID, &payloadText); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan claimed outbox row: %w", err)
		}
		ev.Payload = []byte(payloadText)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate claimed outbox rows: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claimed outbox rows: %w", err)
	}
	return events, nil
}

func (s *Store) MarkOutboxPublished(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = now(),
		    last_error = ''
		WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (s *Store) RecordOutboxFailure(ctx context.Context, id, errMsg string, backoff time.Duration) error {
	ms := backoff.Milliseconds()
	if ms < 500 {
		ms = 500
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE outbox_events
		SET last_error = $2,
		    next_attempt_at = now() + ($3 * interval '1 millisecond')
		WHERE id = $1
		  AND published_at IS NULL`,
		id,
		errMsg,
		ms,
	)
	if err != nil {
		return fmt.Errorf("record outbox failure: %w", err)
	}
	return nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}
