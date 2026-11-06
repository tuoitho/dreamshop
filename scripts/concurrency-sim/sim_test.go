package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunFlashSaleOversell_PreventsOversellUnder50ConcurrentBuyers(t *testing.T) {
	var mu sync.Mutex
	stockQty := 5
	reservedQty := 0
	orderSeq := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			mu.Lock()
			defer mu.Unlock()
			if stockQty-reservedQty < 1 {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":    "INVENTORY_INSUFFICIENT",
					"message": "not enough stock",
				})
				return
			}
			// COD confirms and deducts physical stock via OrderConfirmed async settlement.
			stockQty--
			orderSeq++
			orderID := fmt.Sprintf("ord-%d", orderSeq)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"order": map[string]any{
						"orderId":       orderID,
						"orderCode":     fmt.Sprintf("ORD-20261105-%06d", orderSeq),
						"status":        "CONFIRMED",
						"paymentMethod": "COD",
						"paymentStatus": "UNPAID",
						"totalAmount":   1000000,
					},
				},
				"message": "Order created",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/inventory":
			mu.Lock()
			q := stockQty
			res := reservedQty
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"items": []map[string]any{
						{
							"skuId":     "sku-flash-1",
							"quantity":  q,
							"reserved":  res,
							"available": q - res,
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res, err := RunFlashSaleOversell(context.Background(), FlashSaleConfig{
		BaseURL:        srv.URL + "/api/v1",
		HTTPClient:     srv.Client(),
		AdminToken:     "admin-tok",
		CustomerTokens: []string{"cust-tok"},
		InitialStock:   5,
		Buyers:         50,
		SettleTimeout:  2 * time.Second,
		Product: SnapshotProduct{
			ProductID:      "prod-flash-1",
			SKUID:          "sku-flash-1",
			Name:           "Flash Phone",
			VariantLabel:   "Black",
			UnitPrice:      1000000,
			ProductVersion: 1,
		},
		Shipping: defaultTestShipping(),
	})
	if err != nil {
		t.Fatalf("RunFlashSaleOversell returned unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected scenario to pass, violations=%v", res.Violations)
	}
	if res.StatusHistogram[http.StatusCreated] != 5 {
		t.Fatalf("expected 5 HTTP 201 responses, got %d", res.StatusHistogram[http.StatusCreated])
	}
	if res.ErrorHistogram["INVENTORY_INSUFFICIENT"] != 45 {
		t.Fatalf("expected 45 INVENTORY_INSUFFICIENT responses, got %d", res.ErrorHistogram["INVENTORY_INSUFFICIENT"])
	}
}

func TestRunSnapshotMismatchRace_BlocksStalePriceAfterAdminUpdate(t *testing.T) {
	var mu sync.Mutex
	currentVersion := int64(1)
	currentPrice := int64(10000000)
	orderSeq := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/admin/products/prod-race-1":
			var body struct {
				Price int64 `json:"price"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			currentVersion++
			if body.Price > 0 {
				currentPrice = body.Price
			} else {
				currentPrice = 12000000
			}
			ver := currentVersion
			price := currentPrice
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":      "prod-race-1",
					"version": ver,
					"price":   price,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			var req struct {
				Items []struct {
					SKUID                  string `json:"skuId"`
					UnitPrice              int64  `json:"unitPrice"`
					ExpectedProductVersion int64  `json:"expectedProductVersion"`
				} `json:"items"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			item := req.Items[0]

			mu.Lock()
			defer mu.Unlock()
			if item.ExpectedProductVersion != currentVersion || item.UnitPrice != currentPrice {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":    "PRODUCT_SNAPSHOT_MISMATCH",
					"message": "product snapshot changed",
					"errors": map[string]string{
						item.SKUID: "PRICE_CHANGED",
					},
				})
				return
			}
			orderSeq++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"order": map[string]any{
						"orderId":       fmt.Sprintf("ord-snap-%d", orderSeq),
						"orderCode":     fmt.Sprintf("ORD-SNAP-%04d", orderSeq),
						"status":        "CONFIRMED",
						"paymentMethod": "COD",
						"paymentStatus": "UNPAID",
						"totalAmount":   item.UnitPrice,
						"items": []map[string]any{
							{
								"skuId":     item.SKUID,
								"unitPrice": item.UnitPrice,
								"quantity":  1,
							},
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res, err := RunSnapshotMismatchRace(context.Background(), SnapshotRaceConfig{
		BaseURL:       srv.URL + "/api/v1",
		HTTPClient:    srv.Client(),
		AdminToken:    "admin-tok",
		CustomerToken: "cust-tok",
		Buyers:        20,
		NewPrice:      12000000,
		Product: SnapshotProduct{
			ProductID:      "prod-race-1",
			SKUID:          "sku-race-1",
			Name:           "Race Laptop",
			VariantLabel:   "16GB",
			UnitPrice:      10000000,
			ProductVersion: 1,
		},
		AdminUpdatePayload: map[string]any{
			"price": 12000000,
		},
		Shipping: defaultTestShipping(),
	})
	if err != nil {
		t.Fatalf("RunSnapshotMismatchRace returned unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected snapshot race scenario to pass, violations=%v", res.Violations)
	}
	if res.ErrorHistogram["PRODUCT_SNAPSHOT_MISMATCH"] == 0 {
		t.Fatalf("expected at least one PRODUCT_SNAPSHOT_MISMATCH after admin update, got histogram=%v", res.ErrorHistogram)
	}
}

func TestRunIdempotencyStorm_Deduplicates20SameAndRejects5Mutated(t *testing.T) {
	type idemEntry struct {
		qty       int
		status    string
		orderID   string
		payURL    string
		createdAt time.Time
	}

	var mu sync.Mutex
	records := map[string]*idemEntry{}
	createdOrders := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/orders" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		idemKey := r.Header.Get("Idempotency-Key")
		var req struct {
			Items []struct {
				Quantity int `json:"quantity"`
			} `json:"items"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		qty := req.Items[0].Quantity

		mu.Lock()
		existing, exists := records[idemKey]
		if !exists {
			createdOrders++
			entry := &idemEntry{
				qty:       qty,
				status:    "IN_PROGRESS",
				orderID:   fmt.Sprintf("ord-idem-%d", createdOrders),
				payURL:    fmt.Sprintf("https://momo.test/pay/%d", createdOrders),
				createdAt: time.Now(),
			}
			records[idemKey] = entry
			mu.Unlock()

			time.Sleep(25 * time.Millisecond)

			mu.Lock()
			entry.status = "COMPLETED"
			mu.Unlock()

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"order": map[string]any{
						"orderId":       entry.orderID,
						"orderCode":     "ORD-IDEM-000001",
						"status":        "PENDING_PAYMENT",
						"paymentMethod": "MOMO",
						"paymentStatus": "PENDING",
						"totalAmount":   2000000,
						"payUrl":        entry.payURL,
					},
					"payUrl": entry.payURL,
				},
			})
			return
		}

		if existing.qty != qty {
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    "IDEMPOTENCY_KEY_REUSED",
				"message": "idempotency key reused with different payload",
			})
			return
		}
		if existing.status == "IN_PROGRESS" {
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    "REQUEST_IN_PROGRESS",
				"message": "order creation in progress",
			})
			return
		}
		orderID := existing.orderID
		payURL := existing.payURL
		mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"order": map[string]any{
					"orderId":       orderID,
					"orderCode":     "ORD-IDEM-000001",
					"status":        "PENDING_PAYMENT",
					"paymentMethod": "MOMO",
					"paymentStatus": "PENDING",
					"totalAmount":   2000000,
					"payUrl":        payURL,
				},
				"payUrl": payURL,
			},
		})
	}))
	defer srv.Close()

	res, err := RunIdempotencyStorm(context.Background(), IdempotencyStormConfig{
		BaseURL:               srv.URL + "/api/v1",
		HTTPClient:            srv.Client(),
		CustomerToken:         "cust-tok",
		SamePayloadWorkers:    20,
		MutatedPayloadWorkers: 5,
		IdempotencyKey:        "idem-storm-key-1",
		Product: SnapshotProduct{
			ProductID:      "prod-idem-1",
			SKUID:          "sku-idem-1",
			Name:           "Idem Buds",
			VariantLabel:   "White",
			UnitPrice:      2000000,
			ProductVersion: 1,
		},
		Shipping: defaultTestShipping(),
	})
	if err != nil {
		t.Fatalf("RunIdempotencyStorm returned unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected idempotency storm scenario to pass, violations=%v", res.Violations)
	}
	if createdOrders != 1 {
		t.Fatalf("expected exactly 1 underlying order created, got %d", createdOrders)
	}
	if res.ErrorHistogram["IDEMPOTENCY_KEY_REUSED"] != 5 {
		t.Fatalf("expected 5 IDEMPOTENCY_KEY_REUSED responses, got %d", res.ErrorHistogram["IDEMPOTENCY_KEY_REUSED"])
	}
}

func TestRunMomoCallbackRace_RejectsForgedSignaturesAndConvergesConsistently(t *testing.T) {
	const (
		partnerCode = "MOMO_LOCAL"
		accessKey   = "local-access-key"
		secretKey   = "local-secret-key-change-me"
	)

	var mu sync.Mutex
	orderStatus := "PENDING_PAYMENT"
	paymentStatus := "PENDING"
	amount := int64(3500000)
	orderID := "ord-momo-race-1"

	verifySig := func(vals map[string]string) bool {
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
		expected := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(vals["signature"]))
	}

	applyPaid := func() {
		mu.Lock()
		defer mu.Unlock()
		switch orderStatus {
		case "PENDING_PAYMENT":
			orderStatus = "CONFIRMED"
			paymentStatus = "PAID"
		case "CANCELLED":
			paymentStatus = "REFUND_REQUIRED"
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"order": map[string]any{
						"orderId":       orderID,
						"orderCode":     "ORD-MOMO-RACE01",
						"status":        "PENDING_PAYMENT",
						"paymentMethod": "MOMO",
						"paymentStatus": "PENDING",
						"totalAmount":   amount,
						"payUrl":        "https://momo.test/pay/" + orderID,
					},
					"payUrl": "https://momo.test/pay/" + orderID,
				},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/payments/momo-redirect"):
			q := r.URL.Query()
			vals := map[string]string{}
			for k := range q {
				vals[k] = q.Get(k)
			}
			if !verifySig(vals) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":    "INVALID_MOMO_SIGNATURE",
					"message": "invalid signature",
				})
				return
			}
			if vals["resultCode"] == "0" {
				applyPaid()
			}
			http.Redirect(w, r, "http://localhost:5173/thanh-toan/ket-qua?orderId="+url.QueryEscape(vals["orderId"])+"&result=success", http.StatusFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/payments/momo-ipn":
			var rawMap map[string]any
			dec := json.NewDecoder(r.Body)
			dec.UseNumber()
			_ = dec.Decode(&rawMap)
			vals := map[string]string{}
			for k, v := range rawMap {
				vals[k] = fmt.Sprint(v)
			}
			if !verifySig(vals) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":    "INVALID_MOMO_SIGNATURE",
					"message": "invalid signature",
				})
				return
			}
			if vals["resultCode"] == "0" {
				applyPaid()
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders/"+orderID+"/cancel":
			w.Header().Set("Content-Type", "application/json")
			mu.Lock()
			if orderStatus == "PENDING_PAYMENT" {
				orderStatus = "CANCELLED"
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"orderId":       orderID,
						"status":        "CANCELLED",
						"paymentStatus": paymentStatus,
					},
				})
				return
			}
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    "ORDER_NOT_CANCELLABLE",
				"message": "order already terminal",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orders/"+orderID:
			w.Header().Set("Content-Type", "application/json")
			mu.Lock()
			st := orderStatus
			pst := paymentStatus
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"orderId":       orderID,
					"orderCode":     "ORD-MOMO-RACE01",
					"status":        st,
					"paymentMethod": "MOMO",
					"paymentStatus": pst,
					"totalAmount":   amount,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res, err := RunMomoCallbackRace(context.Background(), MomoCallbackRaceConfig{
		BaseURL:         srv.URL + "/api/v1",
		HTTPClient:      srv.Client(),
		CustomerToken:   "cust-tok",
		PartnerCode:     partnerCode,
		AccessKey:       accessKey,
		SecretKey:       secretKey,
		ValidRedirects:  10,
		ValidIPNs:       10,
		ForgedCallbacks: 5,
		CancelRequests:  5,
		SettleTimeout:   2 * time.Second,
		Product: SnapshotProduct{
			ProductID:      "prod-momo-1",
			SKUID:          "sku-momo-1",
			Name:           "MoMo Watch",
			VariantLabel:   "44mm",
			UnitPrice:      amount,
			ProductVersion: 1,
		},
		Shipping: defaultTestShipping(),
	})
	if err != nil {
		t.Fatalf("RunMomoCallbackRace returned unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected MoMo callback race scenario to pass, violations=%v", res.Violations)
	}
	if res.ErrorHistogram["INVALID_MOMO_SIGNATURE"] != 5 {
		t.Fatalf("expected 5 forged callbacks rejected with INVALID_MOMO_SIGNATURE, got %d", res.ErrorHistogram["INVALID_MOMO_SIGNATURE"])
	}
}

func defaultTestShipping() ShippingPayload {
	return ShippingPayload{
		RecipientName: "Nguyễn Văn Test",
		Phone:         "0901234567",
		ProvinceCode:  "79",
		ProvinceName:  "TP. Hồ Chí Minh",
		WardCode:      "26740",
		WardName:      "Phường Bến Nghé",
		AddressLine:   "1 Lê Duẩn",
	}
}

func TestResolveMomoCredentials_LoadsDotenvWithoutOverridingProcessEnv(t *testing.T) {
	tmpDir := t.TempDir()
	dotenvPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(dotenvPath, []byte("MOMO_PARTNER_CODE=DOTENV_PARTNER\nMOMO_ACCESS_KEY=\"dotenv-access\"\nMOMO_SECRET_KEY='dotenv-secret'\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("MOMO_PARTNER_CODE", "")
	t.Setenv("MOMO_ACCESS_KEY", "override-access")
	t.Setenv("MOMO_SECRET_KEY", "")

	env := loadDotenvFallback(dotenvPath)
	if got := resolveEnv(env, "MOMO_PARTNER_CODE", defaultMomoPartnerCode); got != "DOTENV_PARTNER" {
		t.Fatalf("partnerCode = %q, want DOTENV_PARTNER", got)
	}
	if got := resolveEnv(env, "MOMO_ACCESS_KEY", defaultMomoAccessKey); got != "override-access" {
		t.Fatalf("accessKey = %q, want override-access", got)
	}
	if got := resolveEnv(env, "MOMO_SECRET_KEY", defaultMomoSecretKey); got != "dotenv-secret" {
		t.Fatalf("secretKey = %q, want dotenv-secret", got)
	}
}
