package store

import (
	"context"
	"os"
	"testing"

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
