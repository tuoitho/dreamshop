package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxReviewImagesCount  = 5
	MaxReviewCommentRunes = 2000
	MaxReviewAuthorRunes  = 100
	MaxReviewReplyRunes   = 1000
	MaxReviewReasonRunes  = 255

	ReviewSortNewest  = "newest"
	ReviewSortHighest = "highest"
	ReviewSortLowest  = "lowest"

	AdminReviewStatusAll     = "all"
	AdminReviewStatusVisible = "visible"
	AdminReviewStatusHidden  = "hidden"

	AdminReplyStatusAll       = "all"
	AdminReplyStatusReplied   = "replied"
	AdminReplyStatusUnreplied = "unreplied"

	defaultReviewPageSize = 10
	maxReviewPageSize     = 50
)

var (
	// ErrReviewNotFound is returned when no review matches the requested id.
	ErrReviewNotFound = errors.New("review not found")

	// ErrReviewNotEligible is returned when the user has not had the product delivered in the given order.
	ErrReviewNotEligible = errors.New("order is not eligible to review this product")

	// ErrReviewAlreadyExists is returned when the user already reviewed the product for the same order.
	ErrReviewAlreadyExists = errors.New("review already exists for this order and product")

	// ErrReviewForbidden is returned when a user tries to modify or delete another user's review.
	ErrReviewForbidden = errors.New("review belongs to another user")
)

// ProductReview represents a verified customer review on a product.
type ProductReview struct {
	ID               string
	ProductID        string
	ProductName      string
	UserID           string
	OrderID          string
	SkuID            string
	VariantLabel     string
	AuthorName       string
	Rating           int32
	Comment          string
	Images           []string
	VerifiedPurchase bool
	Hidden           bool
	HiddenReason     string
	AdminReply       string
	AdminRepliedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RatingBreakdown stores the visible review count for one star value (1..5).
type RatingBreakdown struct {
	Rating int32
	Count  int64
}

// ReviewRatingSummary aggregates visible ratings for a product.
type ReviewRatingSummary struct {
	AverageRating float64
	TotalReviews  int64
	Breakdown     []RatingBreakdown
}

// ReviewListFilter controls storefront review pagination and filtering.
type ReviewListFilter struct {
	ProductID string
	Rating    int32
	HasImages bool
	Sort      string
	Page      int32
	PageSize  int32
}

// Normalize clamps filter values to safe defaults.
func (f ReviewListFilter) Normalize() ReviewListFilter {
	f.ProductID = strings.TrimSpace(f.ProductID)
	if f.Rating < 0 || f.Rating > 5 {
		f.Rating = 0
	}
	switch strings.ToLower(strings.TrimSpace(f.Sort)) {
	case ReviewSortHighest:
		f.Sort = ReviewSortHighest
	case ReviewSortLowest:
		f.Sort = ReviewSortLowest
	default:
		f.Sort = ReviewSortNewest
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultReviewPageSize
	}
	if f.PageSize > maxReviewPageSize {
		f.PageSize = maxReviewPageSize
	}
	return f
}

// ReviewListResult is the paginated storefront review response.
type ReviewListResult struct {
	Items      []*ProductReview
	Page       int32
	PageSize   int32
	TotalItems int64
	TotalPages int32
	Summary    ReviewRatingSummary
}

// EligibleReviewOrder represents a delivered order item eligible for reviewing a product.
type EligibleReviewOrder struct {
	OrderID      string
	OrderCode    string
	SkuID        string
	VariantLabel string
	DeliveredAt  time.Time
	Reviewed     bool
	ReviewID     string
}

// ReviewEligibility captures whether a user can write a review for a product and their existing reviews.
type ReviewEligibility struct {
	CanReview      bool
	EligibleOrders []EligibleReviewOrder
	MyReviews      []*ProductReview
}

// DeliveredOrderItem is one product line recorded from an OrderDelivered event.
type DeliveredOrderItem struct {
	ProductID    string
	SkuID        string
	VariantLabel string
}

// CreateReviewInput carries validated inputs for creating a product review.
type CreateReviewInput struct {
	UserID     string
	ProductID  string
	OrderID    string
	AuthorName string
	Rating     int32
	Comment    string
	Images     []string
}

// UpdateReviewInput carries validated inputs for editing an existing product review.
type UpdateReviewInput struct {
	UserID   string
	ReviewID string
	Rating   int32
	Comment  string
	Images   []string
}

// AdminReviewFilter filters administrative review moderation listings.
type AdminReviewFilter struct {
	Query       string
	ProductID   string
	Rating      int32
	Status      string
	ReplyStatus string
	Page        int32
	PageSize    int32
}

// Normalize clamps admin review filter values to safe defaults.
func (f AdminReviewFilter) Normalize() AdminReviewFilter {
	f.Query = strings.TrimSpace(f.Query)
	f.ProductID = strings.TrimSpace(f.ProductID)
	if f.Rating < 0 || f.Rating > 5 {
		f.Rating = 0
	}
	switch strings.ToLower(strings.TrimSpace(f.Status)) {
	case AdminReviewStatusVisible:
		f.Status = AdminReviewStatusVisible
	case AdminReviewStatusHidden:
		f.Status = AdminReviewStatusHidden
	default:
		f.Status = AdminReviewStatusAll
	}
	switch strings.ToLower(strings.TrimSpace(f.ReplyStatus)) {
	case AdminReplyStatusReplied:
		f.ReplyStatus = AdminReplyStatusReplied
	case AdminReplyStatusUnreplied:
		f.ReplyStatus = AdminReplyStatusUnreplied
	default:
		f.ReplyStatus = AdminReplyStatusAll
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultPageSize
	}
	if f.PageSize > adminMaxPageSize {
		f.PageSize = adminMaxPageSize
	}
	return f
}

// AdminReviewListResult is the paginated moderation list with global KPIs.
type AdminReviewListResult struct {
	Items          []*ProductReview
	Page           int32
	PageSize       int32
	TotalItems     int64
	TotalPages     int32
	VisibleCount   int64
	HiddenCount    int64
	UnrepliedCount int64
	AverageRating  float64
}

// ReviewRepository defines persistence operations for product reviews and delivery eligibility.
type ReviewRepository interface {
	RecordDeliveredOrder(ctx context.Context, eventID, orderID, orderCode, userID, customerName string, deliveredAt time.Time, items []DeliveredOrderItem) error
	ListProductReviews(ctx context.Context, filter ReviewListFilter) (*ReviewListResult, error)
	GetReviewEligibility(ctx context.Context, userID, productID string) (*ReviewEligibility, error)
	CreateReview(ctx context.Context, in CreateReviewInput, orphanGrace time.Duration) (*ProductReview, *ReviewRatingSummary, error)
	UpdateReview(ctx context.Context, in UpdateReviewInput, orphanGrace time.Duration) (*ProductReview, *ReviewRatingSummary, error)
	DeleteReview(ctx context.Context, userID, reviewID string, orphanGrace time.Duration) (string, *ReviewRatingSummary, error)
	ListAdminReviews(ctx context.Context, filter AdminReviewFilter) (*AdminReviewListResult, error)
	SetReviewVisibility(ctx context.Context, reviewID, adminID string, hidden bool, reason string) (*ProductReview, error)
	ReplyProductReview(ctx context.Context, reviewID, adminID, reply string) (*ProductReview, error)
}

// ValidateReviewMediaUpload restricts review uploads to images (<= 5 MB).
func ValidateReviewMediaUpload(filename, contentType string, sizeBytes int64) (*MediaUploadSpec, error) {
	spec, err := ValidateMediaUpload(filename, contentType, sizeBytes)
	if err != nil {
		return nil, err
	}
	if spec.MediaType != MediaTypeImage {
		return nil, &ValidationError{
			Field:   "content_type",
			Message: "review uploads support images only (image/jpeg, image/png, image/webp, image/avif)",
		}
	}
	return spec, nil
}

// ValidateCreateReview normalizes and validates a new review submission.
func ValidateCreateReview(in CreateReviewInput) (CreateReviewInput, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	if in.UserID == "" {
		return in, &ValidationError{Field: "user_id", Message: "is required"}
	}
	in.ProductID = strings.TrimSpace(in.ProductID)
	if in.ProductID == "" {
		return in, &ValidationError{Field: "product_id", Message: "is required"}
	}
	in.OrderID = strings.TrimSpace(in.OrderID)
	if in.OrderID == "" {
		return in, &ValidationError{Field: "order_id", Message: "is required"}
	}
	if in.Rating < 1 || in.Rating > 5 {
		return in, &ValidationError{Field: "rating", Message: "must be between 1 and 5"}
	}
	in.AuthorName = strings.TrimSpace(in.AuthorName)
	if utf8.RuneCountInString(in.AuthorName) > MaxReviewAuthorRunes {
		return in, &ValidationError{Field: "author_name", Message: "must not exceed 100 characters"}
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if utf8.RuneCountInString(in.Comment) > MaxReviewCommentRunes {
		return in, &ValidationError{Field: "comment", Message: "must not exceed 2000 characters"}
	}
	images, err := sanitizeMediaURLs("images", in.Images, MaxReviewImagesCount)
	if err != nil {
		return in, err
	}
	in.Images = images
	return in, nil
}

// ValidateUpdateReview normalizes and validates a review update.
func ValidateUpdateReview(in UpdateReviewInput) (UpdateReviewInput, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	if in.UserID == "" {
		return in, &ValidationError{Field: "user_id", Message: "is required"}
	}
	in.ReviewID = strings.TrimSpace(in.ReviewID)
	if in.ReviewID == "" {
		return in, &ValidationError{Field: "review_id", Message: "is required"}
	}
	if in.Rating < 1 || in.Rating > 5 {
		return in, &ValidationError{Field: "rating", Message: "must be between 1 and 5"}
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if utf8.RuneCountInString(in.Comment) > MaxReviewCommentRunes {
		return in, &ValidationError{Field: "comment", Message: "must not exceed 2000 characters"}
	}
	images, err := sanitizeMediaURLs("images", in.Images, MaxReviewImagesCount)
	if err != nil {
		return in, err
	}
	in.Images = images
	return in, nil
}

// ValidateAdminReply validates and trims an official store reply (empty clears reply).
func ValidateAdminReply(reply string) (string, error) {
	clean := strings.TrimSpace(reply)
	if utf8.RuneCountInString(clean) > MaxReviewReplyRunes {
		return "", &ValidationError{Field: "reply", Message: "must not exceed 1000 characters"}
	}
	return clean, nil
}

// ValidateHideReason validates and trims a moderation reason when hiding a review.
func ValidateHideReason(reason string) (string, error) {
	clean := strings.TrimSpace(reason)
	if utf8.RuneCountInString(clean) > MaxReviewReasonRunes {
		return "", &ValidationError{Field: "reason", Message: "must not exceed 255 characters"}
	}
	return clean, nil
}

// EmptyRatingBreakdown initializes the canonical 5..1 star breakdown slice.
func EmptyRatingBreakdown() []RatingBreakdown {
	return []RatingBreakdown{
		{Rating: 5, Count: 0},
		{Rating: 4, Count: 0},
		{Rating: 3, Count: 0},
		{Rating: 2, Count: 0},
		{Rating: 1, Count: 0},
	}
}
