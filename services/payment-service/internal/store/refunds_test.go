package store

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"ecommerce.local/payment-service/internal/domain"
)

var alphanumericRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func TestMigration000002BackfillsExistingRefundRequiredToFailedAndSkipsTombstones(t *testing.T) {
	pool := isolatedPaymentPool(t, "payment_refund_backfill_test")
	ctx := context.Background()

	// Apply migration 000001 manually to simulate pre-existing database state.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text primary key,
			applied_at timestamptz not null default now()
		)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	mig001, err := migrationsFS.ReadFile("migrations/000001_create_payments_and_outbox.up.sql")
	if err != nil {
		t.Fatalf("read 000001: %v", err)
	}
	if _, err := pool.Exec(ctx, string(mig001)); err != nil {
		t.Fatalf("apply 000001: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ('000001_create_payments_and_outbox')`); err != nil {
		t.Fatalf("record 000001: %v", err)
	}

	// Seed (a) captured REFUND_REQUIRED, (b) REFUND_REQUIRED tombstone, (c) SUCCEEDED.
	if _, err := pool.Exec(ctx, `
		INSERT INTO payments (order_id, order_code, user_id, method, status, amount_minor, currency, momo_order_id, momo_request_id, momo_trans_id)
		VALUES
			('ord-bf-1', 'ORD-BF-1', 'user-1', 'MOMO', 'REFUND_REQUIRED', 150000, 'VND', 'ord-bf-1', 'req-bf-1', '998877'),
			('ord-tomb-1', '', '', 'MOMO', 'REFUND_REQUIRED', 0, 'VND', 'ord-tomb-1', 'tombstone:ord-tomb-1', ''),
			('ord-ok-1', 'ORD-OK-1', 'user-1', 'MOMO', 'SUCCEEDED', 200000, 'VND', 'ord-ok-1', 'req-ok-1', '112233')`); err != nil {
		t.Fatalf("seed payments: %v", err)
	}

	// Run Migrate to apply 000002.
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate(000002): %v", err)
	}

	repo := New(pool)

	p1, err := repo.GetByOrderID(ctx, "ord-bf-1")
	if err != nil || p1.Status != domain.StatusRefundFailed {
		t.Fatalf("expected ord-bf-1 to be REFUND_FAILED, got %+v err=%v", p1, err)
	}
	var refundOrderID, refundReqID, refundStatus, lastErr string
	var attempts int
	var nextAttempt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT refund_order_id, refund_request_id, status, attempts, next_attempt_at, last_error
		FROM payment_refunds WHERE order_id = 'ord-bf-1'`).Scan(
		&refundOrderID, &refundReqID, &refundStatus, &attempts, &nextAttempt, &lastErr,
	); err != nil {
		t.Fatalf("query backfilled payment_refunds: %v", err)
	}
	if refundStatus != domain.RefundStatusFailed || attempts != 3 || nextAttempt != nil {
		t.Fatalf("unexpected backfilled refund row: status=%q attempts=%d next=%v", refundStatus, attempts, nextAttempt)
	}
	if !alphanumericRe.MatchString(refundOrderID) || !alphanumericRe.MatchString(refundReqID) {
		t.Fatalf("backfilled IDs must be alphanumeric: orderId=%q reqId=%q", refundOrderID, refundReqID)
	}
	if lastErr == "" {
		t.Fatal("expected non-empty last_error on backfilled refund")
	}

	// Tombstone must NOT be inserted into payment_refunds.
	var tombCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_refunds WHERE order_id = 'ord-tomb-1'`).Scan(&tombCount); err != nil {
		t.Fatalf("count tombstone refund: %v", err)
	}
	if tombCount != 0 {
		t.Fatalf("expected 0 payment_refunds rows for tombstone, got %d", tombCount)
	}

	// SUCCEEDED payment must remain untouched.
	p3, err := repo.GetByOrderID(ctx, "ord-ok-1")
	if err != nil || p3.Status != domain.StatusSucceeded {
		t.Fatalf("expected ord-ok-1 to stay SUCCEEDED, got %+v err=%v", p3, err)
	}
}

func TestRefundLifecycleAuto3AttemptsRotationAndManualRetry(t *testing.T) {
	pool := isolatedPaymentPool(t, "payment_refund_lifecycle_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)
	expires := time.Now().UTC().Add(15 * time.Minute)

	// Create and settle a MoMo payment.
	if _, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID: "ord-ref-1", OrderCode: "ORD-REF-1", UserID: "user-1",
		AmountMinor: 150000, Currency: "VND", MomoRequestID: "req-ref-1", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("BeginCreateSession: %v", err)
	}
	if _, err := repo.CompleteCreateSession(ctx, "ord-ref-1", "https://momo.test/pay/ref1", 0, "OK", nil); err != nil {
		t.Fatalf("CompleteCreateSession: %v", err)
	}
	if _, err := repo.RecordMomoCallback(ctx, domain.CallbackInput{
		MomoOrderID: "ord-ref-1", MomoTransID: "99887766", AmountMinor: 150000, ResultCode: 0, ResultMessage: "Successful.",
	}); err != nil {
		t.Fatalf("RecordMomoCallback: %v", err)
	}

	queued, err := repo.RequestRefund(ctx, "ord-ref-1", "USER_CANCELLED", 3)
	if err != nil || !queued {
		t.Fatalf("RequestRefund: queued=%v err=%v", queued, err)
	}

	// Attempt 1: ambiguous failure -> IDs stay fixed.
	jobs, err := repo.ClaimDueRefunds(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("ClaimDueRefunds #1: len=%d err=%v", len(jobs), err)
	}
	job1 := jobs[0]
	if job1.Attempts != 1 || !alphanumericRe.MatchString(job1.RefundOrderID) || !alphanumericRe.MatchString(job1.RefundRequestID) {
		t.Fatalf("unexpected job1: %+v", job1)
	}
	if job1.RefundOrderID == job1.MomoOrderID {
		t.Fatalf("refund_order_id must differ from original momo_order_id")
	}
	frozenDesc := job1.Description
	if frozenDesc == "" {
		t.Fatal("expected non-empty frozen description")
	}

	outbox, err := repo.CompleteAutoRefundAttempt(ctx, "ord-ref-1", domain.RefundAttemptResult{
		Succeeded:              false,
		ResultCode:             -1,
		LastError:              "timeout",
		RotateIDsOnNextAttempt: false,
	}, time.Millisecond)
	if err != nil || outbox {
		t.Fatalf("CompleteAutoRefundAttempt #1: outbox=%v err=%v", outbox, err)
	}
	time.Sleep(5 * time.Millisecond)

	// Attempt 2: IDs preserved from attempt 1; definite rejection sets RotateIDsOnNextAttempt=true.
	jobs, err = repo.ClaimDueRefunds(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("ClaimDueRefunds #2: len=%d err=%v", len(jobs), err)
	}
	job2 := jobs[0]
	if job2.Attempts != 2 || job2.RefundOrderID != job1.RefundOrderID || job2.RefundRequestID != job1.RefundRequestID {
		t.Fatalf("expected fixed IDs after ambiguous failure: job1=%+v job2=%+v", job1, job2)
	}
	outbox, err = repo.CompleteAutoRefundAttempt(ctx, "ord-ref-1", domain.RefundAttemptResult{
		Succeeded:              false,
		ResultCode:             1080,
		ResultMessage:          "Rejected",
		LastError:              "momo rejected (1080)",
		RotateIDsOnNextAttempt: true,
	}, time.Millisecond)
	if err != nil || outbox {
		t.Fatalf("CompleteAutoRefundAttempt #2: outbox=%v err=%v", outbox, err)
	}
	time.Sleep(5 * time.Millisecond)

	// Attempt 3: fresh alphanumeric IDs minted, frozen description preserved; failure exhausts auto-retries.
	jobs, err = repo.ClaimDueRefunds(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("ClaimDueRefunds #3: len=%d err=%v", len(jobs), err)
	}
	job3 := jobs[0]
	if job3.Attempts != 3 || job3.RefundOrderID == job2.RefundOrderID || job3.RefundRequestID == job2.RefundRequestID {
		t.Fatalf("expected rotated IDs after definite rejection: job2=%+v job3=%+v", job2, job3)
	}
	if !alphanumericRe.MatchString(job3.RefundOrderID) || !alphanumericRe.MatchString(job3.RefundRequestID) {
		t.Fatalf("rotated IDs must be alphanumeric: %+v", job3)
	}
	if job3.Description != frozenDesc {
		t.Fatalf("frozen description mutated: got %q want %q", job3.Description, frozenDesc)
	}
	outbox, err = repo.CompleteAutoRefundAttempt(ctx, "ord-ref-1", domain.RefundAttemptResult{
		Succeeded:              false,
		ResultCode:             -1,
		LastError:              "gateway 503",
		RotateIDsOnNextAttempt: false,
	}, time.Millisecond)
	if err != nil || !outbox {
		t.Fatalf("CompleteAutoRefundAttempt #3: expected outbox=true, got outbox=%v err=%v", outbox, err)
	}

	pFailed, err := repo.GetByOrderID(ctx, "ord-ref-1")
	if err != nil || pFailed.Status != domain.StatusRefundFailed {
		t.Fatalf("expected REFUND_FAILED after 3 attempts, got %+v err=%v", pFailed, err)
	}
	var failedOutboxCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ord-ref-1' AND routing_key = 'payment.refund_failed'`).Scan(&failedOutboxCount); err != nil {
		t.Fatalf("count payment.refund_failed outbox: %v", err)
	}
	if failedOutboxCount != 1 {
		t.Fatalf("expected 1 payment.refund_failed outbox row, got %d", failedOutboxCount)
	}

	// Duplicate RequestRefund must be idempotent no-op and not reset REFUND_FAILED.
	queuedDup, err := repo.RequestRefund(ctx, "ord-ref-1", "DUPLICATE", 3)
	if err != nil || queuedDup {
		t.Fatalf("expected duplicate RequestRefund to be no-op, got queued=%v err=%v", queuedDup, err)
	}

	// Manual retry (attempt 4): keeps frozen description while saving local audit reason.
	beginManual, err := repo.BeginManualRefund(ctx, "ord-ref-1", "Admin retry after MoMo recovery")
	if err != nil || beginManual.AlreadyRefunded || beginManual.Job == nil {
		t.Fatalf("BeginManualRefund: out=%+v err=%v", beginManual, err)
	}
	if beginManual.Job.Attempts != 4 || beginManual.Job.Description != frozenDesc {
		t.Fatalf("unexpected manual job: %+v", beginManual.Job)
	}
	if beginManual.Job.RefundOrderID != job3.RefundOrderID {
		t.Fatalf("expected manual retry after ambiguous failure to keep job3 IDs: got %q want %q", beginManual.Job.RefundOrderID, job3.RefundOrderID)
	}

	// Concurrent manual retry while PROCESSING must fail with ErrRefundInProgress.
	if _, err := repo.BeginManualRefund(ctx, "ord-ref-1", "second"); !errors.Is(err, domain.ErrRefundInProgress) {
		t.Fatalf("expected ErrRefundInProgress, got %v", err)
	}

	outboxManual, err := repo.CompleteManualRefund(ctx, "ord-ref-1", domain.RefundAttemptResult{
		Succeeded:     true,
		RefundTransID: "momo-ref-444",
		ResultCode:    0,
		ResultMessage: "Successful.",
	})
	if err != nil || !outboxManual {
		t.Fatalf("CompleteManualRefund: outbox=%v err=%v", outboxManual, err)
	}

	pRefunded, err := repo.GetByOrderID(ctx, "ord-ref-1")
	if err != nil || pRefunded.Status != domain.StatusRefunded {
		t.Fatalf("expected REFUNDED, got %+v err=%v", pRefunded, err)
	}
	var refundedOutboxCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ord-ref-1' AND routing_key = 'payment.refunded'`).Scan(&refundedOutboxCount); err != nil {
		t.Fatalf("count payment.refunded outbox: %v", err)
	}
	if refundedOutboxCount != 1 {
		t.Fatalf("expected 1 payment.refunded outbox row, got %d", refundedOutboxCount)
	}

	// Repeat manual retry returns idempotent AlreadyRefunded=true.
	repeatManual, err := repo.BeginManualRefund(ctx, "ord-ref-1", "repeat")
	if err != nil || !repeatManual.AlreadyRefunded || repeatManual.RefundTransID != "momo-ref-444" {
		t.Fatalf("expected AlreadyRefunded=true with momo-ref-444, got %+v err=%v", repeatManual, err)
	}
}

func TestRecoverStaleRefundsHandlesUnderAndAtMaxAttempts(t *testing.T) {
	pool := isolatedPaymentPool(t, "payment_refund_stale_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := pool.Exec(ctx, `
		INSERT INTO payments (order_id, order_code, user_id, method, status, amount_minor, currency, momo_order_id, momo_request_id, momo_trans_id)
		VALUES
			('ord-stale-1', 'ORD-S1', 'user-1', 'MOMO', 'REFUND_REQUIRED', 100000, 'VND', 'ord-stale-1', 'req-s1', '111'),
			('ord-stale-3', 'ORD-S3', 'user-1', 'MOMO', 'REFUND_REQUIRED', 100000, 'VND', 'ord-stale-3', 'req-s3', '333')`); err != nil {
		t.Fatalf("seed payments: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment_refunds (order_id, refund_order_id, refund_request_id, amount_minor, currency, momo_trans_id, description, status, attempts, max_auto_attempts, next_attempt_at)
		VALUES
			('ord-stale-1', 'REFUNDSTALE1', 'REFREQSTALE1', 100000, 'VND', '111', 'Hoan tien ORD-S1', 'PROCESSING', 1, 3, now() - interval '1 minute'),
			('ord-stale-3', 'REFUNDSTALE3', 'REFREQSTALE3', 100000, 'VND', '333', 'Hoan tien ORD-S3', 'PROCESSING', 3, 3, now() - interval '1 minute')`); err != nil {
		t.Fatalf("seed stale payment_refunds: %v", err)
	}

	outboxInserted, err := repo.RecoverStaleRefunds(ctx)
	if err != nil || !outboxInserted {
		t.Fatalf("RecoverStaleRefunds: outbox=%v err=%v", outboxInserted, err)
	}

	var status1, status3 string
	if err := pool.QueryRow(ctx, `SELECT status FROM payment_refunds WHERE order_id = 'ord-stale-1'`).Scan(&status1); err != nil {
		t.Fatalf("scan status1: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM payment_refunds WHERE order_id = 'ord-stale-3'`).Scan(&status3); err != nil {
		t.Fatalf("scan status3: %v", err)
	}
	if status1 != domain.RefundStatusPending {
		t.Fatalf("expected ord-stale-1 reset to PENDING, got %q", status1)
	}
	if status3 != domain.RefundStatusFailed {
		t.Fatalf("expected ord-stale-3 finalized to FAILED, got %q", status3)
	}

	p3, err := repo.GetByOrderID(ctx, "ord-stale-3")
	if err != nil || p3.Status != domain.StatusRefundFailed {
		t.Fatalf("expected ord-stale-3 payment REFUND_FAILED, got %+v err=%v", p3, err)
	}
	var outboxCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ord-stale-3' AND routing_key = 'payment.refund_failed'`).Scan(&outboxCount); err != nil {
		t.Fatalf("count stale outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("expected 1 payment.refund_failed outbox row for ord-stale-3, got %d", outboxCount)
	}
}
