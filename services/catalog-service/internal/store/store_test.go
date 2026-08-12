package store

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"ecommerce.local/catalog-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the DATABASE_URL integration database or skips.
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

// TestStoreIntegration exercises migrations, the idempotent seed, and the
// repository against a real PostgreSQL. It is skipped unless DATABASE_URL is
// set; point it at a dedicated catalog database because it runs migrations
// and seeds demo data.
func TestStoreIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Second run must be a no-op (idempotent migrations + seed).
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	repo := New(pool)

	t.Run("GetByID returns seeded product with children", func(t *testing.T) {
		p, err := repo.GetByID(ctx, "dt-1")
		if err != nil {
			t.Fatalf("GetByID(dt-1): %v", err)
		}
		if p.Name != "Điện thoại thông minh Nova X15 Pro 5G (16GB/512GB)" {
			t.Fatalf("unexpected name: %q", p.Name)
		}
		if p.PriceMinor != 18990000 || p.OriginalPriceMinor != 21990000 {
			t.Fatalf("unexpected prices: %d/%d", p.PriceMinor, p.OriginalPriceMinor)
		}
		// rating is stored as real (float4); compare with tolerance.
		if math.Abs(p.Rating-4.9) > 0.001 || p.ReviewCount != 1284 || p.SoldCount != 2310 {
			t.Fatalf("unexpected metrics: %+v", p)
		}
		if len(p.Images) != 4 || p.Images[0] != "https://picsum.photos/seed/dt-1-1/800/800" {
			t.Fatalf("unexpected images: %v", p.Images)
		}
		if len(p.Variants) != 3 || p.Variants[0].Label != "12GB/256GB" {
			t.Fatalf("unexpected variants: %v", p.Variants)
		}
		if len(p.Details) != 5 || p.Details[0] != "Màn hình: 6.8\" AMOLED 144Hz, 3200x1440" {
			t.Fatalf("unexpected details: %v", p.Details)
		}
		if len(p.ImageHue) != 2 || p.ImageHue[0] != 220 || p.ImageHue[1] != 260 {
			t.Fatalf("unexpected imageHue: %v", p.ImageHue)
		}
	})

	t.Run("GetByID returns ErrProductNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "nope")
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
	})

	t.Run("Search filters, sorts, and paginates", func(t *testing.T) {
		result, err := repo.Search(ctx, domain.SearchFilter{CategoryID: "dien-thoai", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if result.TotalItems != 2 || len(result.Items) != 2 {
			t.Fatalf("expected 2 phones, got %+v", result)
		}
		// Default popularity: dt-2 (8920) before dt-1 (2310).
		if result.Items[0].ID != "dt-2" || result.Items[1].ID != "dt-1" {
			t.Fatalf("unexpected popularity order: %v", []string{result.Items[0].ID, result.Items[1].ID})
		}
		if len(result.Items[0].Images) != 4 {
			t.Fatalf("expected images on summary, got %v", result.Items[0].Images)
		}

		onSale, err := repo.Search(ctx, domain.SearchFilter{OnSale: true})
		if err != nil {
			t.Fatalf("Search on-sale: %v", err)
		}
		// dh-2 has originalPrice == price and is the only non-discounted seed.
		if onSale.TotalItems != 15 {
			t.Fatalf("expected 15 on-sale products, got %d", onSale.TotalItems)
		}
	})

	t.Run("SoftDelete hides product from search until restored", func(t *testing.T) {
		if err := repo.SoftDelete(ctx, "dt-1", true); err != nil {
			t.Fatalf("SoftDelete(dt-1, true): %v", err)
		}
		// Restore before the test ends so the shared catalog keeps dt-1 active
		// for any later runs against the same database.
		t.Cleanup(func() {
			if err := repo.SoftDelete(ctx, "dt-1", false); err != nil {
				t.Errorf("restore SoftDelete(dt-1, false): %v", err)
			}
		})

		p, err := repo.GetByID(ctx, "dt-1")
		if err != nil {
			t.Fatalf("GetByID after soft delete: %v", err)
		}
		if !p.Deleted {
			t.Fatalf("expected Deleted=true for soft-deleted product")
		}

		result, err := repo.Search(ctx, domain.SearchFilter{CategoryID: "dien-thoai", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("Search after soft delete: %v", err)
		}
		for _, item := range result.Items {
			if item.ID == "dt-1" {
				t.Fatalf("Search still returned soft-deleted product dt-1")
			}
		}
	})
}

// TestSeedDataCount guards the demo-data mirror invariant: exactly 16 products.
func TestSeedDataCount(t *testing.T) {
	if got := len(seedProducts()); got != 16 {
		t.Fatalf("expected 16 seed products, got %d", got)
	}
}
