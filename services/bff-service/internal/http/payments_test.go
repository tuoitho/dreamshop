package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	paymentclient "ecommerce.local/bff-service/internal/payment"
	paymentv1 "ecommerce.local/contracts/go/payment/v1"
	"google.golang.org/grpc/codes"
)

type fakePaymentClient struct {
	callbackFn func(context.Context, *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error)
}

func (f *fakePaymentClient) HandleMomoCallback(ctx context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
	return f.callbackFn(ctx, req)
}

func TestMomoRedirectReturns302OnValidSignatureAnd400OnInvalidSignature(t *testing.T) {
	mux := http.NewServeMux()
	NewPaymentsHandler(&fakePaymentClient{
		callbackFn: func(_ context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
			if req.GetSource() != "REDIRECT" {
				t.Fatalf("expected source REDIRECT, got %q", req.GetSource())
			}
			if req.GetSignature() == "forged" {
				return nil, &paymentclient.ServiceError{
					GRPCStatus: codes.InvalidArgument,
					Reason:     "INVALID_MOMO_SIGNATURE",
					Message:    "Chữ ký phản hồi MoMo không hợp lệ",
				}
			}
			return &paymentv1.HandleMomoCallbackResponse{
				OrderId:        req.GetOrderId(),
				PaymentStatus:  "SUCCEEDED",
				RedirectResult: "success",
			}, nil
		},
	}, "http://localhost:5173").Register(mux)

	validReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/momo-redirect?partnerCode=MOMO&orderId=ord-123&requestId=req-1&amount=300000&orderInfo=test&orderType=momo_wallet&transId=998877&resultCode=0&message=Successful.&payType=qr&responseTime=1730000000000&extraData=&signature=valid-sig", nil)
	validRec := httptest.NewRecorder()
	mux.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got %d: %s", validRec.Code, validRec.Body.String())
	}
	if loc := validRec.Header().Get("Location"); loc != "http://localhost:5173/thanh-toan/ket-qua?orderId=ord-123&result=success" {
		t.Fatalf("unexpected Location header: %q", loc)
	}

	forgedReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/momo-redirect?partnerCode=MOMO&orderId=ord-123&requestId=req-1&amount=300000&resultCode=0&responseTime=1730000000000&signature=forged", nil)
	forgedRec := httptest.NewRecorder()
	mux.ServeHTTP(forgedRec, forgedReq)
	if forgedRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on forged signature, got %d: %s", forgedRec.Code, forgedRec.Body.String())
	}
	if body := decodeBody(t, forgedRec); body["code"] != "INVALID_MOMO_SIGNATURE" {
		t.Fatalf("expected code INVALID_MOMO_SIGNATURE, got %v", body["code"])
	}
}

func TestMomoIpnReturns204OnValidSignatureAnd400OnForgedSignature(t *testing.T) {
	mux := http.NewServeMux()
	NewPaymentsHandler(&fakePaymentClient{
		callbackFn: func(_ context.Context, req *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
			if req.GetSource() != "IPN" || req.GetTransId() != "4099112233" || req.GetAmount() != 300000 {
				t.Fatalf("unexpected IPN request: %+v", req)
			}
			if req.GetSignature() != "valid-ipn-sig" {
				return nil, &paymentclient.ServiceError{
					GRPCStatus: codes.InvalidArgument,
					Reason:     "INVALID_MOMO_SIGNATURE",
					Message:    "Chữ ký phản hồi MoMo không hợp lệ",
				}
			}
			return &paymentv1.HandleMomoCallbackResponse{
				OrderId:        req.GetOrderId(),
				PaymentStatus:  "SUCCEEDED",
				RedirectResult: "success",
			}, nil
		},
	}, "http://localhost:5173").Register(mux)

	ipnBody := `{"partnerCode":"MOMO","orderId":"ord-123","requestId":"req-1","amount":300000,"orderInfo":"Pay","orderType":"momo_wallet","transId":4099112233,"resultCode":0,"message":"Success","payType":"qr","responseTime":1730000000000,"extraData":"","signature":"valid-ipn-sig","lang":"vi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/momo-ipn", strings.NewReader(ipnBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d: %s", rec.Code, rec.Body.String())
	}

	badBody := strings.ReplaceAll(ipnBody, "valid-ipn-sig", "forged-sig")
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/momo-ipn", strings.NewReader(badBody))
	badRec := httptest.NewRecorder()
	mux.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", badRec.Code, badRec.Body.String())
	}
}
