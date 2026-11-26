package http

import (
	"net/http"
	"strconv"
	"strings"

	"ecommerce.local/bff-service/internal/catalog"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
)

type adminReviewsPageData struct {
	Items          []productReviewDTO `json:"items"`
	Page           int32              `json:"page"`
	PageSize       int32              `json:"pageSize"`
	TotalItems     int64              `json:"totalItems"`
	TotalPages     int32              `json:"totalPages"`
	VisibleCount   int64              `json:"visibleCount"`
	HiddenCount    int64              `json:"hiddenCount"`
	UnrepliedCount int64              `json:"unrepliedCount"`
	AverageRating  float64            `json:"averageRating"`
}

type setReviewVisibilityRequest struct {
	Hidden *bool  `json:"hidden"`
	Reason string `json:"reason"`
}

type replyReviewRequest struct {
	Reply string `json:"reply"`
}

// AdminReviewsHandler serves administrative product review moderation routes.
type AdminReviewsHandler struct {
	catalog catalog.Client
}

func NewAdminReviewsHandler(catalogClient catalog.Client) *AdminReviewsHandler {
	return &AdminReviewsHandler{catalog: catalogClient}
}

func (h *AdminReviewsHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/reviews", correlateTrace(RequireCaller(http.HandlerFunc(h.list))))
	mux.Handle("PATCH /api/v1/admin/reviews/{id}/visibility", correlateTrace(RequireCaller(http.HandlerFunc(h.setVisibility))))
	mux.Handle("PUT /api/v1/admin/reviews/{id}/reply", correlateTrace(RequireCaller(http.HandlerFunc(h.reply))))
}

func (h *AdminReviewsHandler) list(w http.ResponseWriter, r *http.Request) {
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

	statusParam := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if statusParam != "" && statusParam != "all" && statusParam != "visible" && statusParam != "hidden" {
		fields["status"] = "must be one of: all, visible, hidden"
	}

	replyStatus := strings.ToLower(strings.TrimSpace(q.Get("replyStatus")))
	if replyStatus != "" && replyStatus != "all" && replyStatus != "replied" && replyStatus != "unreplied" {
		fields["replyStatus"] = "must be one of: all, replied, unreplied"
	}

	page, err := parseBoundedInt(q.Get("page"), 1, 1, 1<<31-1)
	if err != nil {
		fields["page"] = err.Error()
	}
	pageSize, err := parseBoundedInt(q.Get("pageSize"), 20, 1, 100)
	if err != nil {
		fields["pageSize"] = err.Error()
	}
	if len(fields) > 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid query parameters.", fields)
		return
	}

	resp, err := h.catalog.ListAdminReviews(r.Context(), &catalogv1.ListAdminReviewsRequest{
		UserId:      callerID(r),
		Query:       strings.TrimSpace(q.Get("q")),
		ProductId:   strings.TrimSpace(q.Get("productId")),
		Rating:      rating,
		Status:      statusParam,
		ReplyStatus: replyStatus,
		Page:        int32(page),
		PageSize:    int32(pageSize),
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	items := make([]productReviewDTO, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toProductReviewDTO(item))
	}
	writeEnvelope(w, http.StatusOK, adminReviewsPageData{
		Items:          items,
		Page:           resp.GetPage(),
		PageSize:       resp.GetPageSize(),
		TotalItems:     resp.GetTotalItems(),
		TotalPages:     resp.GetTotalPages(),
		VisibleCount:   resp.GetVisibleCount(),
		HiddenCount:    resp.GetHiddenCount(),
		UnrepliedCount: resp.GetUnrepliedCount(),
		AverageRating:  resp.GetAverageRating(),
	}, "Admin reviews retrieved.")
}

func (h *AdminReviewsHandler) setVisibility(w http.ResponseWriter, r *http.Request) {
	reviewID := strings.TrimSpace(r.PathValue("id"))
	if reviewID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Review id is required.", map[string]string{"id": "is required"})
		return
	}

	var body setReviewVisibilityRequest
	if !decodeReviewJSONBody(w, r, &body) {
		return
	}
	if body.Hidden == nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "hidden flag is required.", map[string]string{"hidden": "is required"})
		return
	}

	resp, err := h.catalog.SetReviewVisibility(r.Context(), &catalogv1.SetReviewVisibilityRequest{
		UserId:   callerID(r),
		ReviewId: reviewID,
		Hidden:   *body.Hidden,
		Reason:   body.Reason,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	msg := "Review restored."
	if *body.Hidden {
		msg = "Review hidden."
	}
	writeEnvelope(w, http.StatusOK, toProductReviewDTO(resp.GetReview()), msg)
}

func (h *AdminReviewsHandler) reply(w http.ResponseWriter, r *http.Request) {
	reviewID := strings.TrimSpace(r.PathValue("id"))
	if reviewID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Review id is required.", map[string]string{"id": "is required"})
		return
	}

	var body replyReviewRequest
	if !decodeReviewJSONBody(w, r, &body) {
		return
	}

	resp, err := h.catalog.ReplyProductReview(r.Context(), &catalogv1.ReplyProductReviewRequest{
		UserId:   callerID(r),
		ReviewId: reviewID,
		Reply:    body.Reply,
	})
	if err != nil {
		writeReviewError(w, r, err)
		return
	}

	msg := "Review reply saved."
	if strings.TrimSpace(body.Reply) == "" {
		msg = "Review reply cleared."
	}
	writeEnvelope(w, http.StatusOK, toProductReviewDTO(resp.GetReview()), msg)
}
