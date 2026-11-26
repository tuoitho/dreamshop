package grpc

import (
	"context"
	"testing"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
	"ecommerce.local/catalog-service/internal/media"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeReviewRepo struct {
	listResult   *domain.ReviewListResult
	eligResult   *domain.ReviewEligibility
	createReview *domain.ProductReview
	createSum    *domain.ReviewRatingSummary
	createErr    error
	updateReview *domain.ProductReview
	updateSum    *domain.ReviewRatingSummary
	updateErr    error
	deleteProdID string
	deleteSum    *domain.ReviewRatingSummary
	deleteErr    error
	adminList    *domain.AdminReviewListResult
	visReview    *domain.ProductReview
	visErr       error
	replyReview  *domain.ProductReview
	replyErr     error
}

func (f *fakeReviewRepo) RecordDeliveredOrder(context.Context, string, string, string, string, string, time.Time, []domain.DeliveredOrderItem) error {
	return nil
}

func (f *fakeReviewRepo) ListProductReviews(context.Context, domain.ReviewListFilter) (*domain.ReviewListResult, error) {
	return f.listResult, nil
}

func (f *fakeReviewRepo) GetReviewEligibility(context.Context, string, string) (*domain.ReviewEligibility, error) {
	return f.eligResult, nil
}

func (f *fakeReviewRepo) CreateReview(context.Context, domain.CreateReviewInput, time.Duration) (*domain.ProductReview, *domain.ReviewRatingSummary, error) {
	return f.createReview, f.createSum, f.createErr
}

func (f *fakeReviewRepo) UpdateReview(context.Context, domain.UpdateReviewInput, time.Duration) (*domain.ProductReview, *domain.ReviewRatingSummary, error) {
	return f.updateReview, f.updateSum, f.updateErr
}

func (f *fakeReviewRepo) DeleteReview(context.Context, string, string, time.Duration) (string, *domain.ReviewRatingSummary, error) {
	return f.deleteProdID, f.deleteSum, f.deleteErr
}

func (f *fakeReviewRepo) ListAdminReviews(context.Context, domain.AdminReviewFilter) (*domain.AdminReviewListResult, error) {
	return f.adminList, nil
}

func (f *fakeReviewRepo) SetReviewVisibility(context.Context, string, string, bool, string) (*domain.ProductReview, error) {
	return f.visReview, f.visErr
}

func (f *fakeReviewRepo) ReplyProductReview(context.Context, string, string, string) (*domain.ProductReview, error) {
	return f.replyReview, f.replyErr
}

type stubPermAuth struct {
	allowed map[string]bool
}

func (s *stubPermAuth) CheckPermission(_ context.Context, _, perm string) (bool, error) {
	return s.allowed[perm], nil
}

type stubReviewSigner struct{}

func (stubReviewSigner) Bucket() string                    { return "product-media" }
func (stubReviewSigner) EnsureBucket(context.Context) error { return nil }
func (stubReviewSigner) PresignPut(key, _ string, _ int64, _ time.Duration) (*media.SignedUpload, error) {
	return &media.SignedUpload{
		UploadURL:        "http://localhost:9000/product-media/" + key + "?sig=1",
		PublicURL:        "http://localhost:9000/product-media/" + key,
		ExpiresInSeconds: 300,
	}, nil
}

func extractReason(t *testing.T, err error) string {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %v", err)
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func TestCreateProductReviewAuthorizationAndErrors(t *testing.T) {
	rr := &fakeReviewRepo{
		createErr: domain.ErrReviewNotEligible,
	}
	srv := NewServer(nil, &stubPermAuth{allowed: map[string]bool{}}, WithReviewRepository(rr))

	_, err := srv.CreateProductReview(context.Background(), &catalogv1.CreateProductReviewRequest{
		UserId:    "u-1",
		ProductId: "dt-1",
		OrderId:   "ord-1",
		Rating:    5,
		Comment:   "Great",
	})
	if status.Code(err) != codes.PermissionDenied || extractReason(t, err) != reasonReviewPermissionDenied {
		t.Fatalf("expected REVIEW_PERMISSION_DENIED, got %v", err)
	}

	srvAllowed := NewServer(nil, &stubPermAuth{allowed: map[string]bool{reviewCreatePermission: true}}, WithReviewRepository(rr))
	_, err = srvAllowed.CreateProductReview(context.Background(), &catalogv1.CreateProductReviewRequest{
		UserId:    "u-1",
		ProductId: "dt-1",
		OrderId:   "ord-1",
		Rating:    5,
	})
	if status.Code(err) != codes.FailedPrecondition || extractReason(t, err) != reasonReviewNotEligible {
		t.Fatalf("expected REVIEW_NOT_ELIGIBLE, got %v", err)
	}

	rr.createErr = nil
	rr.createReview = &domain.ProductReview{
		ID:               "rev-1",
		ProductID:        "dt-1",
		UserID:           "u-1",
		OrderID:          "ord-1",
		AuthorName:       "Customer One",
		Rating:           5,
		Comment:          "Great",
		VerifiedPurchase: true,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	rr.createSum = &domain.ReviewRatingSummary{
		AverageRating: 5,
		TotalReviews:  1,
		Breakdown:     domain.EmptyRatingBreakdown(),
	}
	resp, err := srvAllowed.CreateProductReview(context.Background(), &catalogv1.CreateProductReviewRequest{
		UserId:    "u-1",
		ProductId: "dt-1",
		OrderId:   "ord-1",
		Rating:    5,
		Comment:   "Great",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetReview().GetId() != "rev-1" || resp.GetSummary().GetTotalReviews() != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAdminReviewRPCsRequirePermissions(t *testing.T) {
	rr := &fakeReviewRepo{
		adminList: &domain.AdminReviewListResult{
			Page:         1,
			PageSize:     10,
			VisibleCount: 2,
		},
		visReview: &domain.ProductReview{
			ID:           "rev-1",
			ProductID:    "dt-1",
			Hidden:       true,
			HiddenReason: "Spam",
		},
		replyReview: &domain.ProductReview{
			ID:         "rev-1",
			ProductID:  "dt-1",
			AdminReply: "Cam on ban",
		},
	}

	srvNoPerm := NewServer(nil, &stubPermAuth{allowed: map[string]bool{}}, WithReviewRepository(rr))
	if _, err := srvNoPerm.ListAdminReviews(context.Background(), &catalogv1.ListAdminReviewsRequest{UserId: "a-1"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied on ListAdminReviews, got %v", err)
	}
	if _, err := srvNoPerm.SetReviewVisibility(context.Background(), &catalogv1.SetReviewVisibilityRequest{UserId: "a-1", ReviewId: "rev-1", Hidden: true}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied on SetReviewVisibility, got %v", err)
	}
	if _, err := srvNoPerm.ReplyProductReview(context.Background(), &catalogv1.ReplyProductReviewRequest{UserId: "a-1", ReviewId: "rev-1", Reply: "Thanks"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied on ReplyProductReview, got %v", err)
	}

	srvAdmin := NewServer(nil, &stubPermAuth{allowed: map[string]bool{
		reviewAdminViewPermission:     true,
		reviewAdminModeratePermission: true,
	}}, WithReviewRepository(rr))

	listResp, err := srvAdmin.ListAdminReviews(context.Background(), &catalogv1.ListAdminReviewsRequest{UserId: "a-1"})
	if err != nil || listResp.GetVisibleCount() != 2 {
		t.Fatalf("unexpected ListAdminReviews result: resp=%+v err=%v", listResp, err)
	}
	visResp, err := srvAdmin.SetReviewVisibility(context.Background(), &catalogv1.SetReviewVisibilityRequest{UserId: "a-1", ReviewId: "rev-1", Hidden: true, Reason: "Spam"})
	if err != nil || !visResp.GetReview().GetHidden() {
		t.Fatalf("unexpected SetReviewVisibility result: resp=%+v err=%v", visResp, err)
	}
	replyResp, err := srvAdmin.ReplyProductReview(context.Background(), &catalogv1.ReplyProductReviewRequest{UserId: "a-1", ReviewId: "rev-1", Reply: "Cam on ban"})
	if err != nil || replyResp.GetReview().GetAdminReply() != "Cam on ban" {
		t.Fatalf("unexpected ReplyProductReview result: resp=%+v err=%v", replyResp, err)
	}
}

type stubMediaRepo struct {
	domain.Repository
	assets []*domain.MediaAsset
}

func (s *stubMediaRepo) CreateMediaAsset(_ context.Context, asset *domain.MediaAsset) error {
	s.assets = append(s.assets, asset)
	return nil
}

func TestCreateReviewMediaUploadValidatesImageAndSigns(t *testing.T) {
	baseRepo := &stubMediaRepo{}
	srv := NewServer(baseRepo, &stubPermAuth{allowed: map[string]bool{reviewCreatePermission: true}}, WithMediaSigner(stubReviewSigner{}, 5*time.Minute, 24*time.Hour))

	if _, err := srv.CreateReviewMediaUpload(context.Background(), &catalogv1.CreateReviewMediaUploadRequest{
		UserId:      "u-1",
		Filename:    "clip.mp4",
		ContentType: "video/mp4",
		SizeBytes:   1024,
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for video upload, got %v", err)
	}

	resp, err := srv.CreateReviewMediaUpload(context.Background(), &catalogv1.CreateReviewMediaUploadRequest{
		UserId:      "u-1",
		Filename:    "review.png",
		ContentType: "image/png",
		SizeBytes:   2048,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetMediaType() != domain.MediaTypeImage || resp.GetUploadUrl() == "" || resp.GetPublicUrl() == "" {
		t.Fatalf("unexpected presign response: %+v", resp)
	}
}
