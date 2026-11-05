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
