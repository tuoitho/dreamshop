package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SnapshotProduct struct {
	ProductID      string
	SKUID          string
	Name           string
	Brand          string
	CategoryName   string
	VariantLabel   string
	ImageURL       string
	ImageEmoji     string
	UnitPrice      int64
	ProductVersion int64
}

type ShippingPayload struct {
	RecipientName string `json:"recipientName"`
	Phone         string `json:"phone"`
	ProvinceCode  string `json:"provinceCode"`
	ProvinceName  string `json:"provinceName"`
	WardCode      string `json:"wardCode"`
	WardName      string `json:"wardName"`
	AddressLine   string `json:"addressLine"`
}

type ScenarioResult struct {
	Scenario        string         `json:"scenario"`
	Goroutines      int            `json:"goroutines"`
	Passed          bool           `json:"passed"`
	P50Ms           float64        `json:"p50Ms"`
	P95Ms           float64        `json:"p95Ms"`
	StatusHistogram map[int]int    `json:"statusHistogram"`
	ErrorHistogram  map[string]int `json:"errorHistogram"`
	Notes           []string       `json:"notes,omitempty"`
	Violations      []string       `json:"violations,omitempty"`
}

type FlashSaleConfig struct {
	BaseURL        string
	HTTPClient     *http.Client
	AdminToken     string
	CustomerTokens []string
	Product        SnapshotProduct
	InitialStock   int
	Buyers         int
	SettleTimeout  time.Duration
	Shipping       ShippingPayload
}

type SnapshotRaceConfig struct {
	BaseURL            string
	HTTPClient         *http.Client
	AdminToken         string
	CustomerToken      string
	Product            SnapshotProduct
	NewPrice           int64
	AdminUpdatePayload map[string]any
	Buyers             int
	Shipping           ShippingPayload
}

type IdempotencyStormConfig struct {
	BaseURL               string
	HTTPClient            *http.Client
	CustomerToken         string
	Product               SnapshotProduct
	SamePayloadWorkers    int
	MutatedPayloadWorkers int
	IdempotencyKey        string
	Shipping              ShippingPayload
}

type MomoCallbackRaceConfig struct {
	BaseURL         string
	HTTPClient      *http.Client
	CustomerToken   string
	PartnerCode     string
	AccessKey       string
	SecretKey       string
	ValidRedirects  int
	ValidIPNs       int
	ForgedCallbacks int
	CancelRequests  int
	SettleTimeout   time.Duration
	Product         SnapshotProduct
	Shipping        ShippingPayload
}

type callOutcome struct {
	statusCode int
	errCode    string
	latency    time.Duration
	order      *orderEnvelope
	transport  error
}

type orderEnvelope struct {
	OrderID       string `json:"orderId"`
	OrderCode     string `json:"orderCode"`
	Status        string `json:"status"`
	PaymentMethod string `json:"paymentMethod"`
	PaymentStatus string `json:"paymentStatus"`
	TotalAmount   int64  `json:"totalAmount"`
	PayURL        string `json:"payUrl"`
	Items         []struct {
		SKUID     string `json:"skuId"`
		UnitPrice int64  `json:"unitPrice"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

func RunFlashSaleOversell(ctx context.Context, cfg FlashSaleConfig) (ScenarioResult, error) {
	if cfg.Buyers <= 0 {
		cfg.Buyers = 50
	}
	if cfg.InitialStock <= 0 {
		cfg.InitialStock = 5
	}
	if cfg.SettleTimeout <= 0 {
		cfg.SettleTimeout = 5 * time.Second
	}
	client := httpClientOrDefault(cfg.HTTPClient)
	if len(cfg.CustomerTokens) == 0 {
		return ScenarioResult{}, fmt.Errorf("at least one customer token is required")
	}

	outcomes := make([]callOutcome, cfg.Buyers)
	startGate := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < cfg.Buyers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tok := cfg.CustomerTokens[idx%len(cfg.CustomerTokens)]
			idemKey := fmt.Sprintf("flash-%s-%d", randomHex(6), idx)
			body := buildOrderPayload("COD", cfg.Product, 1, cfg.Shipping)
			<-startGate
			outcomes[idx] = postOrder(ctx, client, cfg.BaseURL, tok, idemKey, body)
		}(i)
	}

	close(startGate)
	wg.Wait()

	res := summarizeOutcomes("flash-sale-oversell", cfg.Buyers, outcomes)
	confirmed := 0
	for _, out := range outcomes {
		if out.transport != nil {
			res.Violations = append(res.Violations, fmt.Sprintf("transport error: %v", out.transport))
			continue
		}
		if out.statusCode == http.StatusCreated && out.order != nil && out.order.Status == "CONFIRMED" {
			confirmed++
		}
	}

	expectedInsufficient := cfg.Buyers - cfg.InitialStock
	if confirmed != cfg.InitialStock {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d CONFIRMED orders, got %d", cfg.InitialStock, confirmed))
	}
	if res.ErrorHistogram["INVENTORY_INSUFFICIENT"] != expectedInsufficient {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d INVENTORY_INSUFFICIENT errors, got %d", expectedInsufficient, res.ErrorHistogram["INVENTORY_INSUFFICIENT"]))
	}

	if cfg.AdminToken != "" {
		settled, invNote := waitForInventorySettlement(ctx, client, cfg.BaseURL, cfg.AdminToken, cfg.Product.SKUID, 0, 0, cfg.SettleTimeout)
		if invNote != "" {
			res.Notes = append(res.Notes, invNote)
		}
		if !settled {
			res.Violations = append(res.Violations, fmt.Sprintf("inventory for %s did not converge to quantity=0, reserved=0 within %s", cfg.Product.SKUID, cfg.SettleTimeout))
		}
	}

	res.Passed = len(res.Violations) == 0
	return res, nil
}

func RunSnapshotMismatchRace(ctx context.Context, cfg SnapshotRaceConfig) (ScenarioResult, error) {
	if cfg.Buyers <= 0 {
		cfg.Buyers = 20
	}
	client := httpClientOrDefault(cfg.HTTPClient)

	outcomes := make([]callOutcome, cfg.Buyers)
	startGate := make(chan struct{})
	var wg sync.WaitGroup
	var adminErr error
	var adminStatus int

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startGate
		payload := cfg.AdminUpdatePayload
		if payload == nil {
			payload = map[string]any{"price": cfg.NewPrice}
		}
		adminStatus, adminErr = putJSON(ctx, client, strings.TrimRight(cfg.BaseURL, "/")+"/admin/products/"+url.PathEscape(cfg.Product.ProductID), cfg.AdminToken, payload)
	}()

	for i := 0; i < cfg.Buyers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startGate
			if idx >= cfg.Buyers/2 {
				time.Sleep(time.Duration(idx-cfg.Buyers/2+1) * 2 * time.Millisecond)
			}
			idemKey := fmt.Sprintf("snap-%s-%d", randomHex(6), idx)
			body := buildOrderPayload("COD", cfg.Product, 1, cfg.Shipping)
			outcomes[idx] = postOrder(ctx, client, cfg.BaseURL, cfg.CustomerToken, idemKey, body)
		}(i)
	}

	close(startGate)
	wg.Wait()

	// Guarantee a post-commit verification probe if all 20 goroutines landed prior to admin commit.
	postCommitProbe := postOrder(
		ctx,
		client,
		cfg.BaseURL,
		cfg.CustomerToken,
		"snap-post-"+randomHex(6),
		buildOrderPayload("COD", cfg.Product, 1, cfg.Shipping),
	)
	allOutcomes := append(outcomes, postCommitProbe)

	res := summarizeOutcomes("snapshot-mismatch-race", cfg.Buyers+1, allOutcomes)
	if adminErr != nil || (adminStatus != http.StatusOK && adminStatus != http.StatusCreated) {
		res.Violations = append(res.Violations, fmt.Sprintf("admin product update failed: status=%d err=%v", adminStatus, adminErr))
	}

	for _, out := range allOutcomes {
		if out.transport != nil {
			res.Violations = append(res.Violations, fmt.Sprintf("transport error: %v", out.transport))
			continue
		}
		if out.statusCode == http.StatusCreated && out.order != nil {
			for _, item := range out.order.Items {
				if item.SKUID == cfg.Product.SKUID && item.UnitPrice != cfg.Product.UnitPrice {
					res.Violations = append(res.Violations, fmt.Sprintf("order %s charged %d instead of snapshot price %d", out.order.OrderID, item.UnitPrice, cfg.Product.UnitPrice))
				}
			}
		} else if out.statusCode == http.StatusConflict {
			if out.errCode != "PRODUCT_SNAPSHOT_MISMATCH" {
				res.Violations = append(res.Violations, fmt.Sprintf("unexpected 409 error code: %s", out.errCode))
			}
		} else {
			res.Violations = append(res.Violations, fmt.Sprintf("unexpected HTTP status: %d (%s)", out.statusCode, out.errCode))
		}
	}

	if postCommitProbe.statusCode != http.StatusConflict || postCommitProbe.errCode != "PRODUCT_SNAPSHOT_MISMATCH" {
		res.Violations = append(res.Violations, fmt.Sprintf("post-commit stale snapshot was not rejected with PRODUCT_SNAPSHOT_MISMATCH (status=%d code=%s)", postCommitProbe.statusCode, postCommitProbe.errCode))
	}

	res.Passed = len(res.Violations) == 0
	return res, nil
}

func RunIdempotencyStorm(ctx context.Context, cfg IdempotencyStormConfig) (ScenarioResult, error) {
	if cfg.SamePayloadWorkers <= 0 {
		cfg.SamePayloadWorkers = 20
	}
	if cfg.MutatedPayloadWorkers <= 0 {
		cfg.MutatedPayloadWorkers = 5
	}
	if strings.TrimSpace(cfg.IdempotencyKey) == "" {
		cfg.IdempotencyKey = "idem-storm-" + randomHex(8)
	}
	client := httpClientOrDefault(cfg.HTTPClient)

	validBody := buildOrderPayload("MOMO", cfg.Product, 1, cfg.Shipping)
	mutatedBody := buildOrderPayload("MOMO", cfg.Product, 2, cfg.Shipping)

	sameOutcomes := make([]callOutcome, cfg.SamePayloadWorkers)
	mutatedOutcomes := make([]callOutcome, cfg.MutatedPayloadWorkers)

	startGate := make(chan struct{})
	var sameWG sync.WaitGroup
	var firstClaimOnce sync.Once
	claimedCh := make(chan struct{})

	for i := 0; i < cfg.SamePayloadWorkers; i++ {
		sameWG.Add(1)
		go func(idx int) {
			defer sameWG.Done()
			<-startGate
			out := postOrder(ctx, client, cfg.BaseURL, cfg.CustomerToken, cfg.IdempotencyKey, validBody)
			firstClaimOnce.Do(func() { close(claimedCh) })
			for retry := 0; retry < 8 && out.statusCode == http.StatusConflict && out.errCode == "REQUEST_IN_PROGRESS"; retry++ {
				time.Sleep(25 * time.Millisecond)
				out = postOrder(ctx, client, cfg.BaseURL, cfg.CustomerToken, cfg.IdempotencyKey, validBody)
			}
			sameOutcomes[idx] = out
		}(i)
	}

	var mutatedWG sync.WaitGroup
	mutatedGate := make(chan struct{})
	for i := 0; i < cfg.MutatedPayloadWorkers; i++ {
		mutatedWG.Add(1)
		go func(idx int) {
			defer mutatedWG.Done()
			<-mutatedGate
			mutatedOutcomes[idx] = postOrder(ctx, client, cfg.BaseURL, cfg.CustomerToken, cfg.IdempotencyKey, mutatedBody)
		}(i)
	}

	close(startGate)
	<-claimedCh
	close(mutatedGate)

	sameWG.Wait()
	mutatedWG.Wait()

	allOutcomes := append(append([]callOutcome{}, sameOutcomes...), mutatedOutcomes...)
	res := summarizeOutcomes("idempotency-storm", cfg.SamePayloadWorkers+cfg.MutatedPayloadWorkers, allOutcomes)

	orderIDs := map[string]struct{}{}
	payURLs := map[string]struct{}{}
	for idx, out := range sameOutcomes {
		if out.transport != nil {
			res.Violations = append(res.Violations, fmt.Sprintf("same-payload worker %d transport error: %v", idx, out.transport))
			continue
		}
		if out.statusCode != http.StatusCreated || out.order == nil {
			res.Violations = append(res.Violations, fmt.Sprintf("same-payload worker %d expected 201 Created, got %d (%s)", idx, out.statusCode, out.errCode))
			continue
		}
		orderIDs[out.order.OrderID] = struct{}{}
		if out.order.PayURL != "" {
			payURLs[out.order.PayURL] = struct{}{}
		}
	}

	if len(orderIDs) != 1 {
		res.Violations = append(res.Violations, fmt.Sprintf("expected exactly 1 unique orderId across %d duplicate requests, got %d", cfg.SamePayloadWorkers, len(orderIDs)))
	}
	if len(payURLs) != 1 {
		res.Violations = append(res.Violations, fmt.Sprintf("expected exactly 1 unique payUrl across %d duplicate requests, got %d", cfg.SamePayloadWorkers, len(payURLs)))
	}

	reusedCount := 0
	for idx, out := range mutatedOutcomes {
		if out.transport != nil {
			res.Violations = append(res.Violations, fmt.Sprintf("mutated worker %d transport error: %v", idx, out.transport))
			continue
		}
		if out.statusCode == http.StatusConflict && out.errCode == "IDEMPOTENCY_KEY_REUSED" {
			reusedCount++
		} else {
			res.Violations = append(res.Violations, fmt.Sprintf("mutated worker %d expected 409 IDEMPOTENCY_KEY_REUSED, got %d (%s)", idx, out.statusCode, out.errCode))
		}
	}
	if reusedCount != cfg.MutatedPayloadWorkers {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d IDEMPOTENCY_KEY_REUSED responses, got %d", cfg.MutatedPayloadWorkers, reusedCount))
	}

	res.Passed = len(res.Violations) == 0
	return res, nil
}

func RunMomoCallbackRace(ctx context.Context, cfg MomoCallbackRaceConfig) (ScenarioResult, error) {
	if cfg.ValidRedirects <= 0 {
		cfg.ValidRedirects = 10
	}
	if cfg.ValidIPNs <= 0 {
		cfg.ValidIPNs = 10
	}
	if cfg.ForgedCallbacks <= 0 {
		cfg.ForgedCallbacks = 5
	}
	if cfg.CancelRequests <= 0 {
		cfg.CancelRequests = 5
	}
	if cfg.SettleTimeout <= 0 {
		cfg.SettleTimeout = 5 * time.Second
	}

	baseClient := httpClientOrDefault(cfg.HTTPClient)
	noRedirectClient := *baseClient
	noRedirectClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	seed := postOrder(
		ctx,
		baseClient,
		cfg.BaseURL,
		cfg.CustomerToken,
		"momo-race-seed-"+randomHex(6),
		buildOrderPayload("MOMO", cfg.Product, 1, cfg.Shipping),
	)
	if seed.transport != nil || seed.statusCode != http.StatusCreated || seed.order == nil {
		return ScenarioResult{}, fmt.Errorf("failed to create seed MOMO order: status=%d code=%s err=%v", seed.statusCode, seed.errCode, seed.transport)
	}
	orderID := seed.order.OrderID
	amount := seed.order.TotalAmount

	totalWorkers := cfg.ValidRedirects + cfg.ValidIPNs + cfg.ForgedCallbacks + cfg.CancelRequests
	outcomes := make([]callOutcome, totalWorkers)
	startGate := make(chan struct{})
	var wg sync.WaitGroup

	validParams := buildSignedMomoParams(cfg.PartnerCode, cfg.AccessKey, cfg.SecretKey, orderID, amount, 0, "9988776655")

	offset := 0
	for i := 0; i < cfg.ValidRedirects; i++ {
		idx := offset + i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			outcomes[idx] = sendMomoRedirect(ctx, &noRedirectClient, cfg.BaseURL, validParams)
		}()
	}
	offset += cfg.ValidRedirects

	for i := 0; i < cfg.ValidIPNs; i++ {
		idx := offset + i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			outcomes[idx] = sendMomoIPN(ctx, baseClient, cfg.BaseURL, validParams)
		}()
	}
	offset += cfg.ValidIPNs

	for i := 0; i < cfg.ForgedCallbacks; i++ {
		idx := offset + i
		forgedCode := 0
		if i%2 == 1 {
			forgedCode = 1006
		}
		forged := buildSignedMomoParams(cfg.PartnerCode, cfg.AccessKey, "forged-wrong-secret-key", orderID, amount, forgedCode, "11111111")
		useIPN := i%2 == 0
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			if useIPN {
				outcomes[idx] = sendMomoIPN(ctx, baseClient, cfg.BaseURL, forged)
			} else {
				outcomes[idx] = sendMomoRedirect(ctx, &noRedirectClient, cfg.BaseURL, forged)
			}
		}()
	}
	offset += cfg.ForgedCallbacks

	for i := 0; i < cfg.CancelRequests; i++ {
		idx := offset + i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			outcomes[idx] = postCancelOrder(ctx, baseClient, cfg.BaseURL, cfg.CustomerToken, orderID)
		}()
	}

	close(startGate)
	wg.Wait()

	res := summarizeOutcomes("momo-callback-race", totalWorkers, outcomes)

	if res.StatusHistogram[http.StatusFound] != cfg.ValidRedirects {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d HTTP 302 valid redirects, got %d", cfg.ValidRedirects, res.StatusHistogram[http.StatusFound]))
	}
	if res.StatusHistogram[http.StatusNoContent] != cfg.ValidIPNs {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d HTTP 204 valid IPNs, got %d", cfg.ValidIPNs, res.StatusHistogram[http.StatusNoContent]))
	}
	if res.ErrorHistogram["INVALID_MOMO_SIGNATURE"] != cfg.ForgedCallbacks {
		res.Violations = append(res.Violations, fmt.Sprintf("expected %d forged callbacks rejected with INVALID_MOMO_SIGNATURE, got %d", cfg.ForgedCallbacks, res.ErrorHistogram["INVALID_MOMO_SIGNATURE"]))
	}

	finalOrder, converged := waitForOrderTerminalConvergence(ctx, baseClient, cfg.BaseURL, cfg.CustomerToken, orderID, cfg.SettleTimeout)
	if !converged || finalOrder == nil {
		res.Violations = append(res.Violations, fmt.Sprintf("order %s did not converge to a valid terminal payment state within %s", orderID, cfg.SettleTimeout))
	} else {
		res.Notes = append(res.Notes, fmt.Sprintf("converged order=%s status=%s paymentStatus=%s", orderID, finalOrder.Status, finalOrder.PaymentStatus))
	}

	res.Passed = len(res.Violations) == 0
	return res, nil
}

func buildOrderPayload(method string, prod SnapshotProduct, qty int, ship ShippingPayload) map[string]any {
	return map[string]any{
		"paymentMethod":   method,
		"shippingAddress": ship,
		"items": []map[string]any{
			{
				"productId":              prod.ProductID,
				"skuId":                  prod.SKUID,
				"quantity":               qty,
				"unitPrice":              prod.UnitPrice,
				"expectedName":           prod.Name,
				"brand":                  prod.Brand,
				"categoryName":           prod.CategoryName,
				"variantLabel":           prod.VariantLabel,
				"imageUrl":               prod.ImageURL,
				"imageEmoji":             prod.ImageEmoji,
				"expectedProductVersion": prod.ProductVersion,
			},
		},
	}
}

func postOrder(ctx context.Context, client *http.Client, baseURL, token, idemKey string, payload map[string]any) callOutcome {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/orders", bytes.NewReader(raw))
	if err != nil {
		return callOutcome{transport: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	started := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return callOutcome{latency: elapsed, transport: err}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	out := callOutcome{
		statusCode: resp.StatusCode,
		latency:    elapsed,
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var env struct {
			Data struct {
				Order  orderEnvelope `json:"order"`
				PayURL string        `json:"payUrl"`
			} `json:"data"`
		}
		if json.Unmarshal(bodyBytes, &env) == nil {
			ord := env.Data.Order
			if ord.PayURL == "" {
				ord.PayURL = env.Data.PayURL
			}
			out.order = &ord
		}
		return out
	}

	var errEnv struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(bodyBytes, &errEnv) == nil {
		out.errCode = errEnv.Code
	}
	return out
}

func postCancelOrder(ctx context.Context, client *http.Client, baseURL, token, orderID string) callOutcome {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/orders/"+url.PathEscape(orderID)+"/cancel", bytes.NewReader([]byte(`{"reason":"sim-race"}`)))
	if err != nil {
		return callOutcome{transport: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	started := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return callOutcome{latency: elapsed, transport: err}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	out := callOutcome{statusCode: resp.StatusCode, latency: elapsed}
	if resp.StatusCode >= 400 {
		var errEnv struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(bodyBytes, &errEnv) == nil {
			out.errCode = errEnv.Code
		}
	}
	return out
}

func putJSON(ctx context.Context, client *http.Client, endpoint, token string, payload any) (int, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func buildSignedMomoParams(partnerCode, accessKey, secretKey, orderID string, amount int64, resultCode int, transID string) map[string]string {
	vals := map[string]string{
		"partnerCode":  partnerCode,
		"orderId":      orderID,
		"requestId":    "req-" + orderID,
		"amount":       strconv.FormatInt(amount, 10),
		"orderInfo":    "Thanh toan don hang " + orderID,
		"orderType":    "momo_wallet",
		"transId":      transID,
		"resultCode":   strconv.Itoa(resultCode),
		"message":      "Success",
		"payType":      "qr",
		"responseTime": strconv.FormatInt(time.Now().UnixMilli(), 10),
		"extraData":    "",
	}
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%s&extraData=%s&message=%s&orderId=%s&orderInfo=%s&orderType=%s&partnerCode=%s&payType=%s&requestId=%s&responseTime=%s&resultCode=%s&transId=%s",
		accessKey,
		vals["amount"],
		vals["extraData"],
		vals["message"],
		vals["orderId"],
		vals["orderInfo"],
		vals["orderType"],
		vals["partnerCode"],
		vals["payType"],
		vals["requestId"],
		vals["responseTime"],
		vals["resultCode"],
		vals["transId"],
	)
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(raw))
	vals["signature"] = hex.EncodeToString(mac.Sum(nil))
	return vals
}

func sendMomoRedirect(ctx context.Context, client *http.Client, baseURL string, params map[string]string) callOutcome {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/payments/momo-redirect?"+q.Encode(), nil)
	if err != nil {
		return callOutcome{transport: err}
	}
	started := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return callOutcome{latency: elapsed, transport: err}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	out := callOutcome{statusCode: resp.StatusCode, latency: elapsed}
	if resp.StatusCode >= 400 {
		var errEnv struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(bodyBytes, &errEnv) == nil {
			out.errCode = errEnv.Code
		}
	}
	return out
}

func sendMomoIPN(ctx context.Context, client *http.Client, baseURL string, params map[string]string) callOutcome {
	amountVal, _ := strconv.ParseInt(params["amount"], 10, 64)
	transVal, _ := strconv.ParseInt(params["transId"], 10, 64)
	resultVal, _ := strconv.Atoi(params["resultCode"])
	respTimeVal, _ := strconv.ParseInt(params["responseTime"], 10, 64)

	body := map[string]any{
		"partnerCode":  params["partnerCode"],
		"orderId":      params["orderId"],
		"requestId":    params["requestId"],
		"amount":       amountVal,
		"orderInfo":    params["orderInfo"],
		"orderType":    params["orderType"],
		"transId":      transVal,
		"resultCode":   resultVal,
		"message":      params["message"],
		"payType":      params["payType"],
		"responseTime": respTimeVal,
		"extraData":    params["extraData"],
		"signature":    params["signature"],
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/payments/momo-ipn", bytes.NewReader(raw))
	if err != nil {
		return callOutcome{transport: err}
	}
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return callOutcome{latency: elapsed, transport: err}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	out := callOutcome{statusCode: resp.StatusCode, latency: elapsed}
	if resp.StatusCode >= 400 {
		var errEnv struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(bodyBytes, &errEnv) == nil {
			out.errCode = errEnv.Code
		}
	}
	return out
}

func waitForInventorySettlement(ctx context.Context, client *http.Client, baseURL, adminToken, skuID string, wantQty, wantReserved int, timeout time.Duration) (bool, string) {
	deadline := time.Now().Add(timeout)
	var lastNote string
	queryURL := strings.TrimRight(baseURL, "/") + "/admin/inventory?q=" + url.QueryEscape(skuID)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
		if err != nil {
			return false, ""
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, err := client.Do(req)
		if err == nil {
			var env struct {
				Data struct {
					Items []struct {
						SkuID    string `json:"skuId"`
						Quantity int    `json:"quantity"`
						Reserved int    `json:"reserved"`
					} `json:"items"`
				} `json:"data"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&env)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				for _, item := range env.Data.Items {
					if item.SkuID != skuID {
						continue
					}
					lastNote = fmt.Sprintf("inventory sku=%s quantity=%d reserved=%d", skuID, item.Quantity, item.Reserved)
					if item.Quantity == wantQty && item.Reserved == wantReserved {
						return true, lastNote
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false, lastNote
}

func waitForOrderTerminalConvergence(ctx context.Context, client *http.Client, baseURL, token, orderID string, timeout time.Duration) (*orderEnvelope, bool) {
	deadline := time.Now().Add(timeout)
	var latest *orderEnvelope
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/orders/"+url.PathEscape(orderID), nil)
		if err != nil {
			return nil, false
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err == nil {
			var env struct {
				Data orderEnvelope `json:"data"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&env)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ord := env.Data
				latest = &ord
				if (ord.Status == "CONFIRMED" && ord.PaymentStatus == "PAID") ||
					(ord.Status == "CANCELLED" && ord.PaymentStatus == "REFUND_REQUIRED") {
					return latest, true
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return latest, false
}

func summarizeOutcomes(name string, goroutines int, outcomes []callOutcome) ScenarioResult {
	statusHist := map[int]int{}
	errHist := map[string]int{}
	latencies := make([]float64, 0, len(outcomes))

	for _, out := range outcomes {
		if out.statusCode > 0 {
			statusHist[out.statusCode]++
		}
		if out.errCode != "" {
			errHist[out.errCode]++
		}
		latencies = append(latencies, float64(out.latency.Microseconds())/1000.0)
	}
	sort.Float64s(latencies)

	return ScenarioResult{
		Scenario:        name,
		Goroutines:      goroutines,
		StatusHistogram: statusHist,
		ErrorHistogram:  errHist,
		P50Ms:           percentile(latencies, 0.50),
		P95Ms:           percentile(latencies, 0.95),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
