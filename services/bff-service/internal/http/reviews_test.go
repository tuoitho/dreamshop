package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce.local/bff-service/internal/catalog"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
)

func TestStorefrontAndCustomerReviewRoutes(t *testing.T) {
	cat := &fakeCatalogClient{
		listReviewsFn: func(_ context.Context, req *catalogv1.ListProductReviewsRequest) (*catalogv1.ListProductReviewsResponse, error) {
			if req.GetProductId() != "dt-1" || req.GetRating() != 5 || !req.GetHasImages() || req.GetSort() != "highest" {
				t.Fatalf("unexpected ListProductReviewsRequest: %+v", req)
			}
			return &catalogv1.ListProductReviewsResponse{
				Items: []*catalogv1.ProductReview{
					{
						Id:               "rev-1",
						ProductId:        "dt-1",
						ProductName:      "Nova X",
						UserId:           "user-1",
						OrderId:          "ord-1",
						SkuId:            "dt-1-v1",
						VariantLabel:     "256GB",
						AuthorName:       "Customer One",
						Rating:           5,
						Comment:          "Rat tot",
						Images:           []string{"http://localhost:9000/product-media/reviews/2026/11/a.png"},
						VerifiedPurchase: true,
						CreatedAt:        "2026-11-26T04:00:00Z",
						UpdatedAt:        "2026-11-26T04:00:00Z",
					},
				},
				Page:       1,
				PageSize:   10,
				TotalItems: 1,
				TotalPages: 1,
				Summary: &catalogv1.ReviewRatingSummary{
					AverageRating: 5,
					TotalReviews:  1,
					Breakdown: []*catalogv1.RatingBreakdown{
						{Rating: 5, Count: 1},
					},
				},
			}, nil
		},
		reviewEligFn: func(_ context.Context, req *catalogv1.GetReviewEligibilityRequest) (*catalogv1.GetReviewEligibilityResponse, error) {
			return &catalogv1.GetReviewEligibilityResponse{
				CanReview: true,
				EligibleOrders: []*catalogv1.EligibleReviewOrder{
					{OrderId: "ord-1", OrderCode: "ORD-001", SkuId: "dt-1-v1", VariantLabel: "256GB", DeliveredAt: "2026-11-26T03:00:00Z"},
				},
			}, nil
		},
		createReviewFn: func(_ context.Context, req *catalogv1.CreateProductReviewRequest) (*catalogv1.CreateProductReviewResponse, error) {
			if req.GetOrderId() == "ord-ineligible" {
				return nil, catalog.ErrReviewNotEligible
			}
			return &catalogv1.CreateProductReviewResponse{
				Review: &catalogv1.ProductReview{
					Id:               "rev-2",
					ProductId:        req.GetProductId(),
					UserId:           req.GetUserId(),
					OrderId:          req.GetOrderId(),
					AuthorName:       req.GetAuthorName(),
					Rating:           req.GetRating(),
					Comment:          req.GetComment(),
					VerifiedPurchase: true,
				},
				Summary: &catalogv1.ReviewRatingSummary{AverageRating: 5, TotalReviews: 2},
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewReviewsHandler(cat).Register(mux)

	// Public list reviews
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products/dt-1/reviews?rating=5&hasImages=true&sort=highest", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET reviews status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// Unauthenticated eligibility -> 401
	recUnauth := httptest.NewRecorder()
	mux.ServeHTTP(recUnauth, httptest.NewRequest(http.MethodGet, "/api/v1/reviews/eligibility?productId=dt-1", nil))
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated eligibility status = %d, want 401", recUnauth.Code)
	}

	// Authenticated create review -> 201
	body, _ := json.Marshal(createReviewRequest{
		ProductID:  "dt-1",
		OrderID:    "ord-1",
		AuthorName: "Customer One",
		Rating:     5,
		Comment:    "Tuyet voi",
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(body))
	reqCreate.Header.Set("X-User-Id", "user-1")
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/reviews status = %d, want 201; body=%s", recCreate.Code, recCreate.Body.String())
	}

	// Ineligible order -> 409 REVIEW_NOT_ELIGIBLE
	bodyInelig, _ := json.Marshal(createReviewRequest{ProductID: "dt-1", OrderID: "ord-ineligible", Rating: 5})
	reqInelig := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyInelig))
	reqInelig.Header.Set("X-User-Id", "user-1")
	reqInelig.Header.Set("Content-Type", "application/json")
	recInelig := httptest.NewRecorder()
	mux.ServeHTTP(recInelig, reqInelig)
	if recInelig.Code != http.StatusConflict {
		t.Fatalf("ineligible review status = %d, want 409; body=%s", recInelig.Code, recInelig.Body.String())
	}
}

func TestAdminReviewRoutes(t *testing.T) {
	cat := &fakeCatalogClient{
		listAdminRevFn: func(_ context.Context, req *catalogv1.ListAdminReviewsRequest) (*catalogv1.ListAdminReviewsResponse, error) {
			if req.GetUserId() != "admin-1" || req.GetStatus() != "visible" {
				t.Fatalf("unexpected admin list req: %+v", req)
			}
			return &catalogv1.ListAdminReviewsResponse{
				Items:          []*catalogv1.ProductReview{{Id: "rev-1", ProductId: "dt-1", Rating: 5}},
				Page:           1,
				PageSize:       20,
				TotalItems:     1,
				TotalPages:     1,
				VisibleCount:   1,
				HiddenCount:    0,
				UnrepliedCount: 1,
				AverageRating:  5,
			}, nil
		},
		setRevVisFn: func(_ context.Context, req *catalogv1.SetReviewVisibilityRequest) (*catalogv1.SetReviewVisibilityResponse, error) {
			return &catalogv1.SetReviewVisibilityResponse{
				Review: &catalogv1.ProductReview{Id: req.GetReviewId(), Hidden: req.GetHidden(), HiddenReason: req.GetReason()},
			}, nil
		},
		replyReviewFn: func(_ context.Context, req *catalogv1.ReplyProductReviewRequest) (*catalogv1.ReplyProductReviewResponse, error) {
			return &catalogv1.ReplyProductReviewResponse{
				Review: &catalogv1.ProductReview{Id: req.GetReviewId(), AdminReply: req.GetReply(), AdminRepliedAt: "2026-11-26T05:00:00Z"},
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewAdminReviewsHandler(cat).Register(mux)

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reviews?status=visible", nil)
	reqList.Header.Set("X-User-Id", "admin-1")
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/admin/reviews status = %d, want 200", recList.Code)
	}

	reqVis := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/reviews/rev-1/visibility", bytes.NewBufferString(`{"hidden":true,"reason":"Spam"}`))
	reqVis.Header.Set("X-User-Id", "admin-1")
	reqVis.Header.Set("Content-Type", "application/json")
	recVis := httptest.NewRecorder()
	mux.ServeHTTP(recVis, reqVis)
	if recVis.Code != http.StatusOK {
		t.Fatalf("PATCH visibility status = %d, want 200; body=%s", recVis.Code, recVis.Body.String())
	}

	reqReply := httptest.NewRequest(http.MethodPut, "/api/v1/admin/reviews/rev-1/reply", bytes.NewBufferString(`{"reply":"Cam on ban"}`))
	reqReply.Header.Set("X-User-Id", "admin-1")
	reqReply.Header.Set("Content-Type", "application/json")
	recReply := httptest.NewRecorder()
	mux.ServeHTTP(recReply, reqReply)
	if recReply.Code != http.StatusOK {
		t.Fatalf("PUT reply status = %d, want 200; body=%s", recReply.Code, recReply.Body.String())
	}
}

func TestReviewUpdateDeletePresignAndErrorCodes(t *testing.T) {
	cat := &fakeCatalogClient{
		updateReviewFn: func(_ context.Context, req *catalogv1.UpdateProductReviewRequest) (*catalogv1.UpdateProductReviewResponse, error) {
			if req.GetReviewId() == "rev-denied" {
				return nil, catalog.ErrPermissionDenied
			}
			if req.GetReviewId() == "rev-auth-down" {
				return nil, catalog.ErrAuthorizationUnavailable
			}
			return &catalogv1.UpdateProductReviewResponse{
				Review:  &catalogv1.ProductReview{Id: req.GetReviewId(), Rating: req.GetRating(), Comment: req.GetComment()},
				Summary: &catalogv1.ReviewRatingSummary{AverageRating: 4.5, TotalReviews: 2},
			}, nil
		},
		deleteReviewFn: func(_ context.Context, req *catalogv1.DeleteProductReviewRequest) (*catalogv1.DeleteProductReviewResponse, error) {
			return &catalogv1.DeleteProductReviewResponse{
				ReviewId:  req.GetReviewId(),
				ProductId: "dt-1",
				Summary:   &catalogv1.ReviewRatingSummary{AverageRating: 5, TotalReviews: 1},
			}, nil
		},
		createRevMediaFn: func(_ context.Context, _ *catalogv1.CreateReviewMediaUploadRequest) (*catalogv1.CreateReviewMediaUploadResponse, error) {
			return &catalogv1.CreateReviewMediaUploadResponse{
				UploadId:         "up-1",
				MediaType:        "IMAGE",
				UploadUrl:        "http://localhost:9000/product-media/reviews/2026/11/a.png?sig=1",
				PublicUrl:        "http://localhost:9000/product-media/reviews/2026/11/a.png",
				ObjectKey:        "reviews/2026/11/a.png",
				ExpiresInSeconds: 300,
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewReviewsHandler(cat).Register(mux)

	// Valid PUT update
	reqUpd := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/rev-1", bytes.NewBufferString(`{"rating":4,"comment":"Updated","images":[]}`))
	reqUpd.Header.Set("X-User-Id", "user-1")
	recUpd := httptest.NewRecorder()
	mux.ServeHTTP(recUpd, reqUpd)
	if recUpd.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/reviews/rev-1 status = %d, want 200", recUpd.Code)
	}

	// Trailing JSON value rejected
	reqTrailing := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/rev-1", bytes.NewBufferString(`{"rating":4}{"extra":1}`))
	reqTrailing.Header.Set("X-User-Id", "user-1")
	recTrailing := httptest.NewRecorder()
	mux.ServeHTTP(recTrailing, reqTrailing)
	if recTrailing.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON status = %d, want 400", recTrailing.Code)
	}

	// 403 REVIEW_PERMISSION_DENIED
	reqDenied := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/rev-denied", bytes.NewBufferString(`{"rating":4}`))
	reqDenied.Header.Set("X-User-Id", "user-1")
	recDenied := httptest.NewRecorder()
	mux.ServeHTTP(recDenied, reqDenied)
	if recDenied.Code != http.StatusForbidden {
		t.Fatalf("permission denied status = %d, want 403", recDenied.Code)
	}
	var errBody map[string]any
	_ = json.Unmarshal(recDenied.Body.Bytes(), &errBody)
	if errBody["code"] != "REVIEW_PERMISSION_DENIED" {
		t.Fatalf("permission denied code = %v, want REVIEW_PERMISSION_DENIED", errBody["code"])
	}

	// 503 AUTHORIZATION_SERVICE_UNAVAILABLE (consistent with all other BFF handlers and FE error maps)
	reqAuthDown := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/rev-auth-down", bytes.NewBufferString(`{"rating":4}`))
	reqAuthDown.Header.Set("X-User-Id", "user-1")
	recAuthDown := httptest.NewRecorder()
	mux.ServeHTTP(recAuthDown, reqAuthDown)
	if recAuthDown.Code != http.StatusServiceUnavailable {
		t.Fatalf("auth down status = %d, want 503", recAuthDown.Code)
	}
	errBody = nil
	_ = json.Unmarshal(recAuthDown.Body.Bytes(), &errBody)
	if errBody["code"] != "AUTHORIZATION_SERVICE_UNAVAILABLE" {
		t.Fatalf("auth down code = %v, want AUTHORIZATION_SERVICE_UNAVAILABLE", errBody["code"])
	}

	// DELETE review
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/rev-1", nil)
	reqDel.Header.Set("X-User-Id", "user-1")
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/reviews/rev-1 status = %d, want 200", recDel.Code)
	}

	// POST presign
	reqPresign := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/media/presign", bytes.NewBufferString(`{"filename":"a.png","contentType":"image/png","sizeBytes":1024}`))
	reqPresign.Header.Set("X-User-Id", "user-1")
	recPresign := httptest.NewRecorder()
	mux.ServeHTTP(recPresign, reqPresign)
	if recPresign.Code != http.StatusOK {
		t.Fatalf("POST presign status = %d, want 200", recPresign.Code)
	}
}
