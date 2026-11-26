package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orderclient "ecommerce.local/bff-service/internal/order"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/grpc/codes"
)

func serveAdminOrders(t *testing.T, client *fakeOrderClient, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewAdminOrdersHandler(client).Register(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAdminListOrdersForwardsFiltersAndReturnsStatusCounts(t *testing.T) {
	client := &fakeOrderClient{
		adminListFn: func(_ context.Context, userID, statusFilter, query string, page, pageSize int32) (*orderv1.AdminListOrdersResponse, error) {
			if userID != "admin-1" || statusFilter != "SHIPPING" || query != "ORD" || page != 2 || pageSize != 30 {
				t.Fatalf("unexpected list args: %q %q %q %d %d", userID, statusFilter, query, page, pageSize)
			}
			return &orderv1.AdminListOrdersResponse{
				Items:        []*orderv1.OrderDetail{orderDetailFixture()},
				Page:         2,
				PageSize:     30,
				TotalItems:   31,
				TotalPages:   2,
				StatusCounts: map[string]int64{"SHIPPING": 31, "DELIVERED": 0},
			}, nil
		},
	}

	rec := serveAdminOrders(t, client, http.MethodGet, "/api/v1/admin/orders?status=SHIPPING&q=ORD&page=2&pageSize=30", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	if data["totalItems"] != float64(31) || data["pageSize"] != float64(30) || len(data["items"].([]any)) != 1 {
		t.Fatalf("unexpected list data: %v", data)
	}
	counts := data["statusCounts"].(map[string]any)
	if counts["SHIPPING"] != float64(31) || counts["DELIVERED"] != float64(0) {
		t.Fatalf("unexpected statusCounts: %v", counts)
	}

	tooLarge := serveAdminOrders(t, client, http.MethodGet, "/api/v1/admin/orders?pageSize=101", "")
	if tooLarge.Code != http.StatusBadRequest || decodeBody(t, tooLarge)["code"] != "INVALID_PARAMETER" {
		t.Fatalf("expected 400 INVALID_PARAMETER for pageSize=101, got %d: %s", tooLarge.Code, tooLarge.Body.String())
	}
}

func TestAdminGetOrderReturnsHistoryAndAllowedNextStatuses(t *testing.T) {
	client := &fakeOrderClient{
		adminGetFn: func(_ context.Context, userID, orderID string) (*orderv1.AdminGetOrderResponse, error) {
			if userID != "admin-1" || orderID != "ord-123" {
				t.Fatalf("unexpected get args: %q %q", userID, orderID)
			}
			detail := orderDetailFixture()
			detail.Status = "CONFIRMED"
			detail.StatusHistory = []*orderv1.OrderStatusChange{{FromStatus: "PENDING_CONFIRMATION", ToStatus: "CONFIRMED", ActorType: "SYSTEM"}}
			return &orderv1.AdminGetOrderResponse{Order: detail, AllowedNextStatuses: []string{"PREPARING", "CANCELLED"}}, nil
		},
	}

	rec := serveAdminOrders(t, client, http.MethodGet, "/api/v1/admin/orders/ord-123", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	order := data["order"].(map[string]any)
	if len(order["statusHistory"].([]any)) != 1 {
		t.Fatalf("expected one history entry, got %v", order["statusHistory"])
	}
	next := data["allowedNextStatuses"].([]any)
	if len(next) != 2 || next[0] != "PREPARING" || next[1] != "CANCELLED" {
		t.Fatalf("unexpected allowedNextStatuses: %v", next)
	}
}

func TestAdminUpdateOrderStatusForwardsBodyAndReturnsMessage(t *testing.T) {
	client := &fakeOrderClient{
		adminUpdateFn: func(_ context.Context, req *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error) {
			if req.GetUserId() != "admin-1" || req.GetOrderId() != "ord-123" || req.GetExpectedStatus() != "PREPARING" ||
				req.GetTargetStatus() != "SHIPPING" || req.GetReason() != "ok" || req.GetShippingCarrier() != "GHN" || req.GetTrackingNumber() != "T1" {
				t.Fatalf("unexpected update request: %+v", req)
			}
			detail := orderDetailFixture()
			detail.Status = "SHIPPING"
			return &orderv1.AdminUpdateOrderStatusResponse{Order: detail, AllowedNextStatuses: []string{"DELIVERED", "DELIVERY_FAILED"}}, nil
		},
	}

	rec := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/status",
		`{"expectedStatus":"PREPARING","targetStatus":"SHIPPING","reason":" ok ","shippingCarrier":" GHN ","trackingNumber":" T1 "}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["message"] != "Cập nhật trạng thái đơn hàng thành công" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["data"].(map[string]any)["order"].(map[string]any)["status"] != "SHIPPING" {
		t.Fatalf("unexpected data: %v", body["data"])
	}

	missingTarget := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/status", `{"expectedStatus":"PREPARING"}`)
	if missingTarget.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing targetStatus, got %d: %s", missingTarget.Code, missingTarget.Body.String())
	}
}

func TestAdminOrderErrorsMapToStableCodes(t *testing.T) {
	for _, tc := range []struct {
		grpc   codes.Code
		reason string
		status int
		code   string
	}{
		{codes.Aborted, "ORDER_STATUS_CONFLICT", http.StatusConflict, "ORDER_STATUS_CONFLICT"},
		{codes.FailedPrecondition, "ORDER_INVALID_TRANSITION", http.StatusConflict, "ORDER_INVALID_TRANSITION"},
		{codes.InvalidArgument, "ORDER_REASON_REQUIRED", http.StatusBadRequest, "ORDER_REASON_REQUIRED"},
		{codes.PermissionDenied, "ORDER_PERMISSION_DENIED", http.StatusForbidden, "FORBIDDEN"},
		{codes.Unavailable, "AUTHORIZATION_UNAVAILABLE", http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE"},
	} {
		client := &fakeOrderClient{
			adminUpdateFn: func(context.Context, *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error) {
				return nil, &orderclient.ServiceError{GRPCStatus: tc.grpc, Reason: tc.reason, Message: "msg"}
			},
		}

		rec := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/status",
			`{"expectedStatus":"SHIPPING","targetStatus":"DELIVERY_FAILED"}`)

		if rec.Code != tc.status || decodeBody(t, rec)["code"] != tc.code {
			t.Fatalf("%s: got %d %s, want %d %s", tc.reason, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}
}

func TestAdminRetryRefundForwardsReasonAndReturnsUpdatedOrder(t *testing.T) {
	var capturedReqs []*orderv1.AdminRetryRefundRequest
	client := &fakeOrderClient{
		adminRetryRefundFn: func(_ context.Context, req *orderv1.AdminRetryRefundRequest) (*orderv1.AdminRetryRefundResponse, error) {
			capturedReqs = append(capturedReqs, req)
			detail := orderDetailFixture()
			detail.Status = "CANCELLED"
			detail.PaymentStatus = "REFUNDED"
			detail.RefundTransId = "momo-ref-99"
			detail.RefundActorType = "ADMIN"
			detail.RefundedAt = "2026-11-25T10:00:00Z"
			detail.RefundReason = req.GetReason()
			return &orderv1.AdminRetryRefundResponse{Order: detail, AllowedNextStatuses: []string{}}, nil
		},
	}

	rec := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/refund",
		`{"reason":"MoMo đã hoạt động lại"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["message"] != "Hoàn tiền đơn hàng thành công" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	orderMap := body["data"].(map[string]any)["order"].(map[string]any)
	if orderMap["paymentStatus"] != "REFUNDED" ||
		orderMap["refundTransId"] != "momo-ref-99" ||
		orderMap["refundActorType"] != "ADMIN" ||
		orderMap["refundedAt"] != "2026-11-25T10:00:00Z" ||
		orderMap["refundReason"] != "MoMo đã hoạt động lại" {
		t.Fatalf("unexpected order refund fields: %v", orderMap)
	}

	// Empty body is also allowed.
	recEmpty := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/refund", "")
	if recEmpty.Code != http.StatusOK {
		t.Fatalf("expected 200 on empty body, got %d: %s", recEmpty.Code, recEmpty.Body.String())
	}
	if len(capturedReqs) != 2 || capturedReqs[0].GetUserId() != "admin-1" || capturedReqs[0].GetOrderId() != "ord-123" || capturedReqs[1].GetReason() != "" {
		t.Fatalf("unexpected captured requests: %+v", capturedReqs)
	}
}

func TestAdminRetryRefundMapsConflictAndUnavailableErrors(t *testing.T) {
	for _, tc := range []struct {
		grpc   codes.Code
		reason string
		status int
		code   string
	}{
		{codes.FailedPrecondition, "REFUND_AUTO_RETRY_IN_PROGRESS", http.StatusConflict, "REFUND_AUTO_RETRY_IN_PROGRESS"},
		{codes.FailedPrecondition, "ORDER_REFUND_NOT_ELIGIBLE", http.StatusConflict, "ORDER_REFUND_NOT_ELIGIBLE"},
		{codes.Aborted, "REFUND_IN_PROGRESS", http.StatusConflict, "REFUND_IN_PROGRESS"},
		{codes.FailedPrecondition, "PAYMENT_GATEWAY_REJECTED", http.StatusConflict, "PAYMENT_GATEWAY_REJECTED"},
		{codes.Unavailable, "PAYMENT_GATEWAY_UNAVAILABLE", http.StatusServiceUnavailable, "PAYMENT_GATEWAY_UNAVAILABLE"},
	} {
		client := &fakeOrderClient{
			adminRetryRefundFn: func(context.Context, *orderv1.AdminRetryRefundRequest) (*orderv1.AdminRetryRefundResponse, error) {
				return nil, &orderclient.ServiceError{GRPCStatus: tc.grpc, Reason: tc.reason, Message: "err msg"}
			},
		}
		rec := serveAdminOrders(t, client, http.MethodPost, "/api/v1/admin/orders/ord-123/refund", `{}`)
		if rec.Code != tc.status || decodeBody(t, rec)["code"] != tc.code {
			t.Fatalf("%s: got %d %s, want %d %s", tc.reason, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}

	// Unknown JSON field returns 400 INVALID_PARAMETER.
	recUnknown := serveAdminOrders(t, &fakeOrderClient{}, http.MethodPost, "/api/v1/admin/orders/ord-123/refund", `{"unknownField":1}`)
	if recUnknown.Code != http.StatusBadRequest || decodeBody(t, recUnknown)["code"] != "INVALID_PARAMETER" {
		t.Fatalf("expected 400 INVALID_PARAMETER on unknown field, got %d: %s", recUnknown.Code, recUnknown.Body.String())
	}
}
