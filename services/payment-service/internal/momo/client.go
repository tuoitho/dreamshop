package momo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrGatewayUnavailable = errors.New("momo payment gateway unavailable")

type GatewayRejectedError struct {
	ResultCode int32
	Message    string
}

func (e *GatewayRejectedError) Error() string {
	return fmt.Sprintf("momo gateway rejected session (resultCode=%d): %s", e.ResultCode, e.Message)
}

type ClientConfig struct {
	PartnerCode    string
	AccessKey      string
	SecretKey      string
	APIEndpoint    string
	RefundEndpoint string
	RedirectURL    string
	IPNURL         string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	MockEnabled    bool
}

type CreateSessionRequest struct {
	OrderID     string
	OrderCode   string
	RequestID   string
	AmountMinor int64
}

type RefundRequest struct {
	OrderID     string
	RequestID   string
	OrigTransID string
	AmountMinor int64
	Description string
}

type RefundResponse struct {
	ResultCode    int32
	Message       string
	RefundTransID string
}

func IsAmbiguousRefundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrGatewayUnavailable) {
		return true
	}
	var rej *GatewayRejectedError
	if errors.As(err, &rej) {
		return rej.ResultCode == 7000 || rej.ResultCode == 9000
	}
	return true
}

type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.PartnerCode == "" {
		cfg.PartnerCode = "MOMO"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 3 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.MockEnabled {
		if cfg.AccessKey == "" {
			cfg.AccessKey = "local-momo-mock-access"
		}
		if cfg.SecretKey == "" {
			cfg.SecretKey = "local-momo-mock-secret"
		}
	}
	if strings.TrimSpace(cfg.RefundEndpoint) == "" {
		if strings.HasSuffix(cfg.APIEndpoint, "/create") {
			cfg.RefundEndpoint = strings.TrimSuffix(cfg.APIEndpoint, "/create") + "/refund"
		} else {
			cfg.RefundEndpoint = "https://test-payment.momo.vn/v2/gateway/api/refund"
		}
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: cfg.ConnectTimeout,
		}).DialContext,
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   cfg.ReadTimeout,
			Transport: transport,
		},
	}
}

func (c *Client) VerifyCallback(params CallbackSignParams, signature string) bool {
	params.AccessKey = c.cfg.AccessKey
	return VerifyCallbackSignature(params, signature, c.cfg.SecretKey)
}

func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (string, error) {
	orderInfo := "Thanh toán đơn hàng " + req.OrderCode
	if req.OrderCode == "" {
		orderInfo = "Thanh toán đơn hàng " + req.OrderID
	}

	if c.cfg.MockEnabled {
		return c.buildSignedMockRedirectURL(req, orderInfo)
	}

	if c.cfg.AccessKey == "" || c.cfg.SecretKey == "" {
		return "", fmt.Errorf("%w: missing MoMo credentials", ErrGatewayUnavailable)
	}

	signature := SignCreateRequest(CreateSignParams{
		AccessKey:   c.cfg.AccessKey,
		Amount:      req.AmountMinor,
		ExtraData:   "",
		IPNURL:      c.cfg.IPNURL,
		OrderID:     req.OrderID,
		OrderInfo:   orderInfo,
		PartnerCode: c.cfg.PartnerCode,
		RedirectURL: c.cfg.RedirectURL,
		RequestID:   req.RequestID,
	}, c.cfg.SecretKey)

	payload := map[string]any{
		"partnerCode": c.cfg.PartnerCode,
		"accessKey":   c.cfg.AccessKey,
		"requestId":   req.RequestID,
		"amount":      req.AmountMinor,
		"orderId":     req.OrderID,
		"orderInfo":   orderInfo,
		"redirectUrl": c.cfg.RedirectURL,
		"ipnUrl":      c.cfg.IPNURL,
		"extraData":   "",
		"requestType": "captureWallet",
		"lang":        "vi",
		"signature":   signature,
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal momo create request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIEndpoint, bytes.NewReader(rawBody))
	if err != nil {
		return "", fmt.Errorf("%w: build request: %v", ErrGatewayUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrGatewayUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: unexpected HTTP status %d", ErrGatewayUnavailable, resp.StatusCode)
	}

	var parsed struct {
		ResultCode int32  `json:"resultCode"`
		Message    string `json:"message"`
		PayURL     string `json:"payUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("%w: decode response: %v", ErrGatewayUnavailable, err)
	}

	if parsed.ResultCode != 0 {
		return "", &GatewayRejectedError{
			ResultCode: parsed.ResultCode,
			Message:    parsed.Message,
		}
	}
	if strings.TrimSpace(parsed.PayURL) == "" {
		return "", fmt.Errorf("%w: empty payUrl", ErrGatewayUnavailable)
	}
	return parsed.PayURL, nil
}

func (c *Client) Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error) {
	description := truncateRunes(strings.TrimSpace(req.Description), 255)
	if description == "" {
		description = "Hoan tien don hang"
	}

	if c.cfg.MockEnabled {
		return &RefundResponse{
			ResultCode:    0,
			Message:       "Successful.",
			RefundTransID: fmt.Sprintf("mock-refund-%d", time.Now().UnixNano()),
		}, nil
	}

	if c.cfg.AccessKey == "" || c.cfg.SecretKey == "" {
		return nil, fmt.Errorf("%w: missing MoMo credentials", ErrGatewayUnavailable)
	}

	origTransID, err := strconv.ParseInt(strings.TrimSpace(req.OrigTransID), 10, 64)
	if err != nil || origTransID <= 0 {
		return nil, &GatewayRejectedError{
			ResultCode: -1,
			Message:    "invalid original momo transId",
		}
	}

	signature := SignRefundRequest(RefundSignParams{
		AccessKey:   c.cfg.AccessKey,
		Amount:      req.AmountMinor,
		Description: description,
		OrderID:     req.OrderID,
		PartnerCode: c.cfg.PartnerCode,
		RequestID:   req.RequestID,
		TransID:     origTransID,
	}, c.cfg.SecretKey)

	payload := map[string]any{
		"partnerCode": c.cfg.PartnerCode,
		"orderId":     req.OrderID,
		"requestId":   req.RequestID,
		"amount":      req.AmountMinor,
		"transId":     origTransID,
		"lang":        "vi",
		"description": description,
		"signature":   signature,
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal momo refund request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.RefundEndpoint, bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("%w: build refund request: %v", ErrGatewayUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGatewayUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: unexpected HTTP status %d", ErrGatewayUnavailable, resp.StatusCode)
	}

	var parsed struct {
		ResultCode int32           `json:"resultCode"`
		Message    string          `json:"message"`
		TransID    json.RawMessage `json:"transId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode refund response: %v", ErrGatewayUnavailable, err)
	}

	if parsed.ResultCode != 0 {
		return nil, &GatewayRejectedError{
			ResultCode: parsed.ResultCode,
			Message:    parsed.Message,
		}
	}

	return &RefundResponse{
		ResultCode:    parsed.ResultCode,
		Message:       parsed.Message,
		RefundTransID: parseRawTransID(parsed.TransID),
	}, nil
}

func parseRawTransID(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asStr string
	if err := json.Unmarshal(raw, &asStr); err == nil {
		return strings.TrimSpace(asStr)
	}
	return trimmed
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func (c *Client) buildSignedMockRedirectURL(req CreateSessionRequest, orderInfo string) (string, error) {
	u, err := url.Parse(c.cfg.RedirectURL)
	if err != nil {
		return "", fmt.Errorf("parse redirect URL: %w", err)
	}

	responseTime := time.Now().UnixMilli()
	transID := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	cbParams := CallbackSignParams{
		AccessKey:    c.cfg.AccessKey,
		Amount:       req.AmountMinor,
		ExtraData:    "",
		Message:      "Successful.",
		OrderID:      req.OrderID,
		OrderInfo:    orderInfo,
		OrderType:    "momo_wallet",
		PartnerCode:  c.cfg.PartnerCode,
		PayType:      "qr",
		RequestID:    req.RequestID,
		ResponseTime: responseTime,
		ResultCode:   0,
		TransID:      transID,
	}
	sig := SignCallback(cbParams, c.cfg.SecretKey)

	q := u.Query()
	q.Set("partnerCode", cbParams.PartnerCode)
	q.Set("orderId", cbParams.OrderID)
	q.Set("requestId", cbParams.RequestID)
	q.Set("amount", strconv.FormatInt(cbParams.Amount, 10))
	q.Set("orderInfo", cbParams.OrderInfo)
	q.Set("orderType", cbParams.OrderType)
	q.Set("transId", cbParams.TransID)
	q.Set("resultCode", strconv.Itoa(int(cbParams.ResultCode)))
	q.Set("message", cbParams.Message)
	q.Set("payType", cbParams.PayType)
	q.Set("responseTime", strconv.FormatInt(cbParams.ResponseTime, 10))
	q.Set("extraData", cbParams.ExtraData)
	q.Set("signature", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
