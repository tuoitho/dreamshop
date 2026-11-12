package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ecommerce.local/inventory-service/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestReserveStockAggregatesDuplicateSkuIDsAndKeepsDistinctVariantsIndependent(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_reserve_dup_sku_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "p-1-v1", "p-1", 3); err != nil {
		t.Fatalf("SetQuantity(p-1-v1): %v", err)
	}
	if _, err := repo.SetQuantity(ctx, "p-1-v2", "p-1", 3); err != nil {
		t.Fatalf("SetQuantity(p-1-v2): %v", err)
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	_, err := repo.ReserveStock(ctx, "ord-dup-1", []domain.ReservationLine{
		{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2},
		{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2},
	}, expiresAt)
	var insuffErr *domain.InsufficientStockError
	if !errors.As(err, &insuffErr) {
		t.Fatalf("expected InsufficientStockError, got %v", err)
	}
	if insuffErr.SkuID != "p-1-v1" || insuffErr.ProductID != "p-1" || insuffErr.Available != 3 || insuffErr.Requested != 4 {
		t.Fatalf("unexpected InsufficientStockError: %+v", insuffErr)
	}

	stV1, err := repo.Get(ctx, "p-1-v1")
	if err != nil {
		t.Fatalf("Get(p-1-v1): %v", err)
	}
	if stV1.Reserved != 0 || stV1.Quantity != 3 {
		t.Fatalf("expected rollback with Reserved=0 Quantity=3, got %+v", stV1)
	}

	status, err := repo.ReserveStock(ctx, "ord-distinct-1", []domain.ReservationLine{
		{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2},
		{SkuID: "p-1-v2", ProductID: "p-1", Quantity: 2},
	}, expiresAt)
	if err != nil {
		t.Fatalf("ReserveStock distinct variants: %v", err)
	}
	if status != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE", status)
	}

	stV1, _ = repo.Get(ctx, "p-1-v1")
	stV2, _ := repo.Get(ctx, "p-1-v2")
	if stV1.Reserved != 2 || stV2.Reserved != 2 {
		t.Fatalf("expected both variants reserved=2, got v1=%+v v2=%+v", stV1, stV2)
	}
}

func TestReserveStockIdempotentReplay(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_reserve_replay_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-replay", "p-replay", 5); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	lines := []domain.ReservationLine{{SkuID: "sku-replay", ProductID: "p-replay", Quantity: 2}}

	st1, err := repo.ReserveStock(ctx, "ord-2", lines, expiresAt)
	if err != nil || st1 != "ACTIVE" {
		t.Fatalf("first ReserveStock: status=%q err=%v", st1, err)
	}
	st2, err := repo.ReserveStock(ctx, "ord-2", lines, expiresAt)
	if err != nil || st2 != "ACTIVE" {
		t.Fatalf("replay ReserveStock: status=%q err=%v", st2, err)
	}

	stock, err := repo.Get(ctx, "sku-replay")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stock.Reserved != 2 || stock.Quantity != 5 {
		t.Fatalf("expected Reserved=2 Quantity=5 after replay, got %+v", stock)
	}
}

func TestReleaseBeforeReserveWritesTombstoneBlockingLateReserve(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_tombstone_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-late", "p-late", 5); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}

	relStatus, err := repo.ReleaseReservation(ctx, "ord-late", "TIMEOUT")
	if err != nil {
		t.Fatalf("ReleaseReservation tombstone: %v", err)
	}
	if relStatus != "RELEASED" {
		t.Fatalf("ReleaseReservation status = %q, want RELEASED", relStatus)
	}

	_, err = repo.ReserveStock(ctx, "ord-late", []domain.ReservationLine{
		{SkuID: "sku-late", ProductID: "p-late", Quantity: 2},
	}, time.Now().UTC().Add(15*time.Minute))
	if !errors.Is(err, domain.ErrReservationAlreadyReleased) {
		t.Fatalf("expected ErrReservationAlreadyReleased, got %v", err)
	}

	stock, err := repo.Get(ctx, "sku-late")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stock.Reserved != 0 || stock.Quantity != 5 {
		t.Fatalf("expected Reserved=0 Quantity=5, got %+v", stock)
	}
}

func TestConcurrentLateReserveAndReleaseNeverRaisesUniqueViolation(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_concurrent_tombstone_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-race", "p-race", 100); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}

	for i := range 20 {
		orderID := fmt.Sprintf("ord-race-%d", i)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var reserveErr, releaseErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, reserveErr = repo.ReserveStock(ctx, orderID, []domain.ReservationLine{
				{SkuID: "sku-race", ProductID: "p-race", Quantity: 2},
			}, time.Now().UTC().Add(15*time.Minute))
		}()
		go func() {
			defer wg.Done()
			<-start
			_, releaseErr = repo.ReleaseReservation(ctx, orderID, "CONCURRENT_CANCEL")
		}()
		close(start)
		wg.Wait()

		for _, err := range []error{reserveErr, releaseErr} {
			if err == nil || errors.Is(err, domain.ErrReservationAlreadyReleased) {
				continue
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				t.Fatalf("iteration %d raised unique violation 23505: %v", i, err)
			}
			t.Fatalf("iteration %d unexpected error: %v", i, err)
		}
	}

	stock, err := repo.Get(ctx, "sku-race")
	if err != nil {
		t.Fatalf("Get(sku-race): %v", err)
	}
	if stock.Reserved != 0 || stock.Quantity != 100 {
		t.Fatalf("expected all 20 concurrent reserve+release pairs to settle at Reserved=0 Quantity=100, got %+v", stock)
	}
}

func TestConfirmReservationDeductsQuantityAndReservedIdempotently(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_confirm_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-conf", "p-conf", 5); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}

	if _, err := repo.ReserveStock(ctx, "ord-conf", []domain.ReservationLine{
		{SkuID: "sku-conf", ProductID: "p-conf", Quantity: 2},
	}, time.Now().UTC().Add(15*time.Minute)); err != nil {
		t.Fatalf("ReserveStock: %v", err)
	}

	st1, err := repo.ConfirmReservation(ctx, "ord-conf")
	if err != nil || st1 != "CONFIRMED" {
		t.Fatalf("first ConfirmReservation: status=%q err=%v", st1, err)
	}
	st2, err := repo.ConfirmReservation(ctx, "ord-conf")
	if err != nil || st2 != "CONFIRMED" {
		t.Fatalf("second ConfirmReservation: status=%q err=%v", st2, err)
	}

	stock, err := repo.Get(ctx, "sku-conf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stock.Quantity != 3 || stock.Reserved != 0 {
		t.Fatalf("expected Quantity=3 Reserved=0, got %+v", stock)
	}
}

func TestConcurrentCrossOrderReservationsDoNotDeadlock(t *testing.T) {
	pool := isolatedInventoryPool(t, "inv_deadlock_order_test")
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-a", "p-a", 50); err != nil {
		t.Fatalf("SetQuantity(sku-a): %v", err)
	}
	if _, err := repo.SetQuantity(ctx, "sku-b", "p-b", 50); err != nil {
		t.Fatalf("SetQuantity(sku-b): %v", err)
	}

	const workers = 20
	start := make(chan struct{})
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(idx int) {
			defer wg.Done()
			<-start
			items := []domain.ReservationLine{
				{SkuID: "sku-a", ProductID: "p-a", Quantity: 1},
				{SkuID: "sku-b", ProductID: "p-b", Quantity: 1},
			}
			if idx%2 == 1 {
				items = []domain.ReservationLine{
					{SkuID: "sku-b", ProductID: "p-b", Quantity: 1},
					{SkuID: "sku-a", ProductID: "p-a", Quantity: 1},
				}
			}
			_, err := repo.ReserveStock(ctx, fmt.Sprintf("ord-dl-%d", idx), items, time.Now().UTC().Add(15*time.Minute))
			errCh <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent cross-order ReserveStock failed: %v", err)
		}
	}

	stA, _ := repo.Get(ctx, "sku-a")
	stB, _ := repo.Get(ctx, "sku-b")
	if stA.Reserved != workers || stB.Reserved != workers {
		t.Fatalf("expected Reserved=%d on both SKUs, got a=%+v b=%+v", workers, stA, stB)
	}
}

func newRestockRepo(t *testing.T, dbName string) (*Store, context.Context) {
	t.Helper()
	pool := isolatedInventoryPool(t, dbName)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)
	if _, err := repo.SetQuantity(ctx, "sku-rs", "p-rs", 10); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}
	return repo, ctx
}

func reserveRestockOrder(t *testing.T, ctx context.Context, repo *Store, orderID string) {
	t.Helper()
	if _, err := repo.ReserveStock(ctx, orderID, []domain.ReservationLine{
		{SkuID: "sku-rs", ProductID: "p-rs", Quantity: 3},
	}, time.Now().UTC().Add(15*time.Minute)); err != nil {
		t.Fatalf("ReserveStock: %v", err)
	}
}

func assertRestockStock(t *testing.T, ctx context.Context, repo *Store, wantQty, wantReserved int32) {
	t.Helper()
	stock, err := repo.Get(ctx, "sku-rs")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stock.Quantity != wantQty || stock.Reserved != wantReserved {
		t.Fatalf("expected Quantity=%d Reserved=%d, got %+v", wantQty, wantReserved, stock)
	}
}

func TestRestockConfirmedReservationReturnsQuantity(t *testing.T) {
	repo, ctx := newRestockRepo(t, "inv_restock_confirmed_test")
	reserveRestockOrder(t, ctx, repo, "ord-rs-1")
	if _, err := repo.ConfirmReservation(ctx, "ord-rs-1"); err != nil {
		t.Fatalf("ConfirmReservation: %v", err)
	}
	assertRestockStock(t, ctx, repo, 7, 0)

	st, err := repo.RestockReservation(ctx, "ord-rs-1", "ADMIN_CANCELLED")
	if err != nil || st != "RESTOCKED" {
		t.Fatalf("RestockReservation: status=%q err=%v", st, err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)

	st, err = repo.RestockReservation(ctx, "ord-rs-1", "ADMIN_CANCELLED")
	if err != nil || st != "RESTOCKED" {
		t.Fatalf("replayed RestockReservation: status=%q err=%v", st, err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)
}

func TestRestockActiveReservationReleasesInsteadOfDeducting(t *testing.T) {
	repo, ctx := newRestockRepo(t, "inv_restock_active_test")
	reserveRestockOrder(t, ctx, repo, "ord-rs-2")
	assertRestockStock(t, ctx, repo, 10, 3)

	st, err := repo.RestockReservation(ctx, "ord-rs-2", "USER_CANCELLED")
	if err != nil || st != "RELEASED" {
		t.Fatalf("RestockReservation: status=%q err=%v", st, err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)

	st, err = repo.ConfirmReservation(ctx, "ord-rs-2")
	if err != nil || st != "RELEASED" {
		t.Fatalf("late ConfirmReservation: status=%q err=%v", st, err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)
}

func TestConfirmAfterRestockIsNoop(t *testing.T) {
	repo, ctx := newRestockRepo(t, "inv_confirm_after_restock_test")
	reserveRestockOrder(t, ctx, repo, "ord-rs-3")
	if _, err := repo.ConfirmReservation(ctx, "ord-rs-3"); err != nil {
		t.Fatalf("ConfirmReservation: %v", err)
	}
	if _, err := repo.RestockReservation(ctx, "ord-rs-3", "ADMIN_CANCELLED"); err != nil {
		t.Fatalf("RestockReservation: %v", err)
	}

	st, err := repo.ConfirmReservation(ctx, "ord-rs-3")
	if err != nil || st != "RESTOCKED" {
		t.Fatalf("ConfirmReservation after restock: status=%q err=%v", st, err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)
}

func TestRestockWithoutReservationWritesTombstone(t *testing.T) {
	repo, ctx := newRestockRepo(t, "inv_restock_tombstone_test")

	st, err := repo.RestockReservation(ctx, "ord-rs-4", "ADMIN_CANCELLED")
	if err != nil || st != "RELEASED" {
		t.Fatalf("RestockReservation: status=%q err=%v", st, err)
	}

	_, err = repo.ReserveStock(ctx, "ord-rs-4", []domain.ReservationLine{
		{SkuID: "sku-rs", ProductID: "p-rs", Quantity: 3},
	}, time.Now().UTC().Add(15*time.Minute))
	if !errors.Is(err, domain.ErrReservationAlreadyReleased) {
		t.Fatalf("expected ErrReservationAlreadyReleased, got %v", err)
	}
	assertRestockStock(t, ctx, repo, 10, 0)
}
