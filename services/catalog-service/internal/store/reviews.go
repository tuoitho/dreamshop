package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ecommerce.local/catalog-service/internal/domain"
)

const pgInvalidTextRepresentation = "22P02"

// RecordDeliveredOrder idempotently records delivered order items as eligible for customer product reviews.
func (s *Store) RecordDeliveredOrder(
	ctx context.Context,
	eventID, orderID, orderCode, userID, customerName string,
	deliveredAt time.Time,
	items []domain.DeliveredOrderItem,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin record delivered order tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, processed_at)
		VALUES ($1, now())
		ON CONFLICT (event_id) DO NOTHING
	`, eventID)
	if err != nil {
		return fmt.Errorf("insert processed event %q: %w", eventID, err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	if deliveredAt.IsZero() {
		deliveredAt = time.Now().UTC()
	}
	cleanOrderCode := strings.TrimSpace(orderCode)
	cleanCustomerName := strings.TrimSpace(customerName)

	for _, item := range items {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO review_eligibilities (
				order_id, order_code, user_id, product_id, sku_id, variant_label, customer_name, delivered_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (order_id, product_id) DO NOTHING
		`,
			orderID,
			cleanOrderCode,
			userID,
			productID,
			strings.TrimSpace(item.SkuID),
			strings.TrimSpace(item.VariantLabel),
			cleanCustomerName,
			deliveredAt.UTC(),
		); err != nil {
			return fmt.Errorf("insert review eligibility order=%q product=%q: %w", orderID, productID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit record delivered order tx: %w", err)
	}
	return nil
}

// ListProductReviews returns paginated visible reviews and the live rating summary for one product.
func (s *Store) ListProductReviews(ctx context.Context, filter domain.ReviewListFilter) (*domain.ReviewListResult, error) {
	filter = filter.Normalize()

	summary, err := s.queryRatingSummary(ctx, s.pool, filter.ProductID)
	if err != nil {
		return nil, err
	}

	clauses := []string{"r.product_id = $1", "r.hidden = false"}
	args := []any{filter.ProductID}
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.Rating > 0 {
		clauses = append(clauses, "r.rating = "+next(filter.Rating))
	}
	if filter.HasImages {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM product_review_images pri WHERE pri.review_id = r.id)")
	}
	where := strings.Join(clauses, " AND ")

	var totalItems int64
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_reviews r WHERE "+where, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count product reviews: %w", err)
	}

	orderBy := "r.created_at DESC, r.id DESC"
	switch filter.Sort {
	case domain.ReviewSortHighest:
		orderBy = "r.rating DESC, r.created_at DESC, r.id DESC"
	case domain.ReviewSortLowest:
		orderBy = "r.rating ASC, r.created_at DESC, r.id DESC"
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))

	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text, r.product_id, COALESCE(p.name, ''), r.user_id, r.order_id,
		       r.sku_id, r.variant_label, r.author_name, r.rating, r.comment,
		       r.verified_purchase, r.hidden, r.hidden_reason, r.admin_reply,
		       r.admin_replied_at, r.created_at, r.updated_at
		FROM product_reviews r
		LEFT JOIN products p ON p.id = r.product_id
		WHERE `+where+`
		ORDER BY `+orderBy+`
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder, args...)
	if err != nil {
		return nil, fmt.Errorf("list product reviews: %w", err)
	}
	defer rows.Close()

	items, err := scanReviews(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachReviewImages(ctx, s.pool, items); err != nil {
		return nil, err
	}

	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	return &domain.ReviewListResult{
		Items:      items,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
		Summary:    *summary,
	}, nil
}

// GetReviewEligibility returns delivered orders for (userID, productID) and any reviews the user already wrote.
func (s *Store) GetReviewEligibility(ctx context.Context, userID, productID string) (*domain.ReviewEligibility, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.order_id, e.order_code, e.sku_id, e.variant_label, e.delivered_at,
		       COALESCE(r.id::text, '') AS review_id
		FROM review_eligibilities e
		LEFT JOIN product_reviews r
		  ON r.user_id = e.user_id AND r.order_id = e.order_id AND r.product_id = e.product_id
		WHERE e.user_id = $1 AND e.product_id = $2
		ORDER BY e.delivered_at DESC, e.order_id ASC
	`, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("query review eligibility: %w", err)
	}
	defer rows.Close()

	var eligible []domain.EligibleReviewOrder
	canReview := false
	for rows.Next() {
		var row domain.EligibleReviewOrder
		if err := rows.Scan(
			&row.OrderID, &row.OrderCode, &row.SkuID, &row.VariantLabel, &row.DeliveredAt, &row.ReviewID,
		); err != nil {
			return nil, fmt.Errorf("scan review eligibility: %w", err)
		}
		row.Reviewed = row.ReviewID != ""
		if !row.Reviewed {
			canReview = true
		}
		eligible = append(eligible, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	myRows, err := s.pool.Query(ctx, `
		SELECT r.id::text, r.product_id, COALESCE(p.name, ''), r.user_id, r.order_id,
		       r.sku_id, r.variant_label, r.author_name, r.rating, r.comment,
		       r.verified_purchase, r.hidden, r.hidden_reason, r.admin_reply,
		       r.admin_replied_at, r.created_at, r.updated_at
		FROM product_reviews r
		LEFT JOIN products p ON p.id = r.product_id
		WHERE r.user_id = $1 AND r.product_id = $2
		ORDER BY r.created_at DESC
	`, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("query user product reviews: %w", err)
	}
	defer myRows.Close()

	myReviews, err := scanReviews(myRows)
	if err != nil {
		return nil, err
	}
	if err := s.attachReviewImages(ctx, s.pool, myReviews); err != nil {
		return nil, err
	}

	return &domain.ReviewEligibility{
		CanReview:      canReview,
		EligibleOrders: eligible,
		MyReviews:      myReviews,
	}, nil
}

// CreateReview verifies delivery eligibility, inserts a review + images, reconciles media assets,
// and atomically updates products.rating and products.review_count.
func (s *Store) CreateReview(ctx context.Context, in domain.CreateReviewInput, orphanGrace time.Duration) (*domain.ProductReview, *domain.ReviewRatingSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin create review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productName string
	if err := tx.QueryRow(ctx, `SELECT name FROM products WHERE id = $1 AND deleted_at IS NULL`, in.ProductID).Scan(&productName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrProductNotFound
		}
		return nil, nil, fmt.Errorf("lookup product %q for review: %w", in.ProductID, err)
	}

	var skuID, variantLabel, customerName string
	if err := tx.QueryRow(ctx, `
		SELECT sku_id, variant_label, customer_name
		FROM review_eligibilities
		WHERE order_id = $1 AND product_id = $2 AND user_id = $3
	`, in.OrderID, in.ProductID, in.UserID).Scan(&skuID, &variantLabel, &customerName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrReviewNotEligible
		}
		return nil, nil, fmt.Errorf("lookup review eligibility: %w", err)
	}

	authorName := strings.TrimSpace(in.AuthorName)
	if authorName == "" {
		authorName = strings.TrimSpace(customerName)
	}
	if authorName == "" {
		authorName = "Khách hàng"
	}

	reviewID, err := domain.NewUUIDv4()
	if err != nil {
		return nil, nil, err
	}

	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO product_reviews (
			id, product_id, user_id, order_id, sku_id, variant_label,
			author_name, rating, comment, verified_purchase, created_at, updated_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, true, now(), now())
		RETURNING created_at, updated_at
	`, reviewID, in.ProductID, in.UserID, in.OrderID, skuID, variantLabel, authorName, in.Rating, in.Comment).Scan(&createdAt, &updatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, nil, domain.ErrReviewAlreadyExists
		}
		return nil, nil, fmt.Errorf("insert product review: %w", err)
	}

	if err := replaceReviewImages(ctx, tx, reviewID, in.Images); err != nil {
		return nil, nil, err
	}
	if err := reconcileReviewMediaAssets(ctx, tx, reviewID, in.UserID, in.Images, orphanGrace, false); err != nil {
		return nil, nil, err
	}

	summary, err := recalculateProductRatingTx(ctx, tx, in.ProductID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit create review tx: %w", err)
	}

	return &domain.ProductReview{
		ID:               reviewID,
		ProductID:        in.ProductID,
		ProductName:      productName,
		UserID:           in.UserID,
		OrderID:          in.OrderID,
		SkuID:            skuID,
		VariantLabel:     variantLabel,
		AuthorName:       authorName,
		Rating:           in.Rating,
		Comment:          in.Comment,
		Images:           in.Images,
		VerifiedPurchase: true,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, summary, nil
}

// UpdateReview updates a customer's own review, reconciles attached images, and recalculates product rating.
func (s *Store) UpdateReview(ctx context.Context, in domain.UpdateReviewInput, orphanGrace time.Duration) (*domain.ProductReview, *domain.ReviewRatingSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin update review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	review, err := lockReviewByID(ctx, tx, in.ReviewID)
	if err != nil {
		return nil, nil, err
	}
	if review.UserID != in.UserID {
		return nil, nil, domain.ErrReviewForbidden
	}

	if err := tx.QueryRow(ctx, `
		UPDATE product_reviews
		SET rating = $2,
		    comment = $3,
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING updated_at
	`, in.ReviewID, in.Rating, in.Comment).Scan(&review.UpdatedAt); err != nil {
		return nil, nil, fmt.Errorf("update product review %q: %w", in.ReviewID, err)
	}
	review.Rating = in.Rating
	review.Comment = in.Comment
	review.Images = in.Images

	if err := replaceReviewImages(ctx, tx, in.ReviewID, in.Images); err != nil {
		return nil, nil, err
	}
	if err := reconcileReviewMediaAssets(ctx, tx, in.ReviewID, in.UserID, in.Images, orphanGrace, true); err != nil {
		return nil, nil, err
	}

	summary, err := recalculateProductRatingTx(ctx, tx, review.ProductID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit update review tx: %w", err)
	}
	return review, summary, nil
}

// DeleteReview deletes a customer's own review, orphans attached images, and recalculates product rating.
func (s *Store) DeleteReview(ctx context.Context, userID, reviewID string, orphanGrace time.Duration) (string, *domain.ReviewRatingSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("begin delete review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	review, err := lockReviewByID(ctx, tx, reviewID)
	if err != nil {
		return "", nil, err
	}
	if review.UserID != userID {
		return "", nil, domain.ErrReviewForbidden
	}

	if err := orphanAllReviewMedia(ctx, tx, reviewID, orphanGrace); err != nil {
		return "", nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM product_reviews WHERE id = $1::uuid`, reviewID); err != nil {
		return "", nil, fmt.Errorf("delete product review %q: %w", reviewID, err)
	}

	summary, err := recalculateProductRatingTx(ctx, tx, review.ProductID)
	if err != nil {
		return "", nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("commit delete review tx: %w", err)
	}
	return review.ProductID, summary, nil
}

// ListAdminReviews returns paginated reviews for moderation along with global KPIs.
func (s *Store) ListAdminReviews(ctx context.Context, filter domain.AdminReviewFilter) (*domain.AdminReviewListResult, error) {
	filter = filter.Normalize()

	var visibleCount, hiddenCount, unrepliedCount int64
	var averageRating float64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE hidden = false),
			COUNT(*) FILTER (WHERE hidden = true),
			COUNT(*) FILTER (WHERE hidden = false AND admin_reply = ''),
			COALESCE(ROUND(AVG(rating) FILTER (WHERE hidden = false)::numeric, 1), 0)::double precision
		FROM product_reviews
	`).Scan(&visibleCount, &hiddenCount, &unrepliedCount, &averageRating); err != nil {
		return nil, fmt.Errorf("count admin review KPIs: %w", err)
	}

	clauses := []string{"1=1"}
	var args []any
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.Query != "" {
		pat := next("%" + filter.Query + "%")
		clauses = append(clauses, fmt.Sprintf(
			"(r.comment ILIKE %s OR r.author_name ILIKE %s OR r.product_id ILIKE %s OR COALESCE(p.name, '') ILIKE %s OR r.order_id ILIKE %s)",
			pat, pat, pat, pat, pat,
		))
	}
	if filter.ProductID != "" {
		clauses = append(clauses, "r.product_id = "+next(filter.ProductID))
	}
	if filter.Rating > 0 {
		clauses = append(clauses, "r.rating = "+next(filter.Rating))
	}
	switch filter.Status {
	case domain.AdminReviewStatusVisible:
		clauses = append(clauses, "r.hidden = false")
	case domain.AdminReviewStatusHidden:
		clauses = append(clauses, "r.hidden = true")
	}
	switch filter.ReplyStatus {
	case domain.AdminReplyStatusReplied:
		clauses = append(clauses, "r.admin_reply <> ''")
	case domain.AdminReplyStatusUnreplied:
		clauses = append(clauses, "r.admin_reply = ''")
	}

	where := strings.Join(clauses, " AND ")

	var totalItems int64
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM product_reviews r
		LEFT JOIN products p ON p.id = r.product_id
		WHERE `+where, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count admin reviews: %w", err)
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))

	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text, r.product_id, COALESCE(p.name, ''), r.user_id, r.order_id,
		       r.sku_id, r.variant_label, r.author_name, r.rating, r.comment,
		       r.verified_purchase, r.hidden, r.hidden_reason, r.admin_reply,
		       r.admin_replied_at, r.created_at, r.updated_at
		FROM product_reviews r
		LEFT JOIN products p ON p.id = r.product_id
		WHERE `+where+`
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder, args...)
	if err != nil {
		return nil, fmt.Errorf("list admin reviews: %w", err)
	}
	defer rows.Close()

	items, err := scanReviews(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachReviewImages(ctx, s.pool, items); err != nil {
		return nil, err
	}

	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	return &domain.AdminReviewListResult{
		Items:          items,
		Page:           filter.Page,
		PageSize:       filter.PageSize,
		TotalItems:     totalItems,
		TotalPages:     totalPages,
		VisibleCount:   visibleCount,
		HiddenCount:    hiddenCount,
		UnrepliedCount: unrepliedCount,
		AverageRating:  averageRating,
	}, nil
}

// SetReviewVisibility hides or restores a review and recalculates product rating atomically.
func (s *Store) SetReviewVisibility(ctx context.Context, reviewID, adminID string, hidden bool, reason string) (*domain.ProductReview, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set review visibility tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	review, err := lockReviewByID(ctx, tx, reviewID)
	if err != nil {
		return nil, err
	}

	if hidden {
		err = tx.QueryRow(ctx, `
			UPDATE product_reviews
			SET hidden = true,
			    hidden_reason = $2,
			    hidden_by = $3,
			    hidden_at = now(),
			    updated_at = now()
			WHERE id = $1::uuid
			RETURNING updated_at
		`, reviewID, reason, adminID).Scan(&review.UpdatedAt)
		review.Hidden = true
		review.HiddenReason = reason
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE product_reviews
			SET hidden = false,
			    hidden_reason = '',
			    hidden_by = '',
			    hidden_at = NULL,
			    updated_at = now()
			WHERE id = $1::uuid
			RETURNING updated_at
		`, reviewID).Scan(&review.UpdatedAt)
		review.Hidden = false
		review.HiddenReason = ""
	}
	if err != nil {
		return nil, fmt.Errorf("update review visibility %q: %w", reviewID, err)
	}

	if _, err := recalculateProductRatingTx(ctx, tx, review.ProductID); err != nil {
		return nil, err
	}
	if err := s.attachReviewImages(ctx, tx, []*domain.ProductReview{review}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set review visibility tx: %w", err)
	}
	return review, nil
}

// ReplyProductReview sets or clears an official store reply on a review.
func (s *Store) ReplyProductReview(ctx context.Context, reviewID, adminID, reply string) (*domain.ProductReview, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reply review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	review, err := lockReviewByID(ctx, tx, reviewID)
	if err != nil {
		return nil, err
	}

	if reply == "" {
		err = tx.QueryRow(ctx, `
			UPDATE product_reviews
			SET admin_reply = '',
			    admin_reply_by = '',
			    admin_replied_at = NULL,
			    updated_at = now()
			WHERE id = $1::uuid
			RETURNING admin_replied_at, updated_at
		`, reviewID).Scan(&review.AdminRepliedAt, &review.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE product_reviews
			SET admin_reply = $2,
			    admin_reply_by = $3,
			    admin_replied_at = now(),
			    updated_at = now()
			WHERE id = $1::uuid
			RETURNING admin_replied_at, updated_at
		`, reviewID, reply, adminID).Scan(&review.AdminRepliedAt, &review.UpdatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("reply product review %q: %w", reviewID, err)
	}
	review.AdminReply = reply

	if err := s.attachReviewImages(ctx, tx, []*domain.ProductReview{review}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reply review tx: %w", err)
	}
	return review, nil
}

type queryRunner interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func lockReviewByID(ctx context.Context, tx pgx.Tx, reviewID string) (*domain.ProductReview, error) {
	r := &domain.ProductReview{}
	err := tx.QueryRow(ctx, `
		SELECT r.id::text, r.product_id, COALESCE(p.name, ''), r.user_id, r.order_id,
		       r.sku_id, r.variant_label, r.author_name, r.rating, r.comment,
		       r.verified_purchase, r.hidden, r.hidden_reason, r.admin_reply,
		       r.admin_replied_at, r.created_at, r.updated_at
		FROM product_reviews r
		LEFT JOIN products p ON p.id = r.product_id
		WHERE r.id = $1::uuid
		FOR UPDATE OF r
	`, reviewID).Scan(
		&r.ID, &r.ProductID, &r.ProductName, &r.UserID, &r.OrderID,
		&r.SkuID, &r.VariantLabel, &r.AuthorName, &r.Rating, &r.Comment,
		&r.VerifiedPurchase, &r.Hidden, &r.HiddenReason, &r.AdminReply,
		&r.AdminRepliedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReviewNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgInvalidTextRepresentation {
			return nil, domain.ErrReviewNotFound
		}
		return nil, fmt.Errorf("lock review %q: %w", reviewID, err)
	}
	return r, nil
}

func scanReviews(rows pgx.Rows) ([]*domain.ProductReview, error) {
	var items []*domain.ProductReview
	for rows.Next() {
		r := &domain.ProductReview{}
		if err := rows.Scan(
			&r.ID, &r.ProductID, &r.ProductName, &r.UserID, &r.OrderID,
			&r.SkuID, &r.VariantLabel, &r.AuthorName, &r.Rating, &r.Comment,
			&r.VerifiedPurchase, &r.Hidden, &r.HiddenReason, &r.AdminReply,
			&r.AdminRepliedAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product review: %w", err)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product reviews: %w", err)
	}
	return items, nil
}

func (s *Store) attachReviewImages(ctx context.Context, q queryRunner, items []*domain.ProductReview) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	rows, err := q.Query(ctx, `
		SELECT review_id::text, url
		FROM product_review_images
		WHERE review_id = ANY($1::uuid[])
		ORDER BY review_id, position ASC
	`, ids)
	if err != nil {
		return fmt.Errorf("load review images: %w", err)
	}
	defer rows.Close()

	byReview := make(map[string][]string, len(items))
	for rows.Next() {
		var rid, url string
		if err := rows.Scan(&rid, &url); err != nil {
			return fmt.Errorf("scan review image: %w", err)
		}
		byReview[rid] = append(byReview[rid], url)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		item.Images = byReview[item.ID]
	}
	return nil
}

func (s *Store) queryRatingSummary(ctx context.Context, q queryRunner, productID string) (*domain.ReviewRatingSummary, error) {
	summary := &domain.ReviewRatingSummary{
		Breakdown: domain.EmptyRatingBreakdown(),
	}
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(rating)::numeric, 1), 0)::double precision, COUNT(*)
		FROM product_reviews
		WHERE product_id = $1 AND hidden = false
	`, productID).Scan(&summary.AverageRating, &summary.TotalReviews); err != nil {
		return nil, fmt.Errorf("query rating summary for %q: %w", productID, err)
	}

	rows, err := q.Query(ctx, `
		SELECT rating, COUNT(*)
		FROM product_reviews
		WHERE product_id = $1 AND hidden = false
		GROUP BY rating
	`, productID)
	if err != nil {
		return nil, fmt.Errorf("query rating breakdown for %q: %w", productID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var star int32
		var count int64
		if err := rows.Scan(&star, &count); err != nil {
			return nil, fmt.Errorf("scan rating breakdown: %w", err)
		}
		if star >= 1 && star <= 5 {
			summary.Breakdown[5-star].Count = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summary, nil
}

func recalculateProductRatingTx(ctx context.Context, tx pgx.Tx, productID string) (*domain.ReviewRatingSummary, error) {
	summary := &domain.ReviewRatingSummary{
		Breakdown: domain.EmptyRatingBreakdown(),
	}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(rating)::numeric, 1), 0)::double precision, COUNT(*)
		FROM product_reviews
		WHERE product_id = $1 AND hidden = false
	`, productID).Scan(&summary.AverageRating, &summary.TotalReviews); err != nil {
		return nil, fmt.Errorf("recalculate rating for %q: %w", productID, err)
	}

	rows, err := tx.Query(ctx, `
		SELECT rating, COUNT(*)
		FROM product_reviews
		WHERE product_id = $1 AND hidden = false
		GROUP BY rating
	`, productID)
	if err != nil {
		return nil, fmt.Errorf("recalculate breakdown for %q: %w", productID, err)
	}
	for rows.Next() {
		var star int32
		var count int64
		if err := rows.Scan(&star, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		if star >= 1 && star <= 5 {
			summary.Breakdown[5-star].Count = count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if _, err := tx.Exec(ctx, `
		UPDATE products
		SET rating = $2,
		    review_count = $3,
		    updated_at = now()
		WHERE id = $1
	`, productID, summary.AverageRating, summary.TotalReviews); err != nil {
		return nil, fmt.Errorf("update product rating for %q: %w", productID, err)
	}
	return summary, nil
}

func replaceReviewImages(ctx context.Context, tx pgx.Tx, reviewID string, images []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM product_review_images WHERE review_id = $1::uuid`, reviewID); err != nil {
		return fmt.Errorf("clear review images: %w", err)
	}
	for i, url := range images {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_review_images (review_id, url, position)
			VALUES ($1::uuid, $2, $3)
		`, reviewID, url, i); err != nil {
			return fmt.Errorf("insert review image %d: %w", i, err)
		}
	}
	return nil
}

func reconcileReviewMediaAssets(ctx context.Context, tx pgx.Tx, reviewID, userID string, images []string, orphanGrace time.Duration, isUpdate bool) error {
	if orphanGrace <= 0 {
		orphanGrace = 24 * time.Hour
	}
	seen := make(map[string]struct{}, len(images))
	urls := make([]string, 0, len(images))
	for _, u := range images {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}

	if len(urls) > 0 {
		rows, err := tx.Query(ctx, `
			SELECT public_url,
			       status,
			       COALESCE(product_id, ''),
			       COALESCE(review_id::text, ''),
			       uploaded_by,
			       ((status IN ('PENDING', 'ORPHANED')) AND (
			           (expires_at IS NOT NULL AND expires_at <= now())
			           OR (sweep_locked_until IS NOT NULL AND sweep_locked_until > now())
			       )) AS is_expired
			FROM product_media_assets
			WHERE public_url = ANY($1)
			ORDER BY id
			FOR UPDATE
		`, urls)
		if err != nil {
			return fmt.Errorf("lock review media assets: %w", err)
		}
		for rows.Next() {
			var publicURL, rowStatus, rowProductID, rowReviewID, uploadedBy string
			var isExpired bool
			if err := rows.Scan(&publicURL, &rowStatus, &rowProductID, &rowReviewID, &uploadedBy, &isExpired); err != nil {
				rows.Close()
				return fmt.Errorf("scan locked review media asset: %w", err)
			}
			if rowStatus == domain.MediaStatusAttached && (rowProductID != "" || (rowReviewID != "" && rowReviewID != reviewID)) {
				rows.Close()
				return domain.ErrMediaAlreadyAttached
			}
			if rowStatus == domain.MediaStatusPending && uploadedBy != "" && uploadedBy != userID {
				rows.Close()
				return domain.ErrMediaAlreadyAttached
			}
			if isExpired {
				rows.Close()
				return domain.ErrMediaExpired
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate locked review media assets: %w", err)
		}
		rows.Close()
	}

	if isUpdate {
		graceSeconds := int64(orphanGrace.Seconds())
		if graceSeconds <= 0 {
			graceSeconds = int64((24 * time.Hour).Seconds())
		}
		if _, err := tx.Exec(ctx, `
			UPDATE product_media_assets
			SET status = 'ORPHANED',
			    review_id = NULL,
			    attached_at = NULL,
			    expires_at = now() + ($2 * interval '1 second'),
			    sweep_locked_until = NULL
			WHERE review_id = $1::uuid
			  AND status = 'ATTACHED'
			  AND NOT (public_url = ANY($3))
		`, reviewID, graceSeconds, urls); err != nil {
			return fmt.Errorf("mark removed review media orphaned: %w", err)
		}
	}

	if len(urls) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE product_media_assets
			SET status = 'ATTACHED',
			    review_id = $1::uuid,
			    attached_at = COALESCE(attached_at, now()),
			    expires_at = NULL,
			    sweep_locked_until = NULL
			WHERE public_url = ANY($2)
			  AND (
			      (status = 'ATTACHED' AND review_id = $1::uuid)
			      OR (status IN ('PENDING', 'ORPHANED')
			          AND (expires_at IS NULL OR expires_at > now())
			          AND (sweep_locked_until IS NULL OR sweep_locked_until <= now()))
			  )
		`, reviewID, urls); err != nil {
			return fmt.Errorf("attach review media assets: %w", err)
		}
	}
	return nil
}

func orphanAllReviewMedia(ctx context.Context, tx pgx.Tx, reviewID string, orphanGrace time.Duration) error {
	graceSeconds := int64(orphanGrace.Seconds())
	if graceSeconds <= 0 {
		graceSeconds = int64((24 * time.Hour).Seconds())
	}
	if _, err := tx.Exec(ctx, `
		UPDATE product_media_assets
		SET status = 'ORPHANED',
		    review_id = NULL,
		    attached_at = NULL,
		    expires_at = now() + ($2 * interval '1 second'),
		    sweep_locked_until = NULL
		WHERE review_id = $1::uuid
		  AND status = 'ATTACHED'
	`, reviewID, graceSeconds); err != nil {
		return fmt.Errorf("orphan media on review delete: %w", err)
	}
	return nil
}
