package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orderclient "ecommerce.local/bff-service/internal/order"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/grpc/codes"
)

type fakeOrderClient struct {
	createFn      func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
	getFn         func(context.Context, string, string) (*orderv1.GetOrderResponse, error)
	listFn        func(context.Context, string, string, int32, int32) (*orderv1.ListOrdersResponse, error)
	cancelFn      func(context.Context, string, string, string) (*orderv1.CancelOrderResponse, error)
	adminListFn   func(context.Context, string, string, string, int32, int32) (*orderv1.AdminListOrdersResponse, error)
	adminGetFn         func(context.Context, string, string) (*orderv1.AdminGetOrderResponse, error)
	adminUpdateFn      func(context.Context, *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error)
	adminRetryRefundFn func(context.Context, *orderv1.AdminRetryRefundRequest) (*orderv1.AdminRetryRefundResponse, error)
}

func (f *fakeOrderClient) AdminListOrders(ctx context.Context, userID, statusFilter, query string, page, pageSize int32) (*orderv1.AdminListOrdersResponse, error) {
	return f.adminListFn(ctx, userID, statusFilter, query, page, pageSize)
}
func (f *fakeOrderClient) AdminGetOrder(ctx context.Context, userID, orderID string) (*orderv1.AdminGetOrderResponse, error) {
	return f.adminGetFn(ctx, userID, orderID)
}
func (f *fakeOrderClient) AdminUpdateOrderStatus(ctx context.Context, req *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error) {
	return f.adminUpdateFn(ctx, req)
}
func (f *fakeOrderClient) AdminRetryRefund(ctx context.Context, req *orderv1.AdminRetryRefundRequest) (*orderv1.AdminRetryRefundResponse, error) {
	return f.adminRetryRefundFn(ctx, req)
}

func (f *fakeOrderClient) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	return f.createFn(ctx, req)
}
func (f *fakeOrderClient) GetOrder(ctx context.Context, userID, orderID string) (*orderv1.GetOrderResponse, error) {
	return f.getFn(ctx, userID, orderID)
}
func (f *fakeOrderClient) ListOrders(ctx context.Context, userID, statusFilter string, page, pageSize int32) (*orderv1.ListOrdersResponse, error) {
	return f.listFn(ctx, userID, statusFilter, page, pageSize)
}
func (f *fakeOrderClient) CancelOrder(ctx context.Context, userID, orderID, reason string) (*orderv1.CancelOrderResponse, error) {
	return f.cancelFn(ctx, userID, orderID, reason)
}

func orderDetailFixture() *orderv1.OrderDetail {
	return &orderv1.OrderDetail{
		OrderId:       "ord-123",
		OrderCode:     "ORD-20261105-ABC123",
		UserId:        "user-1",
		Status:        "PENDING_PAYMENT",
		PaymentMethod: "MOMO",
		PaymentStatus: "PENDING",
		ShippingAddress: &orderv1.ShippingAddressSnapshot{
			RecipientName: "Nguyễn Văn A",
			Phone:         "0901234567",
			ProvinceCode:  "79",
			ProvinceName:  "TP. Hồ Chí Minh",
			WardCode:      "26740",
			WardName:      "Phường Bến Nghé",
			AddressLine:   "1 Lê Duẩn",
			Label:         "Nhà riêng",
		},
		Items: []*orderv1.OrderItem{
			{
				CartItemId:             "ci-1",
				ProductId:              "dt-1",
				SkuId:                  "dt-1-v1",
				Quantity:               2,
				ExpectedName:           "Nova X15",
				Brand:                  "Nova",
				CategoryName:           "Điện thoại",
				VariantLabel:           "Đen 256GB",
				VariantColorHex:        "#111111",
				ImageUrl:               "https://cdn.example/dt-1.png",
				ImageEmoji:             "📱",
				ImageHue:               []uint32{210, 250},
				ExpectedProductVersion: 3,
				UnitPrice:              &commonv1.Money{Currency: "VND", AmountMinor: 150000},
				LineTotal:              &commonv1.Money{Currency: "VND", AmountMinor: 300000},
			},
		},
		Subtotal:         &commonv1.Money{Currency: "VND", AmountMinor: 300000},
		ShippingFee:      &commonv1.Money{Currency: "VND", AmountMinor: 0},
		TotalAmount:      &commonv1.Money{Currency: "VND", AmountMinor: 300000},
		PayUrl:           "https://test-payment.momo.vn/pay/ord-123",
		PaymentExpiresAt: "2026-11-05T10:15:00Z",
		CreatedAt:        "2026-11-05T10:00:00Z",
		UpdatedAt:        "2026-11-05T10:00:00Z",
	}
}

func TestCreateOrderRequiresIdempotencyKeyAndReturns201WithPayUrl(t *testing.T) {
	mux := http.NewServeMux()
	NewOrdersHandler(&fakeOrderClient{
		createFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			if req.GetUserId() != "user-1" || req.GetIdempotencyKey() != "idem-abc-1" || req.GetPaymentMethod() != "MOMO" {
				t.Fatalf("unexpected CreateOrderRequest: %+v", req)
			}
			if len(req.GetItems()) != 1 || req.GetItems()[0].GetExpectedProductVersion() != 3 {
				t.Fatalf("unexpected items: %+v", req.GetItems())
			}
			detail := orderDetailFixture()
			return &orderv1.CreateOrderResponse{
				OrderId:     detail.GetOrderId(),
				Status:      detail.GetStatus(),
				TotalAmount: detail.GetTotalAmount(),
				Order:       detail,
				PayUrl:      detail.GetPayUrl(),
			}, nil
		},
	}).Register(mux)

	payload := `{
		"paymentMethod":"MOMO",
		"note":"Giao giờ hành chính",
		"shippingAddress":{
			"recipientName":"Nguyễn Văn A",
			"phone":"0901234567",
			"provinceCode":"79",
			"provinceName":"TP. Hồ Chí Minh",
			"wardCode":"26740",
			"wardName":"Phường Bến Nghé",
			"addressLine":"1 Lê Duẩn",
			"label":"Nhà riêng"
		},
		"items":[{
			"cartItemId":"ci-1",
			"productId":"dt-1",
			"skuId":"dt-1-v1",
			"quantity":2,
			"unitPrice":150000,
			"expectedName":"Nova X15",
			"variantLabel":"Đen 256GB",
			"expectedProductVersion":3,
			"imageHue":[210,250]
		}]
	}`

	// Missing Idempotency-Key returns 400
	missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(payload))
	missingReq.Header.Set("X-User-Id", "user-1")
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without Idempotency-Key, got %d: %s", missingRec.Code, missingRec.Body.String())
	}

	// Valid request returns 201 Created with order + payUrl
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(payload))
	req.Header.Set("X-User-Id", "user-1")
	req.Header.Set("Idempotency-Key", "idem-abc-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["message"] != "Đặt hàng thành công" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["payUrl"] != "https://test-payment.momo.vn/pay/ord-123" {
		t.Fatalf("expected payUrl, got %v", data["payUrl"])
	}
	orderMap := data["order"].(map[string]any)
	if orderMap["orderCode"] != "ORD-20261105-ABC123" || orderMap["status"] != "PENDING_PAYMENT" {
		t.Fatalf("unexpected order payload: %v", orderMap)
	}
}

func TestCreateOrderMapsSnapshotMismatchAndConflictErrors(t *testing.T) {
	mux := http.NewServeMux()
	NewOrdersHandler(&fakeOrderClient{
		createFn: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			return nil, &orderclient.ServiceError{
				GRPCStatus: codes.FailedPrecondition,
				Reason:     "PRODUCT_SNAPSHOT_MISMATCH",
				Message:    "Giá hoặc thông tin sản phẩm vừa thay đổi",
				Metadata: map[string]string{
					"dt-1-v1": "[PRICE_CHANGED] Giá phân loại vừa thay đổi từ 150000 sang 180000",
				},
			}
		},
	}).Register(mux)

	payload := `{
		"paymentMethod":"COD",
		"shippingAddress":{"recipientName":"A","phone":"0901234567","provinceCode":"79","provinceName":"HCM","wardCode":"26740","wardName":"BN","addressLine":"1 LD"},
		"items":[{"productId":"dt-1","skuId":"dt-1-v1","quantity":1,"unitPrice":150000,"expectedName":"Nova","variantLabel":"Đen","expectedProductVersion":1}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(payload))
	req.Header.Set("X-User-Id", "user-1")
	req.Header.Set("Idempotency-Key", "idem-mismatch-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != "PRODUCT_SNAPSHOT_MISMATCH" {
		t.Fatalf("expected code PRODUCT_SNAPSHOT_MISMATCH, got %v", body["code"])
	}
	errs, ok := body["errors"].(map[string]any)
	if !ok || !strings.Contains(errs["dt-1-v1"].(string), "[PRICE_CHANGED]") {
		t.Fatalf("expected structured per-SKU errors map, got %v", body["errors"])
	}
}

func TestListGetAndCancelOrdersEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	NewOrdersHandler(&fakeOrderClient{
		listFn: func(_ context.Context, userID, statusFilter string, page, pageSize int32) (*orderv1.ListOrdersResponse, error) {
			if userID != "user-1" || statusFilter != "PENDING_PAYMENT" || page != 2 || pageSize != 10 {
				t.Fatalf("unexpected list args: %q %q %d %d", userID, statusFilter, page, pageSize)
			}
			return &orderv1.ListOrdersResponse{
				Items:      []*orderv1.OrderDetail{orderDetailFixture()},
				Page:       2,
				PageSize:   10,
				TotalItems: 11,
				TotalPages: 2,
			}, nil
		},
		getFn: func(_ context.Context, userID, orderID string) (*orderv1.GetOrderResponse, error) {
			if userID != "user-1" || orderID != "ord-123" {
				t.Fatalf("unexpected get args: %q %q", userID, orderID)
			}
			return &orderv1.GetOrderResponse{Order: orderDetailFixture()}, nil
		},
		cancelFn: func(_ context.Context, userID, orderID, reason string) (*orderv1.CancelOrderResponse, error) {
			if userID != "user-1" || orderID != "ord-123" {
				t.Fatalf("unexpected cancel args: %q %q %q", userID, orderID, reason)
			}
			cancelled := orderDetailFixture()
			cancelled.Status = "CANCELLED"
			cancelled.CancelReason = "USER_CANCELLED"
			return &orderv1.CancelOrderResponse{Order: cancelled}, nil
		},
	}).Register(mux)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders?page=2&pageSize=10&status=PENDING_PAYMENT", nil)
	listReq.Header.Set("X-User-Id", "user-1")
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d: %s", listRec.Code, listRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/ord-123", nil)
	getReq.Header.Set("X-User-Id", "user-1")
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d: %s", getRec.Code, getRec.Body.String())
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders/ord-123/cancel", nil)
	cancelReq.Header.Set("X-User-Id", "user-1")
	cancelRec := httptest.NewRecorder()
	mux.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on cancel, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}

func TestOrderDetailJSONIncludesFulfillmentFields(t *testing.T) {
	withoutHistory := newOrderDetailJSON(orderDetailFixture())
	if withoutHistory.StatusHistory == nil || len(withoutHistory.StatusHistory) != 0 {
		t.Fatalf("statusHistory must be an empty array, got %#v", withoutHistory.StatusHistory)
	}

	detail := orderDetailFixture()
	detail.Status = "SHIPPING"
	detail.AutoConfirmAt = "2026-11-12T08:01:00Z"
	detail.ShippingCarrier = "GHN"
	detail.TrackingNumber = "T1"
	detail.StatusHistory = []*orderv1.OrderStatusChange{
		{ToStatus: "PENDING_STOCK", ActorType: "CUSTOMER", CreatedAt: "2026-11-12T08:00:00Z"},
		{FromStatus: "PREPARING", ToStatus: "SHIPPING", ActorType: "ADMIN", ShippingCarrier: "GHN", TrackingNumber: "T1", CreatedAt: "2026-11-12T09:00:00Z"},
	}

	raw, err := json.Marshal(newOrderDetailJSON(detail))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		`"autoConfirmAt":"2026-11-12T08:01:00Z"`,
		`"shippingCarrier":"GHN"`,
		`"trackingNumber":"T1"`,
		`"statusHistory":[{"toStatus":"PENDING_STOCK","actorType":"CUSTOMER","createdAt":"2026-11-12T08:00:00Z"}`,
		`{"fromStatus":"PREPARING","toStatus":"SHIPPING","actorType":"ADMIN","shippingCarrier":"GHN","trackingNumber":"T1","createdAt":"2026-11-12T09:00:00Z"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("order JSON missing %s:\n%s", want, body)
		}
	}
}

func TestCustomerOrderEndpointsOmitInternalRefundFields(t *testing.T) {
	detailWithInternalRefund := func() *orderv1.OrderDetail {
		d := orderDetailFixture()
		d.PaymentStatus = "REFUND_FAILED"
		d.RefundTransId = "momo-ref-42"
		d.RefundedAt = "2026-11-25T10:00:00Z"
		d.RefundLastError = "MoMo HTTP 502 raw gateway stack detail"
		d.RefundActorType = "ADMIN"
		d.RefundReason = "Internal ops escalation note"
		return d
	}

	mux := http.NewServeMux()
	NewOrdersHandler(&fakeOrderClient{
		createFn: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			d := detailWithInternalRefund()
			return &orderv1.CreateOrderResponse{OrderId: d.OrderId, Status: d.Status, TotalAmount: d.TotalAmount, Order: d}, nil
		},
		listFn: func(context.Context, string, string, int32, int32) (*orderv1.ListOrdersResponse, error) {
			return &orderv1.ListOrdersResponse{Items: []*orderv1.OrderDetail{detailWithInternalRefund()}, Page: 1, PageSize: 10, TotalItems: 1, TotalPages: 1}, nil
		},
		getFn: func(context.Context, string, string) (*orderv1.GetOrderResponse, error) {
			return &orderv1.GetOrderResponse{Order: detailWithInternalRefund()}, nil
		},
		cancelFn: func(context.Context, string, string, string) (*orderv1.CancelOrderResponse, error) {
			return &orderv1.CancelOrderResponse{Order: detailWithInternalRefund()}, nil
		},
	}).Register(mux)

	createBody := `{"paymentMethod":"COD","shippingAddress":{"recipientName":"A","phone":"0901234567","provinceCode":"79","provinceName":"HCM","wardCode":"26740","wardName":"BN","addressLine":"1 LD"},"items":[{"productId":"dt-1","skuId":"dt-1-v1","quantity":1,"unitPrice":1000,"expectedName":"Nova","variantLabel":"Default","expectedProductVersion":1}]}`

	cases := []struct {
		name           string
		method         string
		target         string
		body           string
		idempotencyKey string
	}{
		{name: "create", method: http.MethodPost, target: "/api/v1/orders", body: createBody, idempotencyKey: "idem-refund-redact"},
		{name: "list", method: http.MethodGet, target: "/api/v1/orders"},
		{name: "get", method: http.MethodGet, target: "/api/v1/orders/ord-123"},
		{name: "cancel", method: http.MethodPost, target: "/api/v1/orders/ord-123/cancel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.Header.Set("X-User-Id", "user-1")
			if tc.idempotencyKey != "" {
				req.Header.Set("Idempotency-Key", tc.idempotencyKey)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
				t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
			}
			raw := rec.Body.String()
			for _, forbidden := range []string{"refundLastError", "refundActorType", "refundReason"} {
				if strings.Contains(raw, forbidden) {
					t.Fatalf("customer %s response must not expose %q:\n%s", tc.name, forbidden, raw)
				}
			}
			for _, expected := range []string{`"refundTransId":"momo-ref-42"`, `"refundedAt":"2026-11-25T10:00:00Z"`} {
				if !strings.Contains(raw, expected) {
					t.Fatalf("customer %s response missing %s:\n%s", tc.name, expected, raw)
				}
			}
		})
	}
}
