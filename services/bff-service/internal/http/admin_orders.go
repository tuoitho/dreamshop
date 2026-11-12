package http

import (
	"net/http"
	"strconv"
	"strings"

	orderclient "ecommerce.local/bff-service/internal/order"
	orderv1 "ecommerce.local/contracts/go/order/v1"
)

type adminOrderListDataJSON struct {
	Items        []orderDetailJSON `json:"items"`
	Page         int32             `json:"page"`
	PageSize     int32             `json:"pageSize"`
	TotalItems   int64             `json:"totalItems"`
	TotalPages   int32             `json:"totalPages"`
	StatusCounts map[string]int64  `json:"statusCounts"`
}

type adminOrderDataJSON struct {
	Order               orderDetailJSON `json:"order"`
	AllowedNextStatuses []string        `json:"allowedNextStatuses"`
}

type adminUpdateOrderStatusInput struct {
	ExpectedStatus  string `json:"expectedStatus"`
	TargetStatus    string `json:"targetStatus"`
	Reason          string `json:"reason"`
	ShippingCarrier string `json:"shippingCarrier"`
	TrackingNumber  string `json:"trackingNumber"`
}

// AdminOrdersHandler adapts the order-service administrator RPCs to REST.
// Order-service authorizes every call with order.admin.view or order.admin.update.
type AdminOrdersHandler struct {
	orders orderclient.Client
}

func NewAdminOrdersHandler(client orderclient.Client) *AdminOrdersHandler {
	return &AdminOrdersHandler{orders: client}
}

func (h *AdminOrdersHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/orders", correlateTrace(RequireCaller(http.HandlerFunc(h.list))))
	mux.Handle("GET /api/v1/admin/orders/{orderId}", correlateTrace(RequireCaller(http.HandlerFunc(h.get))))
	mux.Handle("POST /api/v1/admin/orders/{orderId}/status", correlateTrace(RequireCaller(http.HandlerFunc(h.updateStatus))))
}

func (h *AdminOrdersHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, ok := parsePositiveInt32(q.Get("page"), 1, 0)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid page parameter.", nil)
		return
	}
	pageSize, ok := parsePositiveInt32(q.Get("pageSize"), 20, 100)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid pageSize parameter.", nil)
		return
	}

	resp, err := h.orders.AdminListOrders(
		r.Context(), callerID(r), strings.TrimSpace(q.Get("status")), strings.TrimSpace(q.Get("q")), page, pageSize)
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}

	items := make([]orderDetailJSON, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		items = append(items, newOrderDetailJSON(it))
	}
	counts := resp.GetStatusCounts()
	if counts == nil {
		counts = map[string]int64{}
	}
	writeEnvelope(w, http.StatusOK, adminOrderListDataJSON{
		Items:        items,
		Page:         resp.GetPage(),
		PageSize:     resp.GetPageSize(),
		TotalItems:   resp.GetTotalItems(),
		TotalPages:   resp.GetTotalPages(),
		StatusCounts: counts,
	}, "Orders retrieved.")
}

func (h *AdminOrdersHandler) get(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("orderId"))
	resp, err := h.orders.AdminGetOrder(r.Context(), callerID(r), orderID)
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newAdminOrderDataJSON(resp.GetOrder(), resp.GetAllowedNextStatuses()), "Order retrieved.")
}

func (h *AdminOrdersHandler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var input adminUpdateOrderStatusInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid order status request body.", nil)
		return
	}
	expected := strings.TrimSpace(input.ExpectedStatus)
	target := strings.TrimSpace(input.TargetStatus)
	if expected == "" || target == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "expectedStatus and targetStatus are required.", nil)
		return
	}

	resp, err := h.orders.AdminUpdateOrderStatus(r.Context(), &orderv1.AdminUpdateOrderStatusRequest{
		UserId:          callerID(r),
		OrderId:         strings.TrimSpace(r.PathValue("orderId")),
		ExpectedStatus:  expected,
		TargetStatus:    target,
		Reason:          strings.TrimSpace(input.Reason),
		ShippingCarrier: strings.TrimSpace(input.ShippingCarrier),
		TrackingNumber:  strings.TrimSpace(input.TrackingNumber),
	})
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newAdminOrderDataJSON(resp.GetOrder(), resp.GetAllowedNextStatuses()),
		"Cập nhật trạng thái đơn hàng thành công")
}

func newAdminOrderDataJSON(detail *orderv1.OrderDetail, allowed []string) adminOrderDataJSON {
	if allowed == nil {
		allowed = []string{}
	}
	return adminOrderDataJSON{Order: newOrderDetailJSON(detail), AllowedNextStatuses: allowed}
}

// parsePositiveInt32 returns fallback for an empty value and rejects values that are not
// positive or exceed max (when max > 0).
func parsePositiveInt32(raw string, fallback, max int32) (int32, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed <= 0 || (max > 0 && parsed > int64(max)) {
		return 0, false
	}
	return int32(parsed), true
}
