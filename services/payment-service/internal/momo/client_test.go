package momo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestClientMockModeGeneratesVerifiableSignedRedirectURL(t *testing.T) {
	client := NewClient(ClientConfig{
		PartnerCode:    "MOMO",
		AccessKey:      "TEST_ACCESS",
		SecretKey:      "TEST_SECRET",
		RedirectURL:    "http://localhost:8081/api/v1/payments/momo-redirect",
		IPNURL:         "http://localhost:8081/api/v1/payments/momo-ipn",
		ConnectTimeout: 1 * time.Second,
		ReadTimeout:    2 * time.Second,
		MockEnabled:    true,
	})

	payURL, err := client.CreateSession(context.Background(), CreateSessionRequest{
		OrderID:     "ord-mock-1",
		OrderCode:   "ORD-20261105-MOCK01",
		RequestID:   "req-mock-1",
		AmountMinor: 250000,
	})
	if err != nil {
		t.Fatalf("CreateSession mock: %v", err)
	}
	parsed, err := url.Parse(payURL)
	if err != nil {
		t.Fatalf("parse mock payURL: %v", err)
	}
	q := parsed.Query()
	amt, _ := strconv.ParseInt(q.Get("amount"), 10, 64)
	respTime, _ := strconv.ParseInt(q.Get("responseTime"), 10, 64)
	resCode, _ := strconv.Atoi(q.Get("resultCode"))
	ok := VerifyCallbackSignature(CallbackSignParams{
		AccessKey:    "TEST_ACCESS",
		Amount:       amt,
		ExtraData:    q.Get("extraData"),
		Message:      q.Get("message"),
		OrderID:      q.Get("orderId"),
		OrderInfo:    q.Get("orderInfo"),
		OrderType:    q.Get("orderType"),
		PartnerCode:  q.Get("partnerCode"),
		PayType:      q.Get("payType"),
		RequestID:    q.Get("requestId"),
		ResponseTime: respTime,
		ResultCode:   int32(resCode),
		TransID:      q.Get("transId"),
	}, q.Get("signature"), "TEST_SECRET")
	if !ok {
		t.Fatalf("mock payURL signature failed verification: %s", payURL)
	}
}

func TestClientRealHTTPHandlesSuccessRejectionAndTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body["orderId"] {
		case "ord-ok":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resultCode": 0,
				"message":    "Successful.",
				"payUrl":     "https://test-payment.momo.vn/pay/ord-ok",
			})
		case "ord-reject":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resultCode": 20,
				"message":    "Bad format request.",
			})
		case "ord-slow":
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{
		PartnerCode:    "MOMO",
		AccessKey:      "TEST_ACCESS",
		SecretKey:      "TEST_SECRET",
		APIEndpoint:    srv.URL,
		RedirectURL:    "http://localhost:8081/api/v1/payments/momo-redirect",
		IPNURL:         "http://localhost:8081/api/v1/payments/momo-ipn",
		ConnectTimeout: 50 * time.Millisecond,
		ReadTimeout:    80 * time.Millisecond,
	})

	payURL, err := client.CreateSession(context.Background(), CreateSessionRequest{
		OrderID:     "ord-ok",
		OrderCode:   "ORD-OK",
		RequestID:   "req-ok",
		AmountMinor: 100000,
	})
	if err != nil || payURL != "https://test-payment.momo.vn/pay/ord-ok" {
		t.Fatalf("CreateSession(ord-ok): payURL=%q err=%v", payURL, err)
	}

	_, err = client.CreateSession(context.Background(), CreateSessionRequest{
		OrderID:     "ord-reject",
		OrderCode:   "ORD-REJ",
		RequestID:   "req-rej",
		AmountMinor: 100000,
	})
	var rejErr *GatewayRejectedError
	if !errors.As(err, &rejErr) || rejErr.ResultCode != 20 {
		t.Fatalf("expected GatewayRejectedError(code=20), got %v", err)
	}

	_, err = client.CreateSession(context.Background(), CreateSessionRequest{
		OrderID:     "ord-slow",
		OrderCode:   "ORD-SLOW",
		RequestID:   "req-slow",
		AmountMinor: 100000,
	})
	if !errors.Is(err, ErrGatewayUnavailable) {
		t.Fatalf("expected ErrGatewayUnavailable on timeout, got %v", err)
	}
}

func TestClientRefundMockModeSucceedsWithoutHTTP(t *testing.T) {
	client := NewClient(ClientConfig{
		PartnerCode: "MOMO",
		MockEnabled: true,
	})

	resp, err := client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUND11111111111111111111111111111111",
		RequestID:   "REFREQ111111111111111111111111",
		OrigTransID: "mock-123",
		AmountMinor: 150000,
		Description: "Hoan tien don hang ORD-1",
	})
	if err != nil {
		t.Fatalf("Refund mock: %v", err)
	}
	if resp.ResultCode != 0 || resp.RefundTransID == "" {
		t.Fatalf("unexpected mock refund response: %+v", resp)
	}
}

func TestClientRefundRealModeSendsSignedPayloadAndParsesNumericTransID(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&captured); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCode":0,"message":"Thành công","transId":144518121}`))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{
		PartnerCode:    "MOMO",
		AccessKey:      "TEST_ACCESS",
		SecretKey:      "TEST_SECRET",
		RefundEndpoint: srv.URL,
		ConnectTimeout: time.Second,
		ReadTimeout:    2 * time.Second,
	})

	resp, err := client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUND22222222222222222222222222222222",
		RequestID:   "REFREQ222222222222222222222222",
		OrigTransID: "9988776655",
		AmountMinor: 250000,
		Description: "Hoan tien don hang ORD-2",
	})
	if err != nil {
		t.Fatalf("Refund real: %v", err)
	}
	if resp.ResultCode != 0 || resp.RefundTransID != "144518121" {
		t.Fatalf("unexpected refund response: %+v", resp)
	}

	wantSig := SignRefundRequest(RefundSignParams{
		AccessKey:   "TEST_ACCESS",
		Amount:      250000,
		Description: "Hoan tien don hang ORD-2",
		OrderID:     "REFUND22222222222222222222222222222222",
		PartnerCode: "MOMO",
		RequestID:   "REFREQ222222222222222222222222",
		TransID:     9988776655,
	}, "TEST_SECRET")
	if captured["partnerCode"] != "MOMO" ||
		captured["orderId"] != "REFUND22222222222222222222222222222222" ||
		captured["requestId"] != "REFREQ222222222222222222222222" ||
		captured["lang"] != "vi" ||
		captured["description"] != "Hoan tien don hang ORD-2" ||
		captured["signature"] != wantSig {
		t.Fatalf("unexpected payload: %+v", captured)
	}
	if amt, ok := captured["amount"].(json.Number); !ok || amt.String() != "250000" {
		t.Fatalf("expected numeric amount 250000, got %#v", captured["amount"])
	}
	if tid, ok := captured["transId"].(json.Number); !ok || tid.String() != "9988776655" {
		t.Fatalf("expected numeric transId 9988776655, got %#v", captured["transId"])
	}
}

func TestClientRefundRealModeHandlesErrorsAndClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body["orderId"] {
		case "REFUND503":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "REFUND1080":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"resultCode":1080,"message":"Refund rejected"}`))
		case "REFUND7000":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"resultCode":7000,"message":"Transaction is being processed"}`))
		}
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{
		PartnerCode:    "MOMO",
		AccessKey:      "TEST_ACCESS",
		SecretKey:      "TEST_SECRET",
		RefundEndpoint: srv.URL,
		ConnectTimeout: time.Second,
		ReadTimeout:    2 * time.Second,
	})

	_, err := client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUNDINVALID",
		RequestID:   "REFREQINVALID",
		OrigTransID: "not-a-number",
		AmountMinor: 100000,
		Description: "Hoan tien",
	})
	var rejErr *GatewayRejectedError
	if !errors.As(err, &rejErr) || IsAmbiguousRefundError(err) {
		t.Fatalf("expected non-ambiguous GatewayRejectedError for non-numeric transId, got %v", err)
	}

	_, err = client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUND503",
		RequestID:   "REFREQ503",
		OrigTransID: "123456",
		AmountMinor: 100000,
		Description: "Hoan tien",
	})
	if !errors.Is(err, ErrGatewayUnavailable) || !IsAmbiguousRefundError(err) {
		t.Fatalf("expected ambiguous ErrGatewayUnavailable on HTTP 503, got %v", err)
	}

	_, err = client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUND1080",
		RequestID:   "REFREQ1080",
		OrigTransID: "123456",
		AmountMinor: 100000,
		Description: "Hoan tien",
	})
	if !errors.As(err, &rejErr) || rejErr.ResultCode != 1080 || IsAmbiguousRefundError(err) {
		t.Fatalf("expected non-ambiguous GatewayRejectedError(1080), got %v", err)
	}

	_, err = client.Refund(context.Background(), RefundRequest{
		OrderID:     "REFUND7000",
		RequestID:   "REFREQ7000",
		OrigTransID: "123456",
		AmountMinor: 100000,
		Description: "Hoan tien",
	})
	if !errors.As(err, &rejErr) || rejErr.ResultCode != 7000 || !IsAmbiguousRefundError(err) {
		t.Fatalf("expected ambiguous GatewayRejectedError(7000), got %v", err)
	}
}
