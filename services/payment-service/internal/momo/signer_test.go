package momo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSignCreateRequestAndVerifyCallbackSignature(t *testing.T) {
	secret := "test-momo-secret-key"
	createParams := CreateSignParams{
		AccessKey:   "MOMO_ACCESS",
		Amount:      150000,
		ExtraData:   "",
		IPNURL:      "http://localhost:8081/api/v1/payments/momo-ipn",
		OrderID:     "ord-123",
		OrderInfo:   "Thanh toán đơn hàng ORD-20261105-ABC123",
		PartnerCode: "MOMO",
		RedirectURL: "http://localhost:8081/api/v1/payments/momo-redirect",
		RequestID:   "req-123",
	}

	rawExpected := "accessKey=MOMO_ACCESS&amount=150000&extraData=&ipnUrl=http://localhost:8081/api/v1/payments/momo-ipn&orderId=ord-123&orderInfo=Thanh toán đơn hàng ORD-20261105-ABC123&partnerCode=MOMO&redirectUrl=http://localhost:8081/api/v1/payments/momo-redirect&requestId=req-123&requestType=captureWallet"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rawExpected))
	wantCreateSig := hex.EncodeToString(mac.Sum(nil))

	gotCreateSig := SignCreateRequest(createParams, secret)
	if gotCreateSig != wantCreateSig {
		t.Fatalf("SignCreateRequest() = %q, want %q", gotCreateSig, wantCreateSig)
	}

	cbParams := CallbackSignParams{
		AccessKey:    "MOMO_ACCESS",
		Amount:       150000,
		ExtraData:    "",
		Message:      "Successful.",
		OrderID:      "ord-123",
		OrderInfo:    "Thanh toán đơn hàng ORD-20261105-ABC123",
		OrderType:    "momo_wallet",
		PartnerCode:  "MOMO",
		PayType:      "qr",
		RequestID:    "req-123",
		ResponseTime: 1730800000000,
		ResultCode:   0,
		TransID:      "9988776655",
	}
	validCbSig := SignCallback(cbParams, secret)
	if !VerifyCallbackSignature(cbParams, validCbSig, secret) {
		t.Fatal("VerifyCallbackSignature rejected valid signature")
	}

	tamperedSuccess := cbParams
	tamperedSuccess.Amount = 1000
	if VerifyCallbackSignature(tamperedSuccess, validCbSig, secret) {
		t.Fatal("VerifyCallbackSignature accepted tampered amount")
	}

	tamperedFailure := cbParams
	tamperedFailure.ResultCode = 1006
	if VerifyCallbackSignature(tamperedFailure, validCbSig, secret) {
		t.Fatal("VerifyCallbackSignature accepted tampered resultCode=1006")
	}

	if VerifyCallbackSignature(cbParams, "deadbeef", secret) {
		t.Fatal("VerifyCallbackSignature accepted forged signature")
	}
}

func TestSignRefundRequestMatchesMoMoV2RawHash(t *testing.T) {
	secret := "test-momo-secret-key"
	params := RefundSignParams{
		AccessKey:   "MOMO_ACCESS",
		Amount:      150000,
		Description: "Hoan tien don hang ORD-20261125-ABC123",
		OrderID:     "REFUND0123456789ABCDEF0123456789ABCDEF",
		PartnerCode: "MOMO",
		RequestID:   "REFREQ0123456789ABCDEF012345",
		TransID:     2314756173,
	}

	rawExpected := "accessKey=MOMO_ACCESS&amount=150000&description=Hoan tien don hang ORD-20261125-ABC123&orderId=REFUND0123456789ABCDEF0123456789ABCDEF&partnerCode=MOMO&requestId=REFREQ0123456789ABCDEF012345&transId=2314756173"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rawExpected))
	want := hex.EncodeToString(mac.Sum(nil))

	got := SignRefundRequest(params, secret)
	if got != want {
		t.Fatalf("SignRefundRequest() = %q, want %q", got, want)
	}

	payMethodParams := CreateSignParams{
		AccessKey:   "MOMO_ACCESS",
		Amount:      50000,
		ExtraData:   "",
		IPNURL:      "https://shop.example.com/api/v1/payments/momo-ipn",
		OrderID:     "ord-pm-1",
		OrderInfo:   "Order",
		PartnerCode: "MOMO",
		RedirectURL: "https://shop.example.com/api/v1/payments/momo-redirect",
		RequestID:   "req-pm-1",
		RequestType: "payWithMethod",
	}
	rawPM := "accessKey=MOMO_ACCESS&amount=50000&extraData=&ipnUrl=https://shop.example.com/api/v1/payments/momo-ipn&orderId=ord-pm-1&orderInfo=Order&partnerCode=MOMO&redirectUrl=https://shop.example.com/api/v1/payments/momo-redirect&requestId=req-pm-1&requestType=payWithMethod"
	macPM := hmac.New(sha256.New, []byte(secret))
	macPM.Write([]byte(rawPM))
	if gotPM := SignCreateRequest(payMethodParams, secret); gotPM != hex.EncodeToString(macPM.Sum(nil)) {
		t.Fatalf("SignCreateRequest(payWithMethod) mismatch: got %q", gotPM)
	}
}
