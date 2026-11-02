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

func TestStoreIntegration(t *testing.T) {
	pool := isolatedInventoryPool(t, "inventory_store_integration_test")
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
	inStock, err := repo.Get(ctx, "dt-1-v1")
	if err != nil {
		t.Fatalf("get seeded stock dt-1-v1: %v", err)
	}
	if inStock.SkuID != "dt-1-v1" || inStock.ProductID != "dt-1" || inStock.AvailableQuantity() <= 0 {
		t.Fatalf("unexpected seeded stock dt-1-v1: %+v", inStock)
	}

	outOfStockVariant, err := repo.Get(ctx, "dt-1-v3")
	if err != nil {
		t.Fatalf("get seeded stock dt-1-v3: %v", err)
	}
	if outOfStockVariant.SkuID != "dt-1-v3" || outOfStockVariant.ProductID != "dt-1" || outOfStockVariant.AvailableQuantity() != 0 {
		t.Fatalf("expected dt-1-v3 to be out of stock alongside in-stock siblings, got %+v", outOfStockVariant)
	}

	missing, err := repo.Get(ctx, "missing-sku")
	if err != nil {
		t.Fatalf("get missing stock: %v", err)
	}
	if missing.SkuID != "missing-sku" || missing.AvailableQuantity() != 0 {
		t.Fatalf("unexpected missing stock: %+v", missing)
	}
}

func TestStoreSkuSetAdjustAndBatchGetOrder(t *testing.T) {
	pool := isolatedInventoryPool(t, "inventory_sku_ops_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	s1, err := repo.SetQuantity(ctx, "dt-1-v1", "dt-1", 10)
	if err != nil {
		t.Fatalf("SetQuantity(dt-1-v1): %v", err)
	}
	if s1.SkuID != "dt-1-v1" || s1.ProductID != "dt-1" || s1.Quantity != 10 || s1.AvailableQuantity() != 10 || s1.UpdatedAt.IsZero() {
		t.Fatalf("unexpected s1: %+v", s1)
	}

	s2, err := repo.AdjustQuantity(ctx, "dt-1-v2", "dt-1", 5)
	if err != nil || s2.Quantity != 5 || s2.ProductID != "dt-1" {
		t.Fatalf("AdjustQuantity(dt-1-v2, +5): %v, %+v", err, s2)
	}

	s1After, err := repo.AdjustQuantity(ctx, "dt-1-v1", "dt-1", -3)
	if err != nil || s1After.Quantity != 7 {
		t.Fatalf("AdjustQuantity(dt-1-v1, -3): %v, %+v", err, s1After)
	}

	// Simulate 4 reserved units on dt-1-v1
	if _, err := pool.Exec(ctx, "UPDATE stock SET reserved = 4 WHERE sku_id = 'dt-1-v1'"); err != nil {
		t.Fatalf("set reserved: %v", err)
	}

	if _, err := repo.SetQuantity(ctx, "dt-1-v1", "dt-1", 3); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on SetQuantity(3 < 4), got %v", err)
	}
	if _, err := repo.AdjustQuantity(ctx, "dt-1-v1", "dt-1", -4); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on AdjustQuantity(7 - 4 < 4), got %v", err)
	}
	if _, err := repo.AdjustQuantity(ctx, "sku-missing", "dt-1", -1); !errors.Is(err, domain.ErrStockBelowReserved) {
		t.Fatalf("expected ErrStockBelowReserved on missing SKU negative adjust, got %v", err)
	}

	batch, err := repo.BatchGet(ctx, []string{"missing-sku", "dt-1-v1", "dt-1-v2"})
	if err != nil || len(batch) != 3 {
		t.Fatalf("BatchGet: %v, %+v", err, batch)
	}
	if batch[0].SkuID != "missing-sku" || batch[0].Quantity != 0 ||
		batch[1].SkuID != "dt-1-v1" || batch[1].Quantity != 7 || batch[1].Reserved != 4 ||
		batch[2].SkuID != "dt-1-v2" || batch[2].Quantity != 5 {
		t.Fatalf("unexpected BatchGet order/values: %+v", batch)
	}

	all, err := repo.List(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("List: %v, len=%d", err, len(all))
	}
}

func TestDeleteUnreservedDryRunAndReservedRejection(t *testing.T) {
	pool := isolatedInventoryPool(t, "inventory_delete_unreserved_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	if _, err := repo.SetQuantity(ctx, "sku-free", "prod-1", 8); err != nil {
		t.Fatalf("SetQuantity(sku-free): %v", err)
	}
	if _, err := repo.SetQuantity(ctx, "sku-reserved", "prod-1", 6); err != nil {
		t.Fatalf("SetQuantity(sku-reserved): %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE stock SET reserved = 2 WHERE sku_id = 'sku-reserved'"); err != nil {
		t.Fatalf("set reserved: %v", err)
	}

	// Dry-run on unreserved SKU succeeds and leaves row intact
	if err := repo.DeleteUnreserved(ctx, []string{"sku-free"}, true); err != nil {
		t.Fatalf("DeleteUnreserved(dryRun=true) on free SKU: %v", err)
	}
	stillThere, err := repo.Get(ctx, "sku-free")
	if err != nil || stillThere.Quantity != 8 {
		t.Fatalf("expected sku-free intact after dryRun, got %+v (err=%v)", stillThere, err)
	}

	// Reserved SKU fails on both dryRun=true and dryRun=false without deleting free sibling
	for _, dryRun := range []bool{true, false} {
		err := repo.DeleteUnreserved(ctx, []string{"sku-free", "sku-reserved"}, dryRun)
		var resErr *domain.VariantHasReservedStockError
		if !errors.As(err, &resErr) || resErr.SkuID != "sku-reserved" {
			t.Fatalf("DeleteUnreserved(dryRun=%v): expected VariantHasReservedStockError{sku-reserved}, got %v", dryRun, err)
		}
	}
	freeAfterFail, _ := repo.Get(ctx, "sku-free")
	if freeAfterFail.Quantity != 8 {
		t.Fatalf("atomic rollback failed; sku-free mutated to %+v", freeAfterFail)
	}

	// Commit delete on unreserved SKU deletes the row
	if err := repo.DeleteUnreserved(ctx, []string{"sku-free", "sku-nonexistent"}, false); err != nil {
		t.Fatalf("DeleteUnreserved(dryRun=false) on free SKU: %v", err)
	}
	all, err := repo.List(ctx)
	if err != nil || len(all) != 1 || all[0].SkuID != "sku-reserved" {
		t.Fatalf("expected only sku-reserved remaining, got %+v (err=%v)", all, err)
	}
}
