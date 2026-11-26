package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) requireReviewRepo() (domain.ReviewRepository, error) {
	if s.reviewRepo == nil {
		return nil, status.Error(codes.Unimplemented, "review repository is not configured")
	}
	return s.reviewRepo, nil
}

func (s *Server) authorizeReview(ctx context.Context, userID, permission string) error {
	cleanUserID := strings.TrimSpace(userID)
	if cleanUserID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.authorization == nil {
		slog.ErrorContext(ctx, "review authorization denied: authorization client not configured", "user_id", cleanUserID, "permission", permission)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	authCtx, traceID := withAuthorizationTrace(ctx)
	allowed, err := s.authorization.CheckPermission(authCtx, cleanUserID, permission)
	if err != nil {
		slog.ErrorContext(ctx, "review authorization check failed", "user_id", cleanUserID, "permission", permission, "trace_id", traceID, "error", err)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	if !allowed {
		return statusWithReason(codes.PermissionDenied, fmt.Sprintf("missing %s permission", permission), reasonReviewPermissionDenied)
	}
	return nil
}

// ListProductReviews returns public visible reviews and rating summary for one product.
func (s *Server) ListProductReviews(ctx context.Context, req *catalogv1.ListProductReviewsRequest) (*catalogv1.ListProductReviewsResponse, error) {
	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	res, err := repo.ListProductReviews(ctx, domain.ReviewListFilter{
		ProductID: productID,
		Rating:    req.GetRating(),
		HasImages: req.GetHasImages(),
		Sort:      req.GetSort(),
		Page:      req.GetPage(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list product reviews %q: %v", productID, err)
	}

	out := &catalogv1.ListProductReviewsResponse{
		Page:       res.Page,
		PageSize:   res.PageSize,
		TotalItems: res.TotalItems,
		TotalPages: res.TotalPages,
		Summary:    reviewSummaryToProto(&res.Summary),
	}
	for _, item := range res.Items {
		out.Items = append(out.Items, reviewToProto(item))
	}
	return out, nil
}

// GetReviewEligibility returns delivered orders eligible for review and existing reviews by the caller.
func (s *Server) GetReviewEligibility(ctx context.Context, req *catalogv1.GetReviewEligibilityRequest) (*catalogv1.GetReviewEligibilityResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	productID := strings.TrimSpace(req.GetProductId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	elig, err := repo.GetReviewEligibility(ctx, userID, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get review eligibility: %v", err)
	}

	out := &catalogv1.GetReviewEligibilityResponse{
		CanReview: elig.CanReview,
	}
	for _, ord := range elig.EligibleOrders {
		var deliveredAt string
		if !ord.DeliveredAt.IsZero() {
			deliveredAt = ord.DeliveredAt.UTC().Format(time.RFC3339)
		}
		out.EligibleOrders = append(out.EligibleOrders, &catalogv1.EligibleReviewOrder{
			OrderId:      ord.OrderID,
			OrderCode:    ord.OrderCode,
			SkuId:        ord.SkuID,
			VariantLabel: ord.VariantLabel,
			DeliveredAt:  deliveredAt,
			Reviewed:     ord.Reviewed,
			ReviewId:     ord.ReviewID,
		})
	}
	for _, rev := range elig.MyReviews {
		out.MyReviews = append(out.MyReviews, reviewToProto(rev))
	}
	return out, nil
}

// CreateProductReview authorizes review.create, verifies delivery eligibility, and persists a new review.
func (s *Server) CreateProductReview(ctx context.Context, req *catalogv1.CreateProductReviewRequest) (*catalogv1.CreateProductReviewResponse, error) {
	prepared, err := domain.ValidateCreateReview(domain.CreateReviewInput{
		UserID:     req.GetUserId(),
		ProductID:  req.GetProductId(),
		OrderID:    req.GetOrderId(),
		AuthorName: req.GetAuthorName(),
		Rating:     req.GetRating(),
		Comment:    req.GetComment(),
		Images:     req.GetImages(),
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeReview(ctx, prepared.UserID, reviewCreatePermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	review, summary, err := repo.CreateReview(ctx, prepared, s.orphanGrace)
	if err != nil {
		return nil, mapReviewError(err, prepared.ProductID)
	}
	return &catalogv1.CreateProductReviewResponse{
		Review:  reviewToProto(review),
		Summary: reviewSummaryToProto(summary),
	}, nil
}

// UpdateProductReview updates the caller's own review and recalculates product rating.
func (s *Server) UpdateProductReview(ctx context.Context, req *catalogv1.UpdateProductReviewRequest) (*catalogv1.UpdateProductReviewResponse, error) {
	prepared, err := domain.ValidateUpdateReview(domain.UpdateReviewInput{
		UserID:   req.GetUserId(),
		ReviewID: req.GetReviewId(),
		Rating:   req.GetRating(),
		Comment:  req.GetComment(),
		Images:   req.GetImages(),
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeReview(ctx, prepared.UserID, reviewCreatePermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	review, summary, err := repo.UpdateReview(ctx, prepared, s.orphanGrace)
	if err != nil {
		return nil, mapReviewError(err, prepared.ReviewID)
	}
	return &catalogv1.UpdateProductReviewResponse{
		Review:  reviewToProto(review),
		Summary: reviewSummaryToProto(summary),
	}, nil
}

// DeleteProductReview deletes the caller's own review and recalculates product rating.
func (s *Server) DeleteProductReview(ctx context.Context, req *catalogv1.DeleteProductReviewRequest) (*catalogv1.DeleteProductReviewResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	reviewID := strings.TrimSpace(req.GetReviewId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if reviewID == "" {
		return nil, status.Error(codes.InvalidArgument, "review_id is required")
	}
	if err := s.authorizeReview(ctx, userID, reviewCreatePermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	productID, summary, err := repo.DeleteReview(ctx, userID, reviewID, s.orphanGrace)
	if err != nil {
		return nil, mapReviewError(err, reviewID)
	}
	return &catalogv1.DeleteProductReviewResponse{
		ReviewId:  reviewID,
		ProductId: productID,
		Summary:   reviewSummaryToProto(summary),
	}, nil
}

// CreateReviewMediaUpload authorizes review.create, records a PENDING asset, and signs a MinIO PUT URL.
func (s *Server) CreateReviewMediaUpload(ctx context.Context, req *catalogv1.CreateReviewMediaUploadRequest) (*catalogv1.CreateReviewMediaUploadResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	spec, err := domain.ValidateReviewMediaUpload(req.GetFilename(), req.GetContentType(), req.GetSizeBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeReview(ctx, userID, reviewCreatePermission); err != nil {
		return nil, err
	}
	if s.mediaSigner == nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}
	if err := s.mediaSigner.EnsureBucket(ctx); err != nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}

	uploadID, err := domain.NewUUIDv4()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate upload id: %v", err)
	}
	now := time.Now().UTC()
	objectKey := fmt.Sprintf("reviews/%04d/%02d/%s%s", now.Year(), int(now.Month()), uploadID, spec.Extension)

	signed, err := s.mediaSigner.PresignPut(objectKey, spec.ContentType, spec.SizeBytes, s.presignTTL)
	if err != nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}

	expiresAt := now.Add(s.orphanGrace)
	asset := &domain.MediaAsset{
		ID:               uploadID,
		Bucket:           s.mediaSigner.Bucket(),
		ObjectKey:        objectKey,
		PublicURL:        signed.PublicURL,
		MediaType:        spec.MediaType,
		OriginalFilename: spec.OriginalFilename,
		ContentType:      spec.ContentType,
		SizeBytes:        spec.SizeBytes,
		Status:           domain.MediaStatusPending,
		UploadedBy:       userID,
		ExpiresAt:        &expiresAt,
	}
	if err := s.repo.CreateMediaAsset(ctx, asset); err != nil {
		return nil, status.Errorf(codes.Internal, "record pending review media upload: %v", err)
	}

	return &catalogv1.CreateReviewMediaUploadResponse{
		UploadId:         uploadID,
		MediaType:        spec.MediaType,
		UploadUrl:        signed.UploadURL,
		PublicUrl:        signed.PublicURL,
		ObjectKey:        objectKey,
		ExpiresInSeconds: signed.ExpiresInSeconds,
	}, nil
}

// ListAdminReviews returns paginated reviews for moderation after verifying review.admin.view.
func (s *Server) ListAdminReviews(ctx context.Context, req *catalogv1.ListAdminReviewsRequest) (*catalogv1.ListAdminReviewsResponse, error) {
	if err := s.authorizeReview(ctx, req.GetUserId(), reviewAdminViewPermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	res, err := repo.ListAdminReviews(ctx, domain.AdminReviewFilter{
		Query:       req.GetQuery(),
		ProductID:   req.GetProductId(),
		Rating:      req.GetRating(),
		Status:      req.GetStatus(),
		ReplyStatus: req.GetReplyStatus(),
		Page:        req.GetPage(),
		PageSize:    req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list admin reviews: %v", err)
	}

	out := &catalogv1.ListAdminReviewsResponse{
		Page:           res.Page,
		PageSize:       res.PageSize,
		TotalItems:     res.TotalItems,
		TotalPages:     res.TotalPages,
		VisibleCount:   res.VisibleCount,
		HiddenCount:    res.HiddenCount,
		UnrepliedCount: res.UnrepliedCount,
		AverageRating:  res.AverageRating,
	}
	for _, item := range res.Items {
		out.Items = append(out.Items, reviewToProto(item))
	}
	return out, nil
}

// SetReviewVisibility hides or restores a review after verifying review.admin.moderate.
func (s *Server) SetReviewVisibility(ctx context.Context, req *catalogv1.SetReviewVisibilityRequest) (*catalogv1.SetReviewVisibilityResponse, error) {
	reviewID := strings.TrimSpace(req.GetReviewId())
	if reviewID == "" {
		return nil, status.Error(codes.InvalidArgument, "review_id is required")
	}
	reason, err := domain.ValidateHideReason(req.GetReason())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeReview(ctx, req.GetUserId(), reviewAdminModeratePermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	updated, err := repo.SetReviewVisibility(ctx, reviewID, strings.TrimSpace(req.GetUserId()), req.GetHidden(), reason)
	if err != nil {
		return nil, mapReviewError(err, reviewID)
	}
	return &catalogv1.SetReviewVisibilityResponse{
		Review: reviewToProto(updated),
	}, nil
}

// ReplyProductReview posts or clears an official store reply after verifying review.admin.moderate.
func (s *Server) ReplyProductReview(ctx context.Context, req *catalogv1.ReplyProductReviewRequest) (*catalogv1.ReplyProductReviewResponse, error) {
	reviewID := strings.TrimSpace(req.GetReviewId())
	if reviewID == "" {
		return nil, status.Error(codes.InvalidArgument, "review_id is required")
	}
	reply, err := domain.ValidateAdminReply(req.GetReply())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeReview(ctx, req.GetUserId(), reviewAdminModeratePermission); err != nil {
		return nil, err
	}
	repo, err := s.requireReviewRepo()
	if err != nil {
		return nil, err
	}

	updated, err := repo.ReplyProductReview(ctx, reviewID, strings.TrimSpace(req.GetUserId()), reply)
	if err != nil {
		return nil, mapReviewError(err, reviewID)
	}
	return &catalogv1.ReplyProductReviewResponse{
		Review: reviewToProto(updated),
	}, nil
}

func mapReviewError(err error, targetID string) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		return notFoundError(targetID)
	case errors.Is(err, domain.ErrReviewNotFound):
		return statusWithReason(codes.NotFound, "review not found", reasonReviewNotFound)
	case errors.Is(err, domain.ErrReviewNotEligible):
		return statusWithReason(codes.FailedPrecondition, "only customers with delivered orders can review this product", reasonReviewNotEligible)
	case errors.Is(err, domain.ErrReviewAlreadyExists):
		return statusWithReason(codes.AlreadyExists, "you have already reviewed this product for this order", reasonReviewAlreadyExists)
	case errors.Is(err, domain.ErrReviewForbidden):
		return statusWithReason(codes.PermissionDenied, "you can only modify your own review", reasonReviewForbidden)
	case errors.Is(err, domain.ErrMediaAlreadyAttached):
		return statusWithReason(codes.FailedPrecondition, "media asset is already attached", reasonMediaAlreadyAttached)
	case errors.Is(err, domain.ErrMediaExpired):
		return status.Error(codes.InvalidArgument, "media asset upload has expired; please re-upload")
	default:
		return status.Errorf(codes.Internal, "review operation failed: %v", err)
	}
}

func reviewToProto(r *domain.ProductReview) *catalogv1.ProductReview {
	if r == nil {
		return nil
	}
	var repliedAt, createdAt, updatedAt string
	if r.AdminRepliedAt != nil && !r.AdminRepliedAt.IsZero() {
		repliedAt = r.AdminRepliedAt.UTC().Format(time.RFC3339)
	}
	if !r.CreatedAt.IsZero() {
		createdAt = r.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !r.UpdatedAt.IsZero() {
		updatedAt = r.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return &catalogv1.ProductReview{
		Id:               r.ID,
		ProductId:        r.ProductID,
		ProductName:      r.ProductName,
		UserId:           r.UserID,
		OrderId:          r.OrderID,
		SkuId:            r.SkuID,
		VariantLabel:     r.VariantLabel,
		AuthorName:       r.AuthorName,
		Rating:           r.Rating,
		Comment:          r.Comment,
		Images:           r.Images,
		VerifiedPurchase: r.VerifiedPurchase,
		Hidden:           r.Hidden,
		HiddenReason:     r.HiddenReason,
		AdminReply:       r.AdminReply,
		AdminRepliedAt:   repliedAt,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}
}

func reviewSummaryToProto(s *domain.ReviewRatingSummary) *catalogv1.ReviewRatingSummary {
	if s == nil {
		return &catalogv1.ReviewRatingSummary{}
	}
	out := &catalogv1.ReviewRatingSummary{
		AverageRating: s.AverageRating,
		TotalReviews:  s.TotalReviews,
	}
	for _, b := range s.Breakdown {
		out.Breakdown = append(out.Breakdown, &catalogv1.RatingBreakdown{
			Rating: b.Rating,
			Count:  b.Count,
		})
	}
	return out
}
