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
