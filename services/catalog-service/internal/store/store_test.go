package store

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"testing"
	"time"

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

func TestSeedConcurrentOnEmptyCatalog(t *testing.T) {
	adminPool := testPool(t)
	ctx := context.Background()

	const schemaName = "catalog_seed_concurrent_test"
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

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate isolated schema: %v", err)
	}

	const replicas = 8
	start := make(chan struct{})
	errs := make(chan error, replicas)
	var wg sync.WaitGroup
	wg.Add(replicas)
	for range replicas {
		go func() {
			defer wg.Done()
			<-start
			errs <- Seed(ctx, pool)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Seed() failed: %v", err)
		}
	}

	var count int64
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&count); err != nil {
		t.Fatalf("count seeded products: %v", err)
	}
	if count != 16 {
		t.Fatalf("expected 16 seeded products after concurrent Seed(), got %d", count)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM products WHERE id <> 'dt-1'"); err != nil {
		t.Fatalf("delete subset of products: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("re-seed non-empty catalog: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&count); err != nil {
		t.Fatalf("re-count products: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected non-empty catalog to remain untouched at 1 product, got %d", count)
	}
}

func isolatedStorePool(t *testing.T, schemaName string) *pgxpool.Pool {
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

func TestAdminCatalogAndMediaLifecycle(t *testing.T) {
	pool := isolatedStorePool(t, "catalog_admin_media_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	now := time.Now().UTC()
	assetImg1 := &domain.MediaAsset{
		ID:               "11111111-1111-1111-1111-111111111111",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/img1.webp",
		PublicURL:        "/product-media/products/2026/10/img1.webp",
		MediaType:        domain.MediaTypeImage,
		OriginalFilename: "img1.webp",
		ContentType:      "image/webp",
		SizeBytes:        12000,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &now,
	}
	future := now.Add(24 * time.Hour)
	assetImg1.ExpiresAt = &future

	assetImg2 := &domain.MediaAsset{
		ID:               "22222222-2222-2222-2222-222222222222",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/img2.webp",
		PublicURL:        "/product-media/products/2026/10/img2.webp",
		MediaType:        domain.MediaTypeImage,
		OriginalFilename: "img2.webp",
		ContentType:      "image/webp",
		SizeBytes:        14000,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &future,
	}

	assetVid1 := &domain.MediaAsset{
		ID:               "33333333-3333-3333-3333-333333333333",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/vid1.mp4",
		PublicURL:        "/product-media/products/2026/10/vid1.mp4",
		MediaType:        domain.MediaTypeVideo,
		OriginalFilename: "vid1.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        480000,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &future,
	}

	past := now.Add(-1 * time.Hour)
	abandonedAsset := &domain.MediaAsset{
		ID:               "44444444-4444-4444-4444-444444444444",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/abandoned.png",
		PublicURL:        "/product-media/products/2026/10/abandoned.png",
		MediaType:        domain.MediaTypeImage,
		OriginalFilename: "abandoned.png",
		ContentType:      "image/png",
		SizeBytes:        8192,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &past,
	}

	for _, asset := range []*domain.MediaAsset{assetImg1, assetImg2, assetVid1, abandonedAsset} {
		if err := repo.CreateMediaAsset(ctx, asset); err != nil {
			t.Fatalf("CreateMediaAsset(%s): %v", asset.ID, err)
		}
	}

	created, err := repo.Create(ctx, &domain.Product{
		ID:                 "laptop-pro-m4",
		Name:               "Laptop Pro M4",
		ShortDescription:   "Mỏng nhẹ hiệu năng cao",
		Description:        "Chi tiết sản phẩm",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         32990000,
		OriginalPriceMinor: 35990000,
		Images:             []string{assetImg1.PublicURL, assetImg2.PublicURL},
		Videos:             []string{assetVid1.PublicURL},
		IsNew:              true,
		Badge:              "new",
		Featured:           true,
		Emoji:              "💻",
		ImageHue:           []uint32{210, 250},
		Variants:           []domain.ProductVariant{{Label: "16GB/512GB", ColorHex: "#1e293b"}},
		Details:            []string{"CPU: M4 10 nhân"},
	}, 24*time.Hour)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(created.Images) != 2 || len(created.Videos) != 1 || created.Videos[0] != assetVid1.PublicURL {
		t.Fatalf("unexpected created media: images=%v videos=%v", created.Images, created.Videos)
	}

	// Duplicate ID must return domain.ErrProductExists.
	_, err = repo.Create(ctx, &domain.Product{
		ID:                 "laptop-pro-m4",
		Name:               "Duplicate",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         10000,
		OriginalPriceMinor: 10000,
		Variants:           []domain.ProductVariant{{Label: "Default"}},
	}, 24*time.Hour)
	if !errors.Is(err, domain.ErrProductExists) {
		t.Fatalf("expected ErrProductExists on duplicate ID, got %v", err)
	}

	// Media attached to laptop-pro-m4 cannot be claimed by another product.
	_, err = repo.Create(ctx, &domain.Product{
		ID:                 "other-laptop",
		Name:               "Other Laptop",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         15000000,
		OriginalPriceMinor: 15000000,
		Images:             []string{assetImg1.PublicURL},
		Variants:           []domain.ProductVariant{{Label: "Default"}},
	}, 24*time.Hour)
	if !errors.Is(err, domain.ErrMediaAlreadyAttached) {
		t.Fatalf("expected ErrMediaAlreadyAttached, got %v", err)
	}

	// Update laptop-pro-m4 removing img2 and vid1 -> both become ORPHANED with non-null expires_at.
	updated, err := repo.Update(ctx, &domain.Product{
		ID:                 "laptop-pro-m4",
		Name:               "Laptop Pro M4 Max",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         31990000,
		OriginalPriceMinor: 35990000,
		Images:             []string{assetImg1.PublicURL},
		Videos:             nil,
		Variants:           []domain.ProductVariant{{Label: "32GB/1TB", ColorHex: "#0f172a"}},
		Details:            []string{"CPU: M4 Max"},
	}, 24*time.Hour)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 1 || len(updated.Videos) != 0 {
		t.Fatalf("unexpected updated media: images=%v videos=%v", updated.Images, updated.Videos)
	}

	var statusImg2, statusVid1 string
	var expiresImg2 *time.Time
	if err := pool.QueryRow(ctx, "SELECT status, expires_at FROM product_media_assets WHERE id = $1", assetImg2.ID).Scan(&statusImg2, &expiresImg2); err != nil {
		t.Fatalf("query assetImg2: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT status FROM product_media_assets WHERE id = $1", assetVid1.ID).Scan(&statusVid1); err != nil {
		t.Fatalf("query assetVid1: %v", err)
	}
	if statusImg2 != domain.MediaStatusOrphaned || expiresImg2 == nil || statusVid1 != domain.MediaStatusOrphaned {
		t.Fatalf("expected removed assets ORPHANED with expires_at, got img2=%s(%v) vid1=%s", statusImg2, expiresImg2, statusVid1)
	}

	// Re-attaching vid1 within grace window restores ATTACHED and clears expires_at.
	reattached, err := repo.Update(ctx, &domain.Product{
		ID:                 "laptop-pro-m4",
		Name:               "Laptop Pro M4 Max",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         31990000,
		OriginalPriceMinor: 35990000,
		Images:             []string{assetImg1.PublicURL},
		Videos:             []string{assetVid1.PublicURL},
		Variants:           []domain.ProductVariant{{Label: "32GB/1TB", ColorHex: "#0f172a"}},
	}, 24*time.Hour)
	if err != nil || len(reattached.Videos) != 1 {
		t.Fatalf("Update(reattach vid1): %v, videos=%v", err, reattached.Videos)
	}
	var expiresVid1 *time.Time
	if err := pool.QueryRow(ctx, "SELECT status, expires_at FROM product_media_assets WHERE id = $1", assetVid1.ID).Scan(&statusVid1, &expiresVid1); err != nil {
		t.Fatalf("query reattached vid1: %v", err)
	}
	if statusVid1 != domain.MediaStatusAttached || expiresVid1 != nil {
		t.Fatalf("expected reattached vid1 ATTACHED with nil expires_at, got %s (%v)", statusVid1, expiresVid1)
	}

	// SoftDelete must preserve ATTACHED status on media so restore never loses files.
	if err := repo.SoftDelete(ctx, "laptop-pro-m4", true); err != nil {
		t.Fatalf("SoftDelete(true): %v", err)
	}
	var statusImg1 string
	if err := pool.QueryRow(ctx, "SELECT status FROM product_media_assets WHERE id = $1", assetImg1.ID).Scan(&statusImg1); err != nil {
		t.Fatalf("query assetImg1 after soft delete: %v", err)
	}
	if statusImg1 != domain.MediaStatusAttached {
		t.Fatalf("expected SoftDelete to preserve ATTACHED media, got %s", statusImg1)
	}

	adminRes, err := repo.ListAdmin(ctx, domain.AdminSearchFilter{Status: "all", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListAdmin: %v", err)
	}
	if adminRes.DeletedCount != 1 || adminRes.ActiveCount != 0 || len(adminRes.Items) != 1 || adminRes.Items[0].VideoCount != 1 {
		t.Fatalf("unexpected ListAdmin result: %+v, item=%+v", adminRes, adminRes.Items[0])
	}

	invItems, err := repo.ListInventoryItems(ctx, domain.InventoryCatalogFilter{IncludeDeleted: true})
	if err != nil || len(invItems) != 1 || !invItems[0].Deleted || invItems[0].ImageURL != assetImg1.PublicURL {
		t.Fatalf("unexpected ListInventoryItems: err=%v items=%+v", err, invItems)
	}

	// SweepExpiredMedia removes abandonedAsset (expired) and leaves future-expiring assetImg2 untouched.
	var removedKeys []string
	swept, err := repo.SweepExpiredMedia(ctx, 50, func(_ context.Context, key string) error {
		removedKeys = append(removedKeys, key)
		return nil
	})
	if err != nil {
		t.Fatalf("SweepExpiredMedia: %v", err)
	}
	if swept != 1 || len(removedKeys) != 1 || removedKeys[0] != abandonedAsset.ObjectKey {
		t.Fatalf("expected 1 swept abandoned asset, got swept=%d keys=%v", swept, removedKeys)
	}
}

func TestSweepLeaseBlocksConcurrentAttachAndLocksMediaOwnership(t *testing.T) {
	pool := isolatedStorePool(t, "catalog_media_concurrency_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(pool)

	now := time.Now().UTC()
	past := now.Add(-2 * time.Minute)
	sweepingAsset := &domain.MediaAsset{
		ID:               "55555555-5555-5555-5555-555555555555",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/sweeping.webp",
		PublicURL:        "/product-media/products/2026/10/sweeping.webp",
		MediaType:        domain.MediaTypeImage,
		OriginalFilename: "sweeping.webp",
		ContentType:      "image/webp",
		SizeBytes:        4096,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &past,
	}
	if err := repo.CreateMediaAsset(ctx, sweepingAsset); err != nil {
		t.Fatalf("CreateMediaAsset(sweeping): %v", err)
	}

	enteredRemove := make(chan struct{})
	releaseRemove := make(chan struct{})
	sweepDone := make(chan error, 1)
	var sweptCount int

	go func() {
		var err error
		sweptCount, err = repo.SweepExpiredMedia(ctx, 10, func(_ context.Context, _ string) error {
			close(enteredRemove)
			<-releaseRemove
			return nil
		})
		sweepDone <- err
	}()

	<-enteredRemove

	// While removeObject is in-flight after SweepExpiredMedia committed its lease,
	// a concurrent Create referencing sweepingAsset MUST be rejected rather than re-attaching a deleting S3 object.
	_, attachErr := repo.Create(ctx, &domain.Product{
		ID:                 "late-attach-product",
		Name:               "Late Attach Product",
		Brand:              "Apex",
		CategoryID:         "laptop",
		CategoryName:       "Laptop",
		PriceMinor:         10000000,
		OriginalPriceMinor: 10000000,
		Images:             []string{sweepingAsset.PublicURL},
		Variants:           []domain.ProductVariant{{Label: "Tiêu chuẩn"}},
	}, 24*time.Hour)
	if !errors.Is(attachErr, domain.ErrMediaExpired) {
		close(releaseRemove)
		<-sweepDone
		t.Fatalf("expected ErrMediaExpired when attaching in-flight sweeping asset, got %v", attachErr)
	}

	close(releaseRemove)
	if err := <-sweepDone; err != nil {
		t.Fatalf("SweepExpiredMedia: %v", err)
	}
	if sweptCount != 1 {
		t.Fatalf("sweptCount = %d, want 1", sweptCount)
	}

	// Concurrent Create calls referencing the same unexpired PENDING asset must serialize via FOR UPDATE
	// so exactly one succeeds and the second fails with ErrMediaAlreadyAttached.
	future := now.Add(24 * time.Hour)
	sharedPending := &domain.MediaAsset{
		ID:               "66666666-6666-6666-6666-666666666666",
		Bucket:           "product-media",
		ObjectKey:        "products/2026/10/shared.webp",
		PublicURL:        "/product-media/products/2026/10/shared.webp",
		MediaType:        domain.MediaTypeImage,
		OriginalFilename: "shared.webp",
		ContentType:      "image/webp",
		SizeBytes:        4096,
		Status:           domain.MediaStatusPending,
		UploadedBy:       "admin-1",
		ExpiresAt:        &future,
	}
	if err := repo.CreateMediaAsset(ctx, sharedPending); err != nil {
		t.Fatalf("CreateMediaAsset(shared): %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for idx, pid := range []string{"concurrent-prod-a", "concurrent-prod-b"} {
		wg.Add(1)
		go func(i int, productID string) {
			defer wg.Done()
			_, errs[i] = repo.Create(ctx, &domain.Product{
				ID:                 productID,
				Name:               "Concurrent " + productID,
				Brand:              "Apex",
				CategoryID:         "laptop",
				CategoryName:       "Laptop",
				PriceMinor:         12000000,
				OriginalPriceMinor: 12000000,
				Images:             []string{sharedPending.PublicURL},
				Variants:           []domain.ProductVariant{{Label: "Tiêu chuẩn"}},
			}, 24*time.Hour)
		}(idx, pid)
	}
	wg.Wait()

	var okCount, conflictCount int
	for _, err := range errs {
		switch {
		case err == nil:
			okCount++
		case errors.Is(err, domain.ErrMediaAlreadyAttached):
			conflictCount++
		default:
			t.Fatalf("unexpected concurrent Create error: %v", err)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("expected 1 success and 1 ErrMediaAlreadyAttached, got ok=%d conflict=%d (errs=%v)", okCount, conflictCount, errs)
	}
}
