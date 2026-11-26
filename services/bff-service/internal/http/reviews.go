package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ecommerce.local/bff-service/internal/catalog"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type productReviewDTO struct {
	ID               string   `json:"id"`
	ProductID        string   `json:"productId"`
	ProductName      string   `json:"productName"`
	UserID           string   `json:"userId"`
	OrderID          string   `json:"orderId"`
	SkuID            string   `json:"skuId"`
	VariantLabel     string   `json:"variantLabel"`
	AuthorName       string   `json:"authorName"`
	Rating           int32    `json:"rating"`
	Comment          string   `json:"comment"`
	Images           []string `json:"images"`
	VerifiedPurchase bool     `json:"verifiedPurchase"`
	Hidden           bool     `json:"hidden"`
	HiddenReason     string   `json:"hiddenReason,omitempty"`
	AdminReply       string   `json:"adminReply,omitempty"`
	AdminRepliedAt   string   `json:"adminRepliedAt,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type ratingBreakdownDTO struct {
	Rating int32 `json:"rating"`
	Count  int64 `json:"count"`
}

type reviewSummaryDTO struct {
	AverageRating float64              `json:"averageRating"`
	TotalReviews  int64                `json:"totalReviews"`
	Breakdown     []ratingBreakdownDTO `json:"breakdown"`
}

type productReviewsPageData struct {
	Items      []productReviewDTO `json:"items"`
	Page       int32              `json:"page"`
	PageSize   int32              `json:"pageSize"`
	TotalItems int64              `json:"totalItems"`
	TotalPages int32              `json:"totalPages"`
	Summary    reviewSummaryDTO   `json:"summary"`
}

type eligibleReviewOrderDTO struct {
	OrderID      string `json:"orderId"`
	OrderCode    string `json:"orderCode"`
	SkuID        string `json:"skuId"`
	VariantLabel string `json:"variantLabel"`
	DeliveredAt  string `json:"deliveredAt"`
	Reviewed     bool   `json:"reviewed"`
	ReviewID     string `json:"reviewId,omitempty"`
}

type reviewEligibilityData struct {
	CanReview      bool                     `json:"canReview"`
	EligibleOrders []eligibleReviewOrderDTO `json:"eligibleOrders"`
	MyReviews      []productReviewDTO       `json:"myReviews"`
}

type createReviewRequest struct {
	ProductID  string   `json:"productId"`
	OrderID    string   `json:"orderId"`
	AuthorName string   `json:"authorName"`
	Rating     int32    `json:"rating"`
	Comment    string   `json:"comment"`
	Images     []string `json:"images"`
}

type updateReviewRequest struct {
	Rating  int32    `json:"rating"`
	Comment string   `json:"comment"`
	Images  []string `json:"images"`
}

type reviewMutationData struct {
	Review  productReviewDTO `json:"review"`
	Summary reviewSummaryDTO `json:"summary"`
}

type deleteReviewData struct {
	ReviewID  string           `json:"reviewId"`
	ProductID string           `json:"productId"`
	Summary   reviewSummaryDTO `json:"summary"`
}

// ReviewsHandler serves storefront public review reads and authenticated customer review endpoints.
type ReviewsHandler struct {
	catalog catalog.Client
}

func NewReviewsHandler(catalogClient catalog.Client) *ReviewsHandler {
	return &ReviewsHandler{catalog: catalogClient}
}

func (h *ReviewsHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/storefront/products/{id}/reviews", correlateTrace(http.HandlerFunc(h.listProductReviews)))
	mux.Handle("GET /api/v1/reviews/eligibility", correlateTrace(RequireCaller(http.HandlerFunc(h.getEligibility))))
	mux.Handle("POST /api/v1/reviews", correlateTrace(RequireCaller(http.HandlerFunc(h.createReview))))
	mux.Handle("PUT /api/v1/reviews/{id}", correlateTrace(RequireCaller(http.HandlerFunc(h.updateReview))))
	mux.Handle("DELETE /api/v1/reviews/{id}", correlateTrace(RequireCaller(http.HandlerFunc(h.deleteReview))))
	mux.Handle("POST /api/v1/reviews/media/presign", correlateTrace(RequireCaller(http.HandlerFunc(h.presignReviewMedia))))
}

func (h *ReviewsHandler) listProductReviews(w http.ResponseWriter, r *http.Request) {
	productID := strings.TrimSpace(r.PathValue("id"))
	if productID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Product id is required.", map[string]string{"id": "is required"})
		return
	}

	q := r.URL.Query()
	fields := map[string]string{}

	var rating int32
	if raw := strings.TrimSpace(q.Get("rating")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 5 {
			fields["rating"] = "must be an integer between 1 and 5"
		} else {
			rating = int32(n)
		}
	}

	hasImages := false
	if raw := strings.TrimSpace(q.Get("hasImages")); raw != "" {
		switch strings.ToLower(raw) {
		case "true", "1":
			hasImages = true
		case "false", "0":
			hasImages = false
		default:
			fields["hasImages"] = "must be true or false"
		}
	}

	sortKey := strings.TrimSpace(q.Get("sort"))
	if sortKey != "" && sortKey != "newest" && sortKey != "highest" && sortKey != "lowest" {
		fields["sort"] = "must be one of: newest, highest, lowest"
	}

	page, err := parseBoundedInt(q.Get("page"), 1, 1, 1<<31-1)
	if err != nil {
		fields["page"] = err.Error()
	}
	pageSize, err := parseBoundedInt(q.Get("pageSize"), 10, 1, 50)
	if err != nil {
		fields["pageSize"] = err.Error()
	}
	if len(fields) > 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid query parameters.", fields)
		return
	}

	resp, err := h.catalog.ListProductReviews(r.Context(), &catalogv1.ListProductReviewsRequest{
		ProductId: productID,
		Rating:    rating,
		HasImages: hasImages,
		Sort:      sortKey,
		Page:      int32(page),
		PageSize:  int32(pageSize),
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	items := make([]productReviewDTO, 0, len(resp.GetItems()))
	for _, rev := range resp.GetItems() {
		items = append(items, toProductReviewDTO(rev))
	}
	writeEnvelope(w, http.StatusOK, productReviewsPageData{
		Items:      items,
		Page:       resp.GetPage(),
		PageSize:   resp.GetPageSize(),
		TotalItems: resp.GetTotalItems(),
		TotalPages: resp.GetTotalPages(),
		Summary:    toReviewSummaryDTO(resp.GetSummary()),
	}, "Product reviews retrieved.")
}

func (h *ReviewsHandler) getEligibility(w http.ResponseWriter, r *http.Request) {
	productID := strings.TrimSpace(r.URL.Query().Get("productId"))
	if productID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "productId is required.", map[string]string{"productId": "is required"})
		return
	}

	resp, err := h.catalog.GetReviewEligibility(r.Context(), &catalogv1.GetReviewEligibilityRequest{
		UserId:    callerID(r),
		ProductId: productID,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	orders := make([]eligibleReviewOrderDTO, 0, len(resp.GetEligibleOrders()))
	for _, ord := range resp.GetEligibleOrders() {
		orders = append(orders, eligibleReviewOrderDTO{
			OrderID:      ord.GetOrderId(),
			OrderCode:    ord.GetOrderCode(),
			SkuID:        ord.GetSkuId(),
			VariantLabel: ord.GetVariantLabel(),
			DeliveredAt:  ord.GetDeliveredAt(),
			Reviewed:     ord.GetReviewed(),
			ReviewID:     ord.GetReviewId(),
		})
	}
	myReviews := make([]productReviewDTO, 0, len(resp.GetMyReviews()))
	for _, rev := range resp.GetMyReviews() {
		myReviews = append(myReviews, toProductReviewDTO(rev))
	}

	writeEnvelope(w, http.StatusOK, reviewEligibilityData{
		CanReview:      resp.GetCanReview(),
		EligibleOrders: orders,
		MyReviews:      myReviews,
	}, "Review eligibility retrieved.")
}

func (h *ReviewsHandler) createReview(w http.ResponseWriter, r *http.Request) {
	var body createReviewRequest
	if !decodeReviewJSONBody(w, r, &body) {
		return
	}

	resp, err := h.catalog.CreateProductReview(r.Context(), &catalogv1.CreateProductReviewRequest{
		UserId:     callerID(r),
		ProductId:  strings.TrimSpace(body.ProductID),
		OrderId:    strings.TrimSpace(body.OrderID),
		AuthorName: strings.TrimSpace(body.AuthorName),
		Rating:     body.Rating,
		Comment:    body.Comment,
		Images:     body.Images,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	writeEnvelope(w, http.StatusCreated, reviewMutationData{
		Review:  toProductReviewDTO(resp.GetReview()),
		Summary: toReviewSummaryDTO(resp.GetSummary()),
	}, "Review submitted.")
}

func (h *ReviewsHandler) updateReview(w http.ResponseWriter, r *http.Request) {
	reviewID := strings.TrimSpace(r.PathValue("id"))
	if reviewID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Review id is required.", map[string]string{"id": "is required"})
		return
	}

	var body updateReviewRequest
	if !decodeReviewJSONBody(w, r, &body) {
		return
	}

	resp, err := h.catalog.UpdateProductReview(r.Context(), &catalogv1.UpdateProductReviewRequest{
		UserId:   callerID(r),
		ReviewId: reviewID,
		Rating:   body.Rating,
		Comment:  body.Comment,
		Images:   body.Images,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	writeEnvelope(w, http.StatusOK, reviewMutationData{
		Review:  toProductReviewDTO(resp.GetReview()),
		Summary: toReviewSummaryDTO(resp.GetSummary()),
	}, "Review updated.")
}

func (h *ReviewsHandler) deleteReview(w http.ResponseWriter, r *http.Request) {
	reviewID := strings.TrimSpace(r.PathValue("id"))
	if reviewID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Review id is required.", map[string]string{"id": "is required"})
		return
	}

	resp, err := h.catalog.DeleteProductReview(r.Context(), &catalogv1.DeleteProductReviewRequest{
		UserId:   callerID(r),
		ReviewId: reviewID,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	writeEnvelope(w, http.StatusOK, deleteReviewData{
		ReviewID:  resp.GetReviewId(),
		ProductID: resp.GetProductId(),
		Summary:   toReviewSummaryDTO(resp.GetSummary()),
	}, "Review deleted.")
}

func (h *ReviewsHandler) presignReviewMedia(w http.ResponseWriter, r *http.Request) {
	var body mediaPresignRequest
	if !decodeReviewJSONBody(w, r, &body) {
		return
	}

	resp, err := h.catalog.CreateReviewMediaUpload(r.Context(), &catalogv1.CreateReviewMediaUploadRequest{
		UserId:      callerID(r),
		Filename:    body.Filename,
		ContentType: body.ContentType,
		SizeBytes:   body.SizeBytes,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	writeEnvelope(w, http.StatusOK, mediaPresignData{
		UploadID:         resp.GetUploadId(),
		MediaType:        resp.GetMediaType(),
		UploadURL:        resp.GetUploadUrl(),
		PublicURL:        resp.GetPublicUrl(),
		ObjectKey:        resp.GetObjectKey(),
		ExpiresInSeconds: resp.GetExpiresInSeconds(),
	}, "Review media upload URL created.")
}

func decodeReviewJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := decodeJSON(r, dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Request body must be valid JSON.", nil)
		return false
	}
	return true
}

func toProductReviewDTO(r *catalogv1.ProductReview) productReviewDTO {
	if r == nil {
		return productReviewDTO{Images: []string{}}
	}
	return productReviewDTO{
		ID:               r.GetId(),
		ProductID:        r.GetProductId(),
		ProductName:      r.GetProductName(),
		UserID:           r.GetUserId(),
		OrderID:          r.GetOrderId(),
		SkuID:            r.GetSkuId(),
		VariantLabel:     r.GetVariantLabel(),
		AuthorName:       r.GetAuthorName(),
		Rating:           r.GetRating(),
		Comment:          r.GetComment(),
		Images:           stringsOrEmpty(r.GetImages()),
		VerifiedPurchase: r.GetVerifiedPurchase(),
		Hidden:           r.GetHidden(),
		HiddenReason:     r.GetHiddenReason(),
		AdminReply:       r.GetAdminReply(),
		AdminRepliedAt:   r.GetAdminRepliedAt(),
		CreatedAt:        r.GetCreatedAt(),
		UpdatedAt:        r.GetUpdatedAt(),
	}
}

func toReviewSummaryDTO(s *catalogv1.ReviewRatingSummary) reviewSummaryDTO {
	if s == nil {
		return reviewSummaryDTO{Breakdown: []ratingBreakdownDTO{}}
	}
	breakdown := make([]ratingBreakdownDTO, 0, len(s.GetBreakdown()))
	for _, b := range s.GetBreakdown() {
		breakdown = append(breakdown, ratingBreakdownDTO{
			Rating: b.GetRating(),
			Count:  b.GetCount(),
		})
	}
	return reviewSummaryDTO{
		AverageRating: s.GetAverageRating(),
		TotalReviews:  s.GetTotalReviews(),
		Breakdown:     breakdown,
	}
}

func writeReviewError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, catalog.ErrProductNotFound):
		writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
	case errors.Is(err, catalog.ErrReviewNotFound):
		writeError(w, r, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found.", nil)
	case errors.Is(err, catalog.ErrReviewNotEligible):
		writeError(w, r, http.StatusConflict, "REVIEW_NOT_ELIGIBLE", "Only customers with delivered orders can review this product.", nil)
	case errors.Is(err, catalog.ErrReviewAlreadyExists):
		writeError(w, r, http.StatusConflict, "REVIEW_ALREADY_EXISTS", "You have already reviewed this product for this order.", nil)
	case errors.Is(err, catalog.ErrReviewForbidden):
		writeError(w, r, http.StatusForbidden, "REVIEW_FORBIDDEN", "You can only modify your own review.", nil)
	case errors.Is(err, catalog.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "REVIEW_PERMISSION_DENIED", "You do not have permission to perform this action.", nil)
	case errors.Is(err, catalog.ErrMediaAlreadyAttached):
		writeError(w, r, http.StatusConflict, "MEDIA_ALREADY_ATTACHED", "One or more uploaded images are already attached.", nil)
	case errors.Is(err, catalog.ErrMediaStorageUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "MEDIA_STORAGE_UNAVAILABLE", "Media storage is temporarily unavailable.", nil)
	case errors.Is(err, catalog.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", status.Convert(err).Message(), nil)
	default:
		writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
	}
}
