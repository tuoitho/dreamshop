package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"ecommerce.local/inventory-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	return pool
}

func TestStoreIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	repo := New(pool)
	stock, err := repo.Get(ctx, "dt-1")
	if err != nil {
		t.Fatalf("get seeded stock: %v", err)
	}
	if stock.Quantity != 3 || stock.Reserved != 0 || stock.AvailableQuantity() != 3 {
		t.Fatalf("unexpected seeded stock: %+v", stock)
	}

	missing, err := repo.Get(ctx, "missing-product")
	if err != nil {
		t.Fatalf("get missing stock: %v", err)
	}
	if missing.ProductID != "missing-product" || missing.AvailableQuantity() != 0 {
		t.Fatalf("unexpected missing stock: %+v", missing)
	}
}

func isolatedInventoryPool(t *testing.T, schemaName string) *pgxpool.Pool {
	t.Helper()
	adminPool := testPool(t)
	ctx := context.Background()

	if _, err := adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	})

	poolCfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
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

func TestAdminStockOperations(t *testing.T) {
	pool := isolatedInventoryPool(t, "inventory_admin_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	// SetQuantity creates missing row
	s1, err := repo.SetQuantity(ctx, "prod-a", 12)
	if err != nil {
		t.Fatalf("SetQuantity(prod-a, 12): %v", err)
	}
	if s1.Quantity != 12 || s1.Reserved != 0 || s1.AvailableQuantity() != 12 || s1.UpdatedAt.IsZero() {
		t.Fatalf("unexpected s1: %+v", s1)
	}

	// Simulate 4 reserved units on prod-a
	if _, err := pool.Exec(ctx, "UPDATE stock SET reserved = 4 WHERE product_id = 'prod-a'"); err != nil {
		t.Fatalf("set reserved: %v", err)
	}

	// SetQuantity below reserved must fail with domain.ErrStockBelowReserved
	if _, err := repo.SetQuantity(ctx, "prod-a", 3); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on SetQuantity(3 < 4), got %v", err)
	}

	// AdjustQuantity positive on missing row creates row
	s2, err := repo.AdjustQuantity(ctx, "prod-b", 5)
	if err != nil || s2.Quantity != 5 {
		t.Fatalf("AdjustQuantity(prod-b, +5): %v, %+v", err, s2)
	}

	// AdjustQuantity negative on missing row fails with domain.ErrStockBelowReserved
	if _, err := repo.AdjustQuantity(ctx, "prod-missing", -1); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on missing row negative adjust, got %v", err)
	}

	// AdjustQuantity negative below reserved fails with domain.ErrStockBelowReserved
	if _, err := repo.AdjustQuantity(ctx, "prod-a", -9); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on AdjustQuantity(12 - 9 < 4), got %v", err)
	}

	// AdjustQuantity negative respecting reserved succeeds (12 - 6 = 6 >= 4)
	s1After, err := repo.AdjustQuantity(ctx, "prod-a", -6)
	if err != nil || s1After.Quantity != 6 || s1After.AvailableQuantity() != 2 {
		t.Fatalf("AdjustQuantity(prod-a, -6): %v, %+v", err, s1After)
	}

	// BatchGet preserves requested order and returns zero-stock for missing IDs
	batch, err := repo.BatchGet(ctx, []string{"prod-b", "none", "prod-a"})
	if err != nil || len(batch) != 3 {
		t.Fatalf("BatchGet: %v, %+v", err, batch)
	}
	if batch[0].ProductID != "prod-b" || batch[1].ProductID != "none" || batch[1].Quantity != 0 || batch[2].ProductID != "prod-a" || batch[2].Reserved != 4 {
		t.Fatalf("unexpected BatchGet order/values: %+v", batch)
	}

	// ListAll returns persisted rows
	all, err := repo.ListAll(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAll: %v, len=%d", err, len(all))
	}
}
