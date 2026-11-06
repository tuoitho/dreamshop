package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func isolatedPaymentPool(t *testing.T, schemaName string) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	})

	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	poolCfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("connect isolated schema pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestCreatePaymentSessionTwoPhaseCommitReplayAndTombstone(t *testing.T) {
	pool := isolatedPaymentPool(t, "payment_two_phase_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)
	expires := time.Now().UTC().Add(15 * time.Minute)

	// 1. Happy two-phase session creation + idempotent replay
	begin1, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID:       "ord-1",
		OrderCode:     "ORD-20261105-000001",
		UserID:        "user-1",
		AmountMinor:   180000,
		Currency:      "VND",
		MomoRequestID: "req-1",
		ExpiresAt:     expires,
	})
	if err != nil || !begin1.ProceedToGateway {
		t.Fatalf("BeginCreateSession(ord-1): out=%+v err=%v", begin1, err)
	}

	savedURL, err := repo.CompleteCreateSession(ctx, "ord-1", "https://momo.test/pay/ord-1", 0, "Successful.", nil)
	if err != nil || savedURL != "https://momo.test/pay/ord-1" {
		t.Fatalf("CompleteCreateSession(ord-1): url=%q err=%v", savedURL, err)
	}

	replay, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID:       "ord-1",
		OrderCode:     "ORD-20261105-000001",
		UserID:        "user-1",
		AmountMinor:   180000,
		Currency:      "VND",
		MomoRequestID: "req-1-retry",
		ExpiresAt:     expires,
	})
	if err != nil {
		t.Fatalf("replay BeginCreateSession(ord-1): %v", err)
	}
	if replay.ProceedToGateway || replay.PayURL != "https://momo.test/pay/ord-1" || replay.Status != "PENDING" {
		t.Fatalf("expected cached PENDING payURL on replay, got %+v", replay)
	}

	// 2. Pre-arrival CANCELLED tombstone blocks late BeginCreateSession
	if _, err := repo.CancelPayment(ctx, "ord-tomb", "TIMEOUT"); err != nil {
		t.Fatalf("CancelPayment(ord-tomb): %v", err)
	}
	_, err = repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID:       "ord-tomb",
		OrderCode:     "ORD-TOMB",
		UserID:        "user-1",
		AmountMinor:   100000,
		Currency:      "VND",
		MomoRequestID: "req-tomb",
		ExpiresAt:     expires,
	})
	if !errors.Is(err, domain.ErrPaymentAlreadyCancelled) {
		t.Fatalf("expected ErrPaymentAlreadyCancelled on tombstoned order, got %v", err)
	}

	// 3. Mid-flight CancelPayment between TX #A and TX #B keeps CANCELLED and hides payUrl
	beginMid, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID:       "ord-mid",
		OrderCode:     "ORD-MID",
		UserID:        "user-1",
		AmountMinor:   120000,
		Currency:      "VND",
		MomoRequestID: "req-mid",
		ExpiresAt:     expires,
	})
	if err != nil || !beginMid.ProceedToGateway {
		t.Fatalf("BeginCreateSession(ord-mid): out=%+v err=%v", beginMid, err)
	}
	if _, err := repo.CancelPayment(ctx, "ord-mid", "SAGA_CANCEL"); err != nil {
		t.Fatalf("mid-flight CancelPayment(ord-mid): %v", err)
	}
	_, err = repo.CompleteCreateSession(ctx, "ord-mid", "https://momo.test/pay/ord-mid", 0, "Successful.", nil)
	if !errors.Is(err, domain.ErrPaymentAlreadyCancelled) {
		t.Fatalf("expected ErrPaymentAlreadyCancelled on mid-flight cancel, got %v", err)
	}
	pMid, err := repo.GetByOrderID(ctx, "ord-mid")
	if err != nil || pMid.Status != "CANCELLED" || pMid.PayURL != "" {
		t.Fatalf("expected CANCELLED with empty PayURL, got %+v err=%v", pMid, err)
	}
}

func TestHandleCallbackIdempotencyAndLatePaymentRefundRequired(t *testing.T) {
	pool := isolatedPaymentPool(t, "payment_callback_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)
	expires := time.Now().UTC().Add(15 * time.Minute)

	// Case 1: Concurrent resultCode=0 callbacks produce SUCCEEDED and exactly 1 outbox event
	if _, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID: "ord-cb-1", OrderCode: "ORD-CB-1", UserID: "user-1",
		AmountMinor: 200000, Currency: "VND", MomoRequestID: "req-cb-1", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("BeginCreateSession: %v", err)
	}
	if _, err := repo.CompleteCreateSession(ctx, "ord-cb-1", "https://momo.test/pay/1", 0, "OK", nil); err != nil {
		t.Fatalf("CompleteCreateSession: %v", err)
	}

	const workers = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			_, _ = repo.RecordMomoCallback(ctx, domain.CallbackInput{
				MomoOrderID:   "ord-cb-1",
				MomoTransID:   "trans-111",
				AmountMinor:   200000,
				ResultCode:    0,
				ResultMessage: "Successful.",
				CorrelationID: "trace-cb-1",
			})
		}()
	}
	close(start)
	wg.Wait()

	p1, err := repo.GetByOrderID(ctx, "ord-cb-1")
	if err != nil || p1.Status != "SUCCEEDED" || p1.MomoTransID != "trans-111" {
		t.Fatalf("expected SUCCEEDED payment, got %+v err=%v", p1, err)
	}
	var outboxCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ord-cb-1' AND routing_key = 'payment.succeeded'").Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox_events: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("expected exactly 1 payment.succeeded outbox row, got %d", outboxCount)
	}

	// Subsequent resultCode=1006 never downgrades SUCCEEDED
	resAfterFail, err := repo.RecordMomoCallback(ctx, domain.CallbackInput{
		MomoOrderID:   "ord-cb-1",
		MomoTransID:   "trans-112",
		AmountMinor:   200000,
		ResultCode:    1006,
		ResultMessage: "Cancelled by user.",
	})
	if err != nil || resAfterFail.Status != "SUCCEEDED" {
		t.Fatalf("expected SUCCEEDED to stay immutable on late failure callback, got %+v err=%v", resAfterFail, err)
	}

	// Case 2: Late payment on CANCELLED transitions CANCELLED -> REFUND_REQUIRED + writes payment.succeeded outbox
	if _, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID: "ord-late-pay", OrderCode: "ORD-LATE", UserID: "user-1",
		AmountMinor: 350000, Currency: "VND", MomoRequestID: "req-late", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("BeginCreateSession(ord-late-pay): %v", err)
	}
	if _, err := repo.CompleteCreateSession(ctx, "ord-late-pay", "https://momo.test/pay/late", 0, "OK", nil); err != nil {
		t.Fatalf("CompleteCreateSession(ord-late-pay): %v", err)
	}
	if _, err := repo.CancelPayment(ctx, "ord-late-pay", "PAYMENT_EXPIRED"); err != nil {
		t.Fatalf("CancelPayment(ord-late-pay): %v", err)
	}

	lateRes, err := repo.RecordMomoCallback(ctx, domain.CallbackInput{
		MomoOrderID:   "ord-late-pay",
		MomoTransID:   "trans-late-999",
		AmountMinor:   350000,
		ResultCode:    0,
		ResultMessage: "Successful.",
	})
	if err != nil || lateRes.Status != "REFUND_REQUIRED" || !lateRes.OutboxInserted {
		t.Fatalf("expected REFUND_REQUIRED + OutboxInserted=true on late payment, got %+v err=%v", lateRes, err)
	}
}
