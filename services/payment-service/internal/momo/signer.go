package momo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type CreateSignParams struct {
	AccessKey   string
	Amount      int64
	ExtraData   string
	IPNURL      string
	OrderID     string
	OrderInfo   string
	PartnerCode string
	RedirectURL string
	RequestID   string
	RequestType string
}

type CallbackSignParams struct {
	AccessKey    string
	Amount       int64
	ExtraData    string
	Message      string
	OrderID      string
	OrderInfo    string
	OrderType    string
	PartnerCode  string
	PayType      string
	RequestID    string
	ResponseTime int64
	ResultCode   int32
	TransID      string
}

type RefundSignParams struct {
	AccessKey   string
	Amount      int64
	Description string
	OrderID     string
	PartnerCode string
	RequestID   string
	TransID     int64
}

func SignRefundRequest(p RefundSignParams, secret string) string {
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%d&description=%s&orderId=%s&partnerCode=%s&requestId=%s&transId=%d",
		p.AccessKey,
		p.Amount,
		p.Description,
		p.OrderID,
		p.PartnerCode,
		p.RequestID,
		p.TransID,
	)
	return hex.EncodeToString(computeHMAC(raw, secret))
}

func SignCreateRequest(p CreateSignParams, secret string) string {
	reqType := strings.TrimSpace(p.RequestType)
	if reqType == "" {
		reqType = "captureWallet"
	}
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%d&extraData=%s&ipnUrl=%s&orderId=%s&orderInfo=%s&partnerCode=%s&redirectUrl=%s&requestId=%s&requestType=%s",
		p.AccessKey,
		p.Amount,
		p.ExtraData,
		p.IPNURL,
		p.OrderID,
		p.OrderInfo,
		p.PartnerCode,
		p.RedirectURL,
		p.RequestID,
		reqType,
	)
	return hex.EncodeToString(computeHMAC(raw, secret))
}

func SignCallback(p CallbackSignParams, secret string) string {
	return hex.EncodeToString(computeCallbackMAC(p, secret))
}

func VerifyCallbackSignature(p CallbackSignParams, providedSignature, secret string) bool {
	if secret == "" {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(providedSignature))
	if err != nil {
		return false
	}
	expected := computeCallbackMAC(p, secret)
	return hmac.Equal(sigBytes, expected)
}

func computeCallbackMAC(p CallbackSignParams, secret string) []byte {
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%d&extraData=%s&message=%s&orderId=%s&orderInfo=%s&orderType=%s&partnerCode=%s&payType=%s&requestId=%s&responseTime=%d&resultCode=%d&transId=%s",
		p.AccessKey,
		p.Amount,
		p.ExtraData,
		p.Message,
		p.OrderID,
		p.OrderInfo,
		p.OrderType,
		p.PartnerCode,
		p.PayType,
		p.RequestID,
		p.ResponseTime,
		p.ResultCode,
		p.TransID,
	)
	return computeHMAC(raw, secret)
}

func computeHMAC(raw, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(raw))
	return mac.Sum(nil)
}
