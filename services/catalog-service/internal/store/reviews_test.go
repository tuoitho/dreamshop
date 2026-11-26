package store

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
)

func TestReviewsStoreIntegration(t *testing.T) {
	pool := isolatedStorePool(t, "catalog_reviews_integration_test")
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := New(pool)
	const productID = "dt-1"
	const userA = "user-review-a"
	const userB = "user-review-b"
	const orderA = "ord-review-a"
	const orderB = "ord-review-b"

	// 1. Unpurchased user cannot review.
	if _, _, err := s.CreateReview(ctx, domain.CreateReviewInput{
		UserID:    userA,
		ProductID: productID,
		OrderID:   orderA,
		Rating:    5,
		Comment:   "Not bought yet",
	}, 24*time.Hour); !errors.Is(err, domain.ErrReviewNotEligible) {
		t.Fatalf("expected ErrReviewNotEligible, got %v", err)
	}

	// 2. RecordDeliveredOrder is idempotent and unlocks eligibility.
	deliveredAt := time.Now().UTC().Truncate(time.Second)
	items := []domain.DeliveredOrderItem{{ProductID: productID, SkuID: "dt-1-v1", VariantLabel: "256GB"}}
	if err := s.RecordDeliveredOrder(ctx, "evt-del-a", orderA, "ORD-A", userA, "Customer A", deliveredAt, items); err != nil {
		t.Fatalf("record delivered order A: %v", err)
	}
	if err := s.RecordDeliveredOrder(ctx, "evt-del-a", orderA, "ORD-A", userA, "Customer A", deliveredAt, items); err != nil {
		t.Fatalf("duplicate record delivered order A: %v", err)
	}
	if err := s.RecordDeliveredOrder(ctx, "evt-del-b", orderB, "ORD-B", userB, "Customer B", deliveredAt, items); err != nil {
		t.Fatalf("record delivered order B: %v", err)
	}

	elig, err := s.GetReviewEligibility(ctx, userA, productID)
	if err != nil {
		t.Fatalf("get eligibility: %v", err)
	}
	if !elig.CanReview || len(elig.EligibleOrders) != 1 || elig.EligibleOrders[0].Reviewed {
		t.Fatalf("expected 1 unreviewed eligible order, got %+v", elig)
	}

	// 3. Create pending review media asset and attach it on review creation.
	uploadID, err := domain.NewUUIDv4()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	imgURL := "http://localhost:9000/product-media/reviews/2026/11/" + uploadID + ".png"
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := s.CreateMediaAsset(ctx, &domain.MediaAsset{
		ID:          uploadID,
		Bucket:      "product-media",
		ObjectKey:   "reviews/2026/11/" + uploadID + ".png",
		PublicURL:   imgURL,
		MediaType:   domain.MediaTypeImage,
		ContentType: "image/png",
		SizeBytes:   1024,
		Status:      domain.MediaStatusPending,
		UploadedBy:  userA,
		ExpiresAt:   &expiresAt,
	}); err != nil {
		t.Fatalf("create media asset: %v", err)
	}

	revA, sumA, err := s.CreateReview(ctx, domain.CreateReviewInput{
		UserID:    userA,
		ProductID: productID,
		OrderID:   orderA,
		Rating:    5,
		Comment:   "Rat hai long",
		Images:    []string{imgURL},
	}, 24*time.Hour)
	if err != nil {
		t.Fatalf("create review A: %v", err)
	}
	if revA.AuthorName != "Customer A" || sumA.TotalReviews != 1 || math.Abs(sumA.AverageRating-5.0) > 0.001 {
		t.Fatalf("unexpected review A / summary: rev=%+v sum=%+v", revA, sumA)
	}

	// Duplicate review on same (user, order, product) is rejected.
	if _, _, err := s.CreateReview(ctx, domain.CreateReviewInput{
		UserID:    userA,
		ProductID: productID,
		OrderID:   orderA,
		Rating:    4,
	}, 24*time.Hour); !errors.Is(err, domain.ErrReviewAlreadyExists) {
		t.Fatalf("expected ErrReviewAlreadyExists, got %v", err)
	}

	// Second user adds a 3-star review -> product average becomes 4.0, count 2.
	revB, sumB, err := s.CreateReview(ctx, domain.CreateReviewInput{
		UserID:     userB,
		ProductID:  productID,
		OrderID:    orderB,
		AuthorName: "Custom Name B",
		Rating:     3,
		Comment:    "Tam on",
	}, 24*time.Hour)
	if err != nil {
		t.Fatalf("create review B: %v", err)
	}
	if sumB.TotalReviews != 2 || math.Abs(sumB.AverageRating-4.0) > 0.001 {
		t.Fatalf("expected avg 4.0 count 2, got %+v", sumB)
	}

	// Product row itself is updated atomically.
	prod, err := s.GetByID(ctx, productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if prod.ReviewCount != 2 || math.Abs(prod.Rating-4.0) > 0.001 {
		t.Fatalf("expected products.rating=4.0 review_count=2, got rating=%v count=%d", prod.Rating, prod.ReviewCount)
	}

	// 4. Hiding review B excludes it from public rating -> back to 5.0, count 1.
	hiddenB, err := s.SetReviewVisibility(ctx, revB.ID, "admin-1", true, "Noi dung khong phu hop")
	if err != nil || !hiddenB.Hidden {
		t.Fatalf("set hidden failed: rev=%+v err=%v", hiddenB, err)
	}
	prodAfterHide, err := s.GetByID(ctx, productID)
	if err != nil {
		t.Fatalf("get product after hide: %v", err)
	}
	if prodAfterHide.ReviewCount != 1 || math.Abs(prodAfterHide.Rating-5.0) > 0.001 {
		t.Fatalf("expected rating=5.0 count=1 after hide, got rating=%v count=%d", prodAfterHide.Rating, prodAfterHide.ReviewCount)
	}

	// 5. Admin reply + ListAdminReviews filters.
	repliedA, err := s.ReplyProductReview(ctx, revA.ID, "admin-1", "Cam on quy khach!")
	if err != nil || repliedA.AdminReply != "Cam on quy khach!" || repliedA.AdminRepliedAt == nil {
		t.Fatalf("reply failed: rev=%+v err=%v", repliedA, err)
	}
	adminRes, err := s.ListAdminReviews(ctx, domain.AdminReviewFilter{Status: domain.AdminReviewStatusHidden})
	if err != nil || adminRes.TotalItems != 1 || adminRes.VisibleCount != 1 || adminRes.HiddenCount != 1 {
		t.Fatalf("unexpected admin list: %+v err=%v", adminRes, err)
	}

	// 6. Another user cannot update or delete review A.
	if _, _, err := s.UpdateReview(ctx, domain.UpdateReviewInput{UserID: userB, ReviewID: revA.ID, Rating: 1}, 24*time.Hour); !errors.Is(err, domain.ErrReviewForbidden) {
		t.Fatalf("expected ErrReviewForbidden on update, got %v", err)
	}

	// 7. Deleting review A orphans its attached media asset and resets visible product rating to 0/0.
	_, sumAfterDel, err := s.DeleteReview(ctx, userA, revA.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("delete review A: %v", err)
	}
	if sumAfterDel.TotalReviews != 0 || sumAfterDel.AverageRating != 0 {
		t.Fatalf("expected 0 reviews after deleting visible review, got %+v", sumAfterDel)
	}
	var assetStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM product_media_assets WHERE id = $1::uuid`, uploadID).Scan(&assetStatus); err != nil {
		t.Fatalf("query media status: %v", err)
	}
	if assetStatus != domain.MediaStatusOrphaned {
		t.Fatalf("expected media asset status ORPHANED after review delete, got %q", assetStatus)
	}
}
