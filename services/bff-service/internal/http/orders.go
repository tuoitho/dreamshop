package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	orderclient "ecommerce.local/bff-service/internal/order"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/grpc/codes"
)

type shippingAddressJSON struct {
	RecipientName string `json:"recipientName"`
	Phone         string `json:"phone"`
	ProvinceCode  string `json:"provinceCode"`
	ProvinceName  string `json:"provinceName"`
	WardCode      string `json:"wardCode"`
	WardName      string `json:"wardName"`
	AddressLine   string `json:"addressLine"`
	Label         string `json:"label,omitempty"`
}

type orderItemJSON struct {
	CartItemID             string   `json:"cartItemId,omitempty"`
	ProductID              string   `json:"productId"`
	SkuID                  string   `json:"skuId"`
	Quantity               int32    `json:"quantity"`
	UnitPrice              int64    `json:"unitPrice"`
	LineTotal              int64    `json:"lineTotal"`
	ExpectedName           string   `json:"expectedName"`
	Brand                  string   `json:"brand"`
	CategoryName           string   `json:"categoryName"`
	VariantLabel           string   `json:"variantLabel"`
	VariantColorHex        string   `json:"variantColorHex,omitempty"`
	ImageURL               string   `json:"imageUrl"`
	ImageEmoji             string   `json:"imageEmoji"`
	ImageHue               []uint32 `json:"imageHue"`
	ExpectedProductVersion int64    `json:"expectedProductVersion"`
}

type orderDetailJSON struct {
	OrderID          string              `json:"orderId"`
	OrderCode        string              `json:"orderCode"`
	UserID           string              `json:"userId"`
	Status           string              `json:"status"`
	PaymentMethod    string              `json:"paymentMethod"`
	PaymentStatus    string              `json:"paymentStatus"`
	ShippingAddress  shippingAddressJSON `json:"shippingAddress"`
	Items            []orderItemJSON     `json:"items"`
	Subtotal         int64               `json:"subtotal"`
	ShippingFee      int64               `json:"shippingFee"`
	TotalAmount      int64               `json:"totalAmount"`
	Currency         string              `json:"currency"`
	Note             string              `json:"note,omitempty"`
	CancelReason     string              `json:"cancelReason,omitempty"`
	PayURL           string              `json:"payUrl,omitempty"`
	PaymentExpiresAt string              `json:"paymentExpiresAt,omitempty"`
	CreatedAt        string              `json:"createdAt"`
	UpdatedAt        string              `json:"updatedAt"`
	AutoConfirmAt    string              `json:"autoConfirmAt,omitempty"`
	ShippingCarrier  string              `json:"shippingCarrier,omitempty"`
	TrackingNumber   string              `json:"trackingNumber,omitempty"`
	// Filled only by single-order reads; list responses return an empty array.
	StatusHistory []orderStatusChangeJSON `json:"statusHistory"`
}

type orderStatusChangeJSON struct {
	FromStatus      string `json:"fromStatus,omitempty"`
	ToStatus        string `json:"toStatus"`
	ActorType       string `json:"actorType"`
	Reason          string `json:"reason,omitempty"`
	ShippingCarrier string `json:"shippingCarrier,omitempty"`
	TrackingNumber  string `json:"trackingNumber,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

type createOrderDataJSON struct {
	Order  orderDetailJSON `json:"order"`
	PayURL string          `json:"payUrl,omitempty"`
}

type orderListDataJSON struct {
	Items      []orderDetailJSON `json:"items"`
	Page       int32             `json:"page"`
	PageSize   int32             `json:"pageSize"`
	TotalItems int64             `json:"totalItems"`
	TotalPages int32             `json:"totalPages"`
}

type createOrderItemInput struct {
	CartItemID             string   `json:"cartItemId"`
	ProductID              string   `json:"productId"`
	SkuID                  string   `json:"skuId"`
	Quantity               int32    `json:"quantity"`
	UnitPrice              int64    `json:"unitPrice"`
	ExpectedName           string   `json:"expectedName"`
	Brand                  string   `json:"brand"`
	CategoryName           string   `json:"categoryName"`
	VariantLabel           string   `json:"variantLabel"`
	VariantColorHex        string   `json:"variantColorHex"`
	ImageURL               string   `json:"imageUrl"`
	ImageEmoji             string   `json:"imageEmoji"`
	ImageHue               []uint32 `json:"imageHue"`
	ExpectedProductVersion int64    `json:"expectedProductVersion"`
}

type createOrderInput struct {
	PaymentMethod   string               `json:"paymentMethod"`
	Note            string               `json:"note"`
	ShippingAddress shippingAddressJSON  `json:"shippingAddress"`
	Items           []createOrderItemInput `json:"items"`
}

type cancelOrderInput struct {
	Reason string `json:"reason"`
}

type OrdersHandler struct {
	orders orderclient.Client
}

func NewOrdersHandler(client orderclient.Client) *OrdersHandler {
	return &OrdersHandler{orders: client}
}

func (h *OrdersHandler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/orders", correlateTrace(RequireCaller(http.HandlerFunc(h.create))))
	mux.Handle("GET /api/v1/orders", correlateTrace(RequireCaller(http.HandlerFunc(h.list))))
	mux.Handle("GET /api/v1/orders/{orderId}", correlateTrace(RequireCaller(http.HandlerFunc(h.get))))
	mux.Handle("POST /api/v1/orders/{orderId}/cancel", correlateTrace(RequireCaller(http.HandlerFunc(h.cancel))))
}

func (h *OrdersHandler) create(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Idempotency-Key header is required.", nil)
		return
	}

	var input createOrderInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid order request body.", nil)
		return
	}
	if len(input.Items) == 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Order must contain at least one item.", nil)
		return
	}

	protoItems := make([]*orderv1.OrderItem, 0, len(input.Items))
	for _, it := range input.Items {
		productID := strings.TrimSpace(it.ProductID)
		skuID := strings.TrimSpace(it.SkuID)
		if productID == "" || skuID == "" || it.Quantity <= 0 || it.UnitPrice < 0 {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid order item parameters.", nil)
			return
		}
		protoItems = append(protoItems, &orderv1.OrderItem{
			CartItemId:             strings.TrimSpace(it.CartItemID),
			ProductId:              productID,
			SkuId:                  skuID,
			Quantity:               it.Quantity,
			UnitPrice:              &commonv1.Money{Currency: "VND", AmountMinor: it.UnitPrice},
			ExpectedName:           strings.TrimSpace(it.ExpectedName),
			Brand:                  strings.TrimSpace(it.Brand),
			CategoryName:           strings.TrimSpace(it.CategoryName),
			VariantLabel:           strings.TrimSpace(it.VariantLabel),
			VariantColorHex:        strings.TrimSpace(it.VariantColorHex),
			ImageUrl:               strings.TrimSpace(it.ImageURL),
			ImageEmoji:             strings.TrimSpace(it.ImageEmoji),
			ImageHue:               it.ImageHue,
			ExpectedProductVersion: it.ExpectedProductVersion,
		})
	}

	resp, err := h.orders.CreateOrder(r.Context(), &orderv1.CreateOrderRequest{
		UserId:         callerID(r),
		IdempotencyKey: idempotencyKey,
		PaymentMethod:  strings.TrimSpace(input.PaymentMethod),
		Note:           strings.TrimSpace(input.Note),
		ShippingAddress: &orderv1.ShippingAddressSnapshot{
			RecipientName: strings.TrimSpace(input.ShippingAddress.RecipientName),
			Phone:         strings.TrimSpace(input.ShippingAddress.Phone),
			ProvinceCode:  strings.TrimSpace(input.ShippingAddress.ProvinceCode),
			ProvinceName:  strings.TrimSpace(input.ShippingAddress.ProvinceName),
			WardCode:      strings.TrimSpace(input.ShippingAddress.WardCode),
			WardName:      strings.TrimSpace(input.ShippingAddress.WardName),
			AddressLine:   strings.TrimSpace(input.ShippingAddress.AddressLine),
			Label:         strings.TrimSpace(input.ShippingAddress.Label),
		},
		Items: protoItems,
	})
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}

	orderJSON := newOrderDetailJSON(resp.GetOrder())
	payURL := resp.GetPayUrl()
	if payURL == "" {
		payURL = orderJSON.PayURL
	}
	writeEnvelope(w, http.StatusCreated, createOrderDataJSON{
		Order:  orderJSON,
		PayURL: payURL,
	}, "Đặt hàng thành công")
}

func (h *OrdersHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := int32(1)
	if raw := strings.TrimSpace(q.Get("page")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid page parameter.", nil)
			return
		}
		page = int32(parsed)
	}
	pageSize := int32(10)
	if raw := strings.TrimSpace(q.Get("pageSize")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 || parsed > 50 {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid pageSize parameter.", nil)
			return
		}
		pageSize = int32(parsed)
	}
	statusFilter := strings.TrimSpace(q.Get("status"))

	resp, err := h.orders.ListOrders(r.Context(), callerID(r), statusFilter, page, pageSize)
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}

	items := make([]orderDetailJSON, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		items = append(items, newOrderDetailJSON(it))
	}
	writeEnvelope(w, http.StatusOK, orderListDataJSON{
		Items:      items,
		Page:       resp.GetPage(),
		PageSize:   resp.GetPageSize(),
		TotalItems: resp.GetTotalItems(),
		TotalPages: resp.GetTotalPages(),
	}, "Orders retrieved.")
}

func (h *OrdersHandler) get(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("orderId"))
	if orderID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Order ID is required.", nil)
		return
	}
	resp, err := h.orders.GetOrder(r.Context(), callerID(r), orderID)
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newOrderDetailJSON(resp.GetOrder()), "Order retrieved.")
}

func (h *OrdersHandler) cancel(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("orderId"))
	if orderID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Order ID is required.", nil)
		return
	}

	var input cancelOrderInput
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid cancel request body.", nil)
			return
		}
	}

	resp, err := h.orders.CancelOrder(r.Context(), callerID(r), orderID, strings.TrimSpace(input.Reason))
	if err != nil {
		writeOrderServiceError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newOrderDetailJSON(resp.GetOrder()), "Đã hủy đơn hàng.")
}

func writeOrderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var svcErr *orderclient.ServiceError
	if errors.As(err, &svcErr) {
		msg := svcErr.Message
		if msg == "" {
			msg = "Không thể xử lý yêu cầu đơn hàng."
		}
		switch svcErr.Reason {
		case "INVALID_PARAMETER":
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", msg, svcErr.Metadata)
			return
		case "ORDER_REASON_REQUIRED":
			writeError(w, r, http.StatusBadRequest, svcErr.Reason, msg, nil)
			return
		case "ORDER_PERMISSION_DENIED":
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", msg, nil)
			return
		case "ORDER_NOT_FOUND":
			writeError(w, r, http.StatusNotFound, "ORDER_NOT_FOUND", msg, nil)
			return
		case "PRODUCT_SNAPSHOT_MISMATCH",
			"INVENTORY_INSUFFICIENT",
			"ORDER_NOT_CANCELLABLE",
			"ORDER_INVALID_TRANSITION",
			"ORDER_STATUS_CONFLICT",
			"PAYMENT_GATEWAY_REJECTED",
			"REQUEST_IN_PROGRESS",
			"IDEMPOTENCY_KEY_REUSED":
			writeError(w, r, http.StatusConflict, svcErr.Reason, msg, svcErr.Metadata)
			return
		case "AUTHORIZATION_UNAVAILABLE":
			writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", msg, nil)
			return
		case "CATALOG_UNAVAILABLE", "INVENTORY_UNAVAILABLE", "PAYMENT_GATEWAY_UNAVAILABLE":
			writeError(w, r, http.StatusServiceUnavailable, svcErr.Reason, msg, nil)
			return
		}

		switch svcErr.GRPCStatus {
		case codes.InvalidArgument:
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", msg, svcErr.Metadata)
		case codes.PermissionDenied:
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", msg, nil)
		case codes.NotFound:
			writeError(w, r, http.StatusNotFound, "ORDER_NOT_FOUND", msg, nil)
		case codes.FailedPrecondition, codes.Aborted, codes.AlreadyExists:
			code := svcErr.Reason
			if code == "" {
				code = "ORDER_CONFLICT"
			}
			writeError(w, r, http.StatusConflict, code, msg, svcErr.Metadata)
		case codes.Unavailable, codes.DeadlineExceeded:
			writeError(w, r, http.StatusServiceUnavailable, "ORDER_SERVICE_UNAVAILABLE", "Dịch vụ đơn hàng tạm thời gián đoạn.", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "ORDER_SERVICE_UNAVAILABLE", "Dịch vụ đơn hàng tạm thời gián đoạn.", nil)
		}
		return
	}

	writeError(w, r, http.StatusBadGateway, "ORDER_SERVICE_UNAVAILABLE", "Dịch vụ đơn hàng tạm thời gián đoạn.", nil)
}

func newOrderDetailJSON(detail *orderv1.OrderDetail) orderDetailJSON {
	if detail == nil {
		return orderDetailJSON{Items: []orderItemJSON{}, Currency: "VND", StatusHistory: []orderStatusChangeJSON{}}
	}
	items := make([]orderItemJSON, 0, len(detail.GetItems()))
	for _, it := range detail.GetItems() {
		items = append(items, orderItemJSON{
			CartItemID:             it.GetCartItemId(),
			ProductID:              it.GetProductId(),
			SkuID:                  it.GetSkuId(),
			Quantity:               it.GetQuantity(),
			UnitPrice:              amountMinor(it.GetUnitPrice()),
			LineTotal:              amountMinor(it.GetLineTotal()),
			ExpectedName:           it.GetExpectedName(),
			Brand:                  it.GetBrand(),
			CategoryName:           it.GetCategoryName(),
			VariantLabel:           it.GetVariantLabel(),
			VariantColorHex:        it.GetVariantColorHex(),
			ImageURL:               it.GetImageUrl(),
			ImageEmoji:             it.GetImageEmoji(),
			ImageHue:               uintsOrEmpty(it.GetImageHue()),
			ExpectedProductVersion: it.GetExpectedProductVersion(),
		})
	}
	currency := detail.GetTotalAmount().GetCurrency()
	if currency == "" {
		currency = "VND"
	}
	history := make([]orderStatusChangeJSON, 0, len(detail.GetStatusHistory()))
	for _, change := range detail.GetStatusHistory() {
		history = append(history, orderStatusChangeJSON{
			FromStatus:      change.GetFromStatus(),
			ToStatus:        change.GetToStatus(),
			ActorType:       change.GetActorType(),
			Reason:          change.GetReason(),
			ShippingCarrier: change.GetShippingCarrier(),
			TrackingNumber:  change.GetTrackingNumber(),
			CreatedAt:       change.GetCreatedAt(),
		})
	}
	addr := detail.GetShippingAddress()
	return orderDetailJSON{
		OrderID:       detail.GetOrderId(),
		OrderCode:     detail.GetOrderCode(),
		UserID:        detail.GetUserId(),
		Status:        detail.GetStatus(),
		PaymentMethod: detail.GetPaymentMethod(),
		PaymentStatus: detail.GetPaymentStatus(),
		ShippingAddress: shippingAddressJSON{
			RecipientName: addr.GetRecipientName(),
			Phone:         addr.GetPhone(),
			ProvinceCode:  addr.GetProvinceCode(),
			ProvinceName:  addr.GetProvinceName(),
			WardCode:      addr.GetWardCode(),
			WardName:      addr.GetWardName(),
			AddressLine:   addr.GetAddressLine(),
			Label:         addr.GetLabel(),
		},
		Items:            items,
		Subtotal:         amountMinor(detail.GetSubtotal()),
		ShippingFee:      amountMinor(detail.GetShippingFee()),
		TotalAmount:      amountMinor(detail.GetTotalAmount()),
		Currency:         currency,
		Note:             detail.GetNote(),
		CancelReason:     detail.GetCancelReason(),
		PayURL:           detail.GetPayUrl(),
		PaymentExpiresAt: detail.GetPaymentExpiresAt(),
		CreatedAt:        detail.GetCreatedAt(),
		UpdatedAt:        detail.GetUpdatedAt(),
		AutoConfirmAt:    detail.GetAutoConfirmAt(),
		ShippingCarrier:  detail.GetShippingCarrier(),
		TrackingNumber:   detail.GetTrackingNumber(),
		StatusHistory:    history,
	}
}
