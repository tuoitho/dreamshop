package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	paymentclient "ecommerce.local/bff-service/internal/payment"
	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"google.golang.org/grpc/codes"
)

type momoIPNPayload struct {
	PartnerCode  string          `json:"partnerCode"`
	OrderID      string          `json:"orderId"`
	RequestID    string          `json:"requestId"`
	Amount       int64           `json:"amount"`
	OrderInfo    string          `json:"orderInfo"`
	OrderType    string          `json:"orderType"`
	TransID      json.RawMessage `json:"transId"`
	ResultCode   int32           `json:"resultCode"`
	Message      string          `json:"message"`
	PayType      string          `json:"payType"`
	ResponseTime int64           `json:"responseTime"`
	ExtraData    string          `json:"extraData"`
	Signature    string          `json:"signature"`
}

type PaymentsHandler struct {
	payments          paymentclient.Client
	storefrontBaseURL string
}

func NewPaymentsHandler(client paymentclient.Client, storefrontBaseURL string) *PaymentsHandler {
	base := strings.TrimRight(strings.TrimSpace(storefrontBaseURL), "/")
	return &PaymentsHandler{
		payments:          client,
		storefrontBaseURL: base,
	}
}

func (h *PaymentsHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/payments/momo-redirect", correlateTrace(http.HandlerFunc(h.momoRedirect)))
	mux.Handle("POST /api/v1/payments/momo-ipn", correlateTrace(http.HandlerFunc(h.momoIPN)))
}

func (h *PaymentsHandler) momoRedirect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	orderID := strings.TrimSpace(q.Get("orderId"))
	signature := strings.TrimSpace(q.Get("signature"))
	if orderID == "" || signature == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Missing MoMo redirect parameters.", nil)
		return
	}

	amount, err := parseOptionalInt64(q.Get("amount"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid MoMo amount.", nil)
		return
	}
	resultCode64, err := parseOptionalInt64(q.Get("resultCode"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid MoMo resultCode.", nil)
		return
	}
	responseTime, err := parseOptionalInt64(q.Get("responseTime"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid MoMo responseTime.", nil)
		return
	}

	resp, err := h.payments.HandleMomoCallback(r.Context(), &paymentv1.HandleMomoCallbackRequest{
		PartnerCode:  q.Get("partnerCode"),
		OrderId:      orderID,
		RequestId:    q.Get("requestId"),
		Amount:       amount,
		OrderInfo:    q.Get("orderInfo"),
		OrderType:    q.Get("orderType"),
		TransId:      q.Get("transId"),
		ResultCode:   int32(resultCode64),
		Message:      q.Get("message"),
		PayType:      q.Get("payType"),
		ResponseTime: responseTime,
		ExtraData:    q.Get("extraData"),
		Signature:    signature,
		Source:       "REDIRECT",
	})
	if err != nil {
		h.writePaymentError(w, r, err)
		return
	}

	redirectResult := resp.GetRedirectResult()
	if redirectResult == "" {
		if resultCode64 == 0 {
			redirectResult = "success"
		} else {
			redirectResult = "failed"
		}
	}
	target := h.storefrontBaseURL + "/thanh-toan/ket-qua?orderId=" + url.QueryEscape(resp.GetOrderId()) + "&result=" + url.QueryEscape(redirectResult)
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *PaymentsHandler) momoIPN(w http.ResponseWriter, r *http.Request) {
	var payload momoIPNPayload
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid MoMo IPN payload.", nil)
		return
	}
	if strings.TrimSpace(payload.OrderID) == "" || strings.TrimSpace(payload.Signature) == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Missing MoMo IPN fields.", nil)
		return
	}

	_, err := h.payments.HandleMomoCallback(r.Context(), &paymentv1.HandleMomoCallbackRequest{
		PartnerCode:  payload.PartnerCode,
		OrderId:      payload.OrderID,
		RequestId:    payload.RequestID,
		Amount:       payload.Amount,
		OrderInfo:    payload.OrderInfo,
		OrderType:    payload.OrderType,
		TransId:      rawJSONScalarString(payload.TransID),
		ResultCode:   payload.ResultCode,
		Message:      payload.Message,
		PayType:      payload.PayType,
		ResponseTime: payload.ResponseTime,
		ExtraData:    payload.ExtraData,
		Signature:    payload.Signature,
		Source:       "IPN",
	})
	if err != nil {
		h.writePaymentError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentsHandler) writePaymentError(w http.ResponseWriter, r *http.Request, err error) {
	var svcErr *paymentclient.ServiceError
	if errors.As(err, &svcErr) {
		msg := svcErr.Message
		if msg == "" {
			msg = "Phản hồi thanh toán không hợp lệ."
		}
		switch svcErr.Reason {
		case "INVALID_MOMO_SIGNATURE", "PAYMENT_AMOUNT_MISMATCH", "INVALID_PARAMETER":
			code := svcErr.Reason
			if code == "" {
				code = "INVALID_PARAMETER"
			}
			writeError(w, r, http.StatusBadRequest, code, msg, nil)
			return
		case "PAYMENT_NOT_FOUND":
			writeError(w, r, http.StatusNotFound, "PAYMENT_NOT_FOUND", msg, nil)
			return
		}

		switch svcErr.GRPCStatus {
		case codes.InvalidArgument:
			writeError(w, r, http.StatusBadRequest, "INVALID_MOMO_SIGNATURE", msg, nil)
		case codes.NotFound:
			writeError(w, r, http.StatusNotFound, "PAYMENT_NOT_FOUND", msg, nil)
		default:
			writeError(w, r, http.StatusServiceUnavailable, "PAYMENT_SERVICE_UNAVAILABLE", "Dịch vụ thanh toán tạm thời gián đoạn.", nil)
		}
		return
	}

	writeError(w, r, http.StatusServiceUnavailable, "PAYMENT_SERVICE_UNAVAILABLE", "Dịch vụ thanh toán tạm thời gián đoạn.", nil)
}

func parseOptionalInt64(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.ParseInt(trimmed, 10, 64)
}

func rawJSONScalarString(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return s
		}
	}
	return string(trimmed)
}
