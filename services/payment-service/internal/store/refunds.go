package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	routingKeyPaymentRefunded     = "payment.refunded"
	routingKeyPaymentRefundFailed = "payment.refund_failed"
	staleRefundErrorMessage       = "previous attempt timed out or crashed"
)

func (s *Store) RequestRefund(ctx context.Context, orderID, reason string, maxAutoAttempts int) (bool, error) {
	if maxAutoAttempts <= 0 {
		maxAutoAttempts = 3
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin request refund tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO payments (order_id, status, momo_order_id, momo_request_id, cancel_reason)
		VALUES ($1, $2, $1, 'tombstone:' || $1, $3)
		ON CONFLICT (order_id) DO NOTHING`,
		orderID,
		domain.StatusRefundRequired,
		reason,
	)
	if err != nil {
		return false, fmt.Errorf("insert refund_required tombstone: %w", err)
	}
	if tag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit refund_required tombstone: %w", err)
		}
		return false, nil
	}

	var status, orderCode, currency, momoTransID string
	var amountMinor int64
	if err := tx.QueryRow(ctx, `
		SELECT status, order_code, amount_minor, currency, momo_trans_id
		FROM payments
		WHERE order_id = $1
		FOR UPDATE`,
		orderID,
	).Scan(&status, &orderCode, &amountMinor, &currency, &momoTransID); err != nil {
		return false, fmt.Errorf("lock payment for request refund: %w", err)
	}

	if status == domain.StatusRefunded || status == domain.StatusRefundFailed {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit terminal refund request no-op: %w", err)
		}
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $2,
		    cancel_reason = CASE WHEN cancel_reason = '' THEN $3 ELSE cancel_reason END,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		domain.StatusRefundRequired,
		reason,
	); err != nil {
		return false, fmt.Errorf("update payment refund_required: %w", err)
	}

	queued := false
	if amountMinor > 0 && strings.TrimSpace(momoTransID) != "" {
		queued, err = insertRefundJobTx(ctx, tx, orderID, orderCode, amountMinor, currency, momoTransID, reason, maxAutoAttempts)
		if err != nil {
			return false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit request refund tx: %w", err)
	}
	return queued, nil
}

func (s *Store) RecoverStaleRefunds(ctx context.Context) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin recover stale refunds tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE payment_refunds
		SET status = $1,
		    next_attempt_at = now(),
		    last_error = $2,
		    updated_at = now()
		WHERE status = $3
		  AND next_attempt_at <= now()
		  AND attempts < max_auto_attempts`,
		domain.RefundStatusPending,
		staleRefundErrorMessage,
		domain.RefundStatusProcessing,
	); err != nil {
		return false, fmt.Errorf("reset under-max stale refunds: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT r.order_id, p.order_code, p.user_id, p.momo_order_id, r.momo_trans_id,
		       r.refund_order_id, r.amount_minor, r.currency, r.attempts
		FROM payment_refunds r
		JOIN payments p ON p.order_id = r.order_id
		WHERE r.status = $1
		  AND r.next_attempt_at <= now()
		  AND r.attempts >= r.max_auto_attempts
		FOR UPDATE OF r, p`,
		domain.RefundStatusProcessing,
	)
	if err != nil {
		return false, fmt.Errorf("select exhausted stale refunds: %w", err)
	}

	type staleRow struct {
		orderID       string
		orderCode     string
		userID        string
		momoOrderID   string
		momoTransID   string
		refundOrderID string
		amountMinor   int64
		currency      string
		attempts      int
	}
	var exhausted []staleRow
	for rows.Next() {
		var row staleRow
		if err := rows.Scan(
			&row.orderID, &row.orderCode, &row.userID, &row.momoOrderID, &row.momoTransID,
			&row.refundOrderID, &row.amountMinor, &row.currency, &row.attempts,
		); err != nil {
			rows.Close()
			return false, fmt.Errorf("scan exhausted stale refund: %w", err)
		}
		exhausted = append(exhausted, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, fmt.Errorf("iterate exhausted stale refunds: %w", err)
	}
	rows.Close()

	outboxInserted := false
	for _, row := range exhausted {
		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    next_attempt_at = NULL,
			    last_error = $3,
			    updated_at = now()
			WHERE order_id = $1`,
			row.orderID,
			domain.RefundStatusFailed,
			staleRefundErrorMessage,
		); err != nil {
			return false, fmt.Errorf("mark exhausted stale refund failed: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    updated_at = now()
			WHERE order_id = $1
			  AND status <> $3`,
			row.orderID,
			domain.StatusRefundFailed,
			domain.StatusRefunded,
		); err != nil {
			return false, fmt.Errorf("mark payment refund_failed for stale refund: %w", err)
		}
		if err := insertRefundFailedOutboxTx(ctx, tx, refundOutboxMeta{
			orderID:       row.orderID,
			orderCode:     row.orderCode,
			userID:        row.userID,
			momoOrderID:   row.momoOrderID,
			momoTransID:   row.momoTransID,
			refundOrderID: row.refundOrderID,
			amountMinor:   row.amountMinor,
			currency:      row.currency,
			attempts:      row.attempts,
			resultCode:    -1,
			message:       staleRefundErrorMessage,
		}); err != nil {
			return false, err
		}
		outboxInserted = true
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit recover stale refunds tx: %w", err)
	}
	return outboxInserted, nil
}

func (s *Store) ClaimDueRefunds(ctx context.Context, limit int) ([]domain.RefundJob, error) {
	if limit <= 0 {
		limit = 10
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim due refunds tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT r.order_id, p.order_code, p.user_id, p.momo_order_id, r.momo_trans_id,
		       r.refund_order_id, r.refund_request_id, r.description, r.reason,
		       r.currency, r.amount_minor, r.attempts, r.max_auto_attempts, r.rotate_ids_on_next_attempt
		FROM payment_refunds r
		JOIN payments p ON p.order_id = r.order_id
		WHERE r.status = $1
		  AND r.attempts < r.max_auto_attempts
		  AND r.next_attempt_at <= now()
		ORDER BY r.next_attempt_at ASC, r.created_at ASC
		LIMIT $2
		FOR UPDATE OF r SKIP LOCKED`,
		domain.RefundStatusPending,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select due refunds: %w", err)
	}

	type candidate struct {
		job    domain.RefundJob
		rotate bool
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(
			&c.job.OrderID, &c.job.OrderCode, &c.job.UserID, &c.job.MomoOrderID, &c.job.MomoTransID,
			&c.job.RefundOrderID, &c.job.RefundRequestID, &c.job.Description, &c.job.Reason,
			&c.job.Currency, &c.job.AmountMinor, &c.job.Attempts, &c.job.MaxAutoAttempts, &c.rotate,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan due refund: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate due refunds: %w", err)
	}
	rows.Close()

	claimed := make([]domain.RefundJob, 0, len(candidates))
	for _, c := range candidates {
		if c.rotate {
			newOrdID, err := newRefundOrderID()
			if err != nil {
				return nil, err
			}
			newReqID, err := newRefundRequestID()
			if err != nil {
				return nil, err
			}
			c.job.RefundOrderID = newOrdID
			c.job.RefundRequestID = newReqID
		}
		c.job.Attempts++

		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    attempts = $3,
			    refund_order_id = $4,
			    refund_request_id = $5,
			    rotate_ids_on_next_attempt = false,
			    next_attempt_at = now() + interval '2 minutes',
			    updated_at = now()
			WHERE order_id = $1`,
			c.job.OrderID,
			domain.RefundStatusProcessing,
			c.job.Attempts,
			c.job.RefundOrderID,
			c.job.RefundRequestID,
		); err != nil {
			return nil, fmt.Errorf("claim due refund %s: %w", c.job.OrderID, err)
		}
		claimed = append(claimed, c.job)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim due refunds tx: %w", err)
	}
	return claimed, nil
}

func (s *Store) CompleteAutoRefundAttempt(
	ctx context.Context,
	orderID string,
	res domain.RefundAttemptResult,
	retryBackoffBase time.Duration,
) (bool, error) {
	if retryBackoffBase <= 0 {
		retryBackoffBase = 10 * time.Second
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin complete auto refund tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, orderCode, userID, momoOrderID, momoTransID, refundOrderID, currency string
	var amountMinor int64
	var attempts, maxAutoAttempts int
	if err := tx.QueryRow(ctx, `
		SELECT r.status, p.order_code, p.user_id, p.momo_order_id, r.momo_trans_id,
		       r.refund_order_id, r.amount_minor, r.currency, r.attempts, r.max_auto_attempts
		FROM payment_refunds r
		JOIN payments p ON p.order_id = r.order_id
		WHERE r.order_id = $1
		FOR UPDATE OF r, p`,
		orderID,
	).Scan(
		&status, &orderCode, &userID, &momoOrderID, &momoTransID,
		&refundOrderID, &amountMinor, &currency, &attempts, &maxAutoAttempts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domain.ErrRefundNotFound
		}
		return false, fmt.Errorf("lock payment_refund for auto completion: %w", err)
	}

	if status != domain.RefundStatusProcessing {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit non-processing auto refund completion: %w", err)
		}
		return false, nil
	}

	if res.Succeeded {
		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    refund_trans_id = $3,
			    result_code = $4,
			    result_message = $5,
			    last_error = '',
			    rotate_ids_on_next_attempt = false,
			    next_attempt_at = NULL,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.RefundStatusSucceeded,
			res.RefundTransID,
			res.ResultCode,
			res.ResultMessage,
		); err != nil {
			return false, fmt.Errorf("mark payment_refund succeeded: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.StatusRefunded,
		); err != nil {
			return false, fmt.Errorf("mark payment refunded: %w", err)
		}
		if err := insertRefundedOutboxTx(ctx, tx, refundOutboxMeta{
			orderID:       orderID,
			orderCode:     orderCode,
			userID:        userID,
			momoOrderID:   momoOrderID,
			momoTransID:   momoTransID,
			refundOrderID: refundOrderID,
			refundTransID: res.RefundTransID,
			amountMinor:   amountMinor,
			currency:      currency,
			resultCode:    res.ResultCode,
			message:       res.ResultMessage,
		}); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit auto refund success: %w", err)
		}
		return true, nil
	}

	lastErr := truncateStr(strings.TrimSpace(res.LastError), 500)
	if lastErr == "" {
		lastErr = truncateStr(strings.TrimSpace(res.ResultMessage), 500)
	}

	if attempts < maxAutoAttempts {
		shift := attempts - 1
		if shift < 0 {
			shift = 0
		}
		if shift > 10 {
			shift = 10
		}
		backoff := retryBackoffBase * time.Duration(1<<shift)
		ms := backoff.Milliseconds()
		if ms < 1 {
			ms = 1
		}
		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    next_attempt_at = now() + ($3 * interval '1 millisecond'),
			    result_code = $4,
			    result_message = $5,
			    last_error = $6,
			    rotate_ids_on_next_attempt = $7,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.RefundStatusPending,
			ms,
			res.ResultCode,
			res.ResultMessage,
			lastErr,
			res.RotateIDsOnNextAttempt,
		); err != nil {
			return false, fmt.Errorf("reschedule auto refund attempt: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit rescheduled auto refund: %w", err)
		}
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payment_refunds
		SET status = $2,
		    next_attempt_at = NULL,
		    result_code = $3,
		    result_message = $4,
		    last_error = $5,
		    rotate_ids_on_next_attempt = $6,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		domain.RefundStatusFailed,
		res.ResultCode,
		res.ResultMessage,
		lastErr,
		res.RotateIDsOnNextAttempt,
	); err != nil {
		return false, fmt.Errorf("mark payment_refund failed: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $2,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		domain.StatusRefundFailed,
	); err != nil {
		return false, fmt.Errorf("mark payment refund_failed: %w", err)
	}
	if err := insertRefundFailedOutboxTx(ctx, tx, refundOutboxMeta{
		orderID:       orderID,
		orderCode:     orderCode,
		userID:        userID,
		momoOrderID:   momoOrderID,
		momoTransID:   momoTransID,
		refundOrderID: refundOrderID,
		amountMinor:   amountMinor,
		currency:      currency,
		attempts:      attempts,
		resultCode:    res.ResultCode,
		message:       lastErr,
	}); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit auto refund failure: %w", err)
	}
	return true, nil
}

func (s *Store) BeginManualRefund(ctx context.Context, orderID, reason string) (*domain.ManualRefundBeginOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin manual refund tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, refundTransID string
	var rotate bool
	var job domain.RefundJob
	if err := tx.QueryRow(ctx, `
		SELECT r.status, r.refund_trans_id, r.rotate_ids_on_next_attempt,
		       r.order_id, p.order_code, p.user_id, p.momo_order_id, r.momo_trans_id,
		       r.refund_order_id, r.refund_request_id, r.description, r.reason,
		       r.currency, r.amount_minor, r.attempts, r.max_auto_attempts
		FROM payment_refunds r
		JOIN payments p ON p.order_id = r.order_id
		WHERE r.order_id = $1
		FOR UPDATE OF r, p`,
		orderID,
	).Scan(
		&status, &refundTransID, &rotate,
		&job.OrderID, &job.OrderCode, &job.UserID, &job.MomoOrderID, &job.MomoTransID,
		&job.RefundOrderID, &job.RefundRequestID, &job.Description, &job.Reason,
		&job.Currency, &job.AmountMinor, &job.Attempts, &job.MaxAutoAttempts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRefundNotFound
		}
		return nil, fmt.Errorf("lock payment_refund for manual retry: %w", err)
	}

	switch status {
	case domain.RefundStatusSucceeded:
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit already refunded manual retry: %w", err)
		}
		return &domain.ManualRefundBeginOutcome{
			AlreadyRefunded: true,
			RefundTransID:   refundTransID,
		}, nil
	case domain.RefundStatusPending:
		return nil, domain.ErrRefundAutoRetryInProgress
	case domain.RefundStatusProcessing:
		return nil, domain.ErrRefundInProgress
	case domain.RefundStatusFailed:
		if rotate {
			newOrdID, err := newRefundOrderID()
			if err != nil {
				return nil, err
			}
			newReqID, err := newRefundRequestID()
			if err != nil {
				return nil, err
			}
			job.RefundOrderID = newOrdID
			job.RefundRequestID = newReqID
		}
		job.Attempts++
		cleanReason := strings.TrimSpace(reason)
		if cleanReason != "" {
			job.Reason = cleanReason
		}

		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    attempts = $3,
			    refund_order_id = $4,
			    refund_request_id = $5,
			    rotate_ids_on_next_attempt = false,
			    next_attempt_at = now() + interval '2 minutes',
			    reason = CASE WHEN $6 <> '' THEN $6 ELSE reason END,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.RefundStatusProcessing,
			job.Attempts,
			job.RefundOrderID,
			job.RefundRequestID,
			cleanReason,
		); err != nil {
			return nil, fmt.Errorf("mark manual refund processing: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit begin manual refund: %w", err)
		}
		return &domain.ManualRefundBeginOutcome{
			AlreadyRefunded: false,
			Job:             &job,
		}, nil
	default:
		return nil, domain.ErrRefundNotFound
	}
}

func (s *Store) CompleteManualRefund(ctx context.Context, orderID string, res domain.RefundAttemptResult) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin complete manual refund tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, orderCode, userID, momoOrderID, momoTransID, refundOrderID, currency string
	var amountMinor int64
	if err := tx.QueryRow(ctx, `
		SELECT r.status, p.order_code, p.user_id, p.momo_order_id, r.momo_trans_id,
		       r.refund_order_id, r.amount_minor, r.currency
		FROM payment_refunds r
		JOIN payments p ON p.order_id = r.order_id
		WHERE r.order_id = $1
		FOR UPDATE OF r, p`,
		orderID,
	).Scan(
		&status, &orderCode, &userID, &momoOrderID, &momoTransID,
		&refundOrderID, &amountMinor, &currency,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domain.ErrRefundNotFound
		}
		return false, fmt.Errorf("lock payment_refund for manual completion: %w", err)
	}

	if res.Succeeded {
		if _, err := tx.Exec(ctx, `
			UPDATE payment_refunds
			SET status = $2,
			    refund_trans_id = $3,
			    result_code = $4,
			    result_message = $5,
			    last_error = '',
			    rotate_ids_on_next_attempt = false,
			    next_attempt_at = NULL,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.RefundStatusSucceeded,
			res.RefundTransID,
			res.ResultCode,
			res.ResultMessage,
		); err != nil {
			return false, fmt.Errorf("mark manual refund succeeded: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE payments
			SET status = $2,
			    updated_at = now()
			WHERE order_id = $1`,
			orderID,
			domain.StatusRefunded,
		); err != nil {
			return false, fmt.Errorf("mark payment refunded on manual retry: %w", err)
		}
		if err := insertRefundedOutboxTx(ctx, tx, refundOutboxMeta{
			orderID:       orderID,
			orderCode:     orderCode,
			userID:        userID,
			momoOrderID:   momoOrderID,
			momoTransID:   momoTransID,
			refundOrderID: refundOrderID,
			refundTransID: res.RefundTransID,
			amountMinor:   amountMinor,
			currency:      currency,
			resultCode:    res.ResultCode,
			message:       res.ResultMessage,
		}); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit manual refund success: %w", err)
		}
		return true, nil
	}

	lastErr := truncateStr(strings.TrimSpace(res.LastError), 500)
	if lastErr == "" {
		lastErr = truncateStr(strings.TrimSpace(res.ResultMessage), 500)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payment_refunds
		SET status = $2,
		    next_attempt_at = NULL,
		    result_code = $3,
		    result_message = $4,
		    last_error = $5,
		    rotate_ids_on_next_attempt = $6,
		    updated_at = now()
		WHERE order_id = $1`,
		orderID,
		domain.RefundStatusFailed,
		res.ResultCode,
		res.ResultMessage,
		lastErr,
		res.RotateIDsOnNextAttempt,
	); err != nil {
		return false, fmt.Errorf("mark manual refund failed: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $2,
		    updated_at = now()
		WHERE order_id = $1
		  AND status <> $3`,
		orderID,
		domain.StatusRefundFailed,
		domain.StatusRefunded,
	); err != nil {
		return false, fmt.Errorf("keep payment refund_failed on manual failure: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit manual refund failure: %w", err)
	}
	return false, nil
}

func insertRefundJobTx(
	ctx context.Context,
	tx pgx.Tx,
	orderID, orderCode string,
	amountMinor int64,
	currency, momoTransID, reason string,
	maxAutoAttempts int,
) (bool, error) {
	if currency == "" {
		currency = "VND"
	}
	refundOrderID, err := newRefundOrderID()
	if err != nil {
		return false, err
	}
	refundRequestID, err := newRefundRequestID()
	if err != nil {
		return false, err
	}
	desc := buildFrozenDescription(orderCode, orderID)

	tag, err := tx.Exec(ctx, `
		INSERT INTO payment_refunds (
			order_id, refund_order_id, refund_request_id,
			amount_minor, currency, momo_trans_id,
			description, reason, status, attempts, max_auto_attempts, next_attempt_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, now())
		ON CONFLICT (order_id) DO NOTHING`,
		orderID,
		refundOrderID,
		refundRequestID,
		amountMinor,
		currency,
		momoTransID,
		desc,
		strings.TrimSpace(reason),
		domain.RefundStatusPending,
		maxAutoAttempts,
	)
	if err != nil {
		return false, fmt.Errorf("insert payment_refunds row: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

type refundOutboxMeta struct {
	orderID       string
	orderCode     string
	userID        string
	momoOrderID   string
	momoTransID   string
	refundOrderID string
	refundTransID string
	amountMinor   int64
	currency      string
	attempts      int
	resultCode    int32
	message       string
}

func insertRefundedOutboxTx(ctx context.Context, tx pgx.Tx, m refundOutboxMeta) error {
	eventID, err := newUUID()
	if err != nil {
		return err
	}
	envelope := map[string]any{
		"eventId":       eventID,
		"eventType":     "PaymentRefunded",
		"eventVersion":  1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"aggregateId":   m.orderID,
		"correlationId": eventID,
		"data": map[string]any{
			"orderId":       m.orderID,
			"orderCode":     m.orderCode,
			"userId":        m.userID,
			"momoOrderId":   m.momoOrderID,
			"momoTransId":   m.momoTransID,
			"refundOrderId": m.refundOrderID,
			"refundTransId": m.refundTransID,
			"amountMinor":   m.amountMinor,
			"currency":      m.currency,
			"resultCode":    m.resultCode,
			"message":       m.message,
		},
	}
	payloadBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal PaymentRefunded outbox event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, routing_key, aggregate_id, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)`,
		eventID,
		"PaymentRefunded",
		routingKeyPaymentRefunded,
		m.orderID,
		string(payloadBytes),
	); err != nil {
		return fmt.Errorf("insert PaymentRefunded outbox event: %w", err)
	}
	return nil
}

func insertRefundFailedOutboxTx(ctx context.Context, tx pgx.Tx, m refundOutboxMeta) error {
	eventID, err := newUUID()
	if err != nil {
		return err
	}
	envelope := map[string]any{
		"eventId":       eventID,
		"eventType":     "PaymentRefundFailed",
		"eventVersion":  1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"aggregateId":   m.orderID,
		"correlationId": eventID,
		"data": map[string]any{
			"orderId":       m.orderID,
			"orderCode":     m.orderCode,
			"userId":        m.userID,
			"momoOrderId":   m.momoOrderID,
			"momoTransId":   m.momoTransID,
			"refundOrderId": m.refundOrderID,
			"amountMinor":   m.amountMinor,
			"currency":      m.currency,
			"attempts":      m.attempts,
			"resultCode":    m.resultCode,
			"message":       m.message,
		},
	}
	payloadBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal PaymentRefundFailed outbox event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, routing_key, aggregate_id, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)`,
		eventID,
		"PaymentRefundFailed",
		routingKeyPaymentRefundFailed,
		m.orderID,
		string(payloadBytes),
	); err != nil {
		return fmt.Errorf("insert PaymentRefundFailed outbox event: %w", err)
	}
	return nil
}

func newRefundOrderID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate refund_order_id: %w", err)
	}
	return "REFUND" + strings.ToUpper(hex.EncodeToString(b[:])), nil
}

func newRefundRequestID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate refund_request_id: %w", err)
	}
	return "REFREQ" + strings.ToUpper(hex.EncodeToString(b[:])), nil
}

func buildFrozenDescription(orderCode, orderID string) string {
	ref := strings.TrimSpace(orderCode)
	if ref == "" {
		ref = strings.TrimSpace(orderID)
	}
	return truncateStr("Hoan tien don hang "+ref, 255)
}

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
