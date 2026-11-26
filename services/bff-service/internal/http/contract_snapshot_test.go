package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	orderclient "ecommerce.local/bff-service/internal/order"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	"google.golang.org/grpc/codes"
)

type contractSnapshotFile struct {
	Domain    string                           `json:"domain"`
	Snapshots map[string]contractSnapshotEntry `json:"snapshots"`
}

type contractSnapshotEntry struct {
	Service      string `json:"service"`
	Endpoint     string `json:"endpoint"`
	Status       int    `json:"status"`
	RequestBody  any    `json:"requestBody,omitempty"`
	ResponseBody any    `json:"responseBody"`
}

func resolveContractSnapshotPath(fileName string) string {
	if override := os.Getenv("CONTRACT_SNAPSHOT_PATH"); override != "" {
		return override
	}
	return filepath.Clean(filepath.Join("..", "..", "..", "..", "contracts", "http-snapshots", fileName))
}

func assertOrRecordContractSnapshot(
	t *testing.T,
	snapshotFilePath string,
	domain string,
	snapshotKey string,
	service string,
	endpoint string,
	requestBody any,
	recorder *httptest.ResponseRecorder,
) {
	t.Helper()

	var actualBody any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &actualBody); err != nil {
			t.Fatalf("response body must be valid JSON for %s (%s): %v", snapshotKey, endpoint, err)
		}
	}

	var normalizedReq any
	if requestBody != nil {
		rawReq, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("marshal requestBody for %s: %v", snapshotKey, err)
		}
		if err := json.Unmarshal(rawReq, &normalizedReq); err != nil {
			t.Fatalf("unmarshal normalized requestBody for %s: %v", snapshotKey, err)
		}
	}

	entry := contractSnapshotEntry{
		Service:      service,
		Endpoint:     endpoint,
		Status:       recorder.Code,
		RequestBody:  normalizedReq,
		ResponseBody: actualBody,
	}

	doc := contractSnapshotFile{
		Domain:    domain,
		Snapshots: map[string]contractSnapshotEntry{},
	}

	if os.Getenv("UPDATE_CONTRACT_SNAPSHOTS") == "1" {
		if _, err := os.Stat(snapshotFilePath); err == nil {
			raw, err := os.ReadFile(snapshotFilePath)
			if err != nil {
				t.Fatalf("read snapshot file %s: %v", snapshotFilePath, err)
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parse snapshot file %s: %v", snapshotFilePath, err)
			}
			if doc.Domain == "" {
				doc.Domain = domain
			}
			if doc.Snapshots == nil {
				doc.Snapshots = map[string]contractSnapshotEntry{}
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat snapshot file %s: %v", snapshotFilePath, err)
		}
		doc.Snapshots[snapshotKey] = entry
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatalf("marshal snapshot file: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(snapshotFilePath), 0o755); err != nil {
			t.Fatalf("mkdir snapshot dir: %v", err)
		}
		if err := os.WriteFile(snapshotFilePath, append(out, '\n'), 0o644); err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}
		return
	}

	if err := verifyContractSnapshotEntry(snapshotFilePath, snapshotKey, service, endpoint, requestBody, recorder); err != nil {
		t.Fatal(err)
	}
}

func verifyContractSnapshotEntry(
	snapshotFilePath string,
	snapshotKey string,
	service string,
	endpoint string,
	requestBody any,
	recorder *httptest.ResponseRecorder,
) error {
	var actualBody any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &actualBody); err != nil {
			return fmt.Errorf("response body must be valid JSON for %s (%s): %w", snapshotKey, endpoint, err)
		}
	}

	var normalizedReq any
	if requestBody != nil {
		rawReq, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal requestBody for %s: %w", snapshotKey, err)
		}
		if err := json.Unmarshal(rawReq, &normalizedReq); err != nil {
			return fmt.Errorf("unmarshal normalized requestBody for %s: %w", snapshotKey, err)
		}
	}

	raw, err := os.ReadFile(snapshotFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing contract snapshot file %s for key %q (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)", snapshotFilePath, snapshotKey)
		}
		return fmt.Errorf("read snapshot file %s: %w", snapshotFilePath, err)
	}
	var doc contractSnapshotFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse snapshot file %s: %w", snapshotFilePath, err)
	}

	expected, exists := doc.Snapshots[snapshotKey]
	if !exists {
		return fmt.Errorf("missing contract snapshot key %q in %s (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)", snapshotKey, snapshotFilePath)
	}
	if expected.Service != service {
		return fmt.Errorf("service mismatch for %s: expected %q, got %q", snapshotKey, expected.Service, service)
	}
	if expected.Endpoint != endpoint {
		return fmt.Errorf("endpoint mismatch for %s: expected %q, got %q", snapshotKey, expected.Endpoint, endpoint)
	}
	if expected.Status != recorder.Code {
		return fmt.Errorf("HTTP status mismatch for %s: expected %d, got %d", snapshotKey, expected.Status, recorder.Code)
	}
	if err := compareContractShape(expected.ResponseBody, actualBody, snapshotKey+".responseBody"); err != nil {
		return err
	}
	if expected.RequestBody != nil || normalizedReq != nil {
		if err := compareContractShape(expected.RequestBody, normalizedReq, snapshotKey+".requestBody"); err != nil {
			return err
		}
	}
	return nil
}

func compareContractShape(expected any, actual any, path string) error {
	expectedShape := extractJSONShape(expected)
	actualShape := extractJSONShape(actual)
	if !reflect.DeepEqual(expectedShape, actualShape) {
		return fmt.Errorf("contract shape drift at %s:\nexpected shape: %#v\nactual shape:   %#v", path, expectedShape, actualShape)
	}
	return nil
}

func extractJSONShape(v any) any {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			out[k] = extractJSONShape(child)
		}
		return out
	case []any:
		if len(val) == 0 {
			return []any{}
		}
		seen := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for _, child := range val {
			childShape := extractJSONShape(child)
			canonicalBytes, err := json.Marshal(childShape)
			canonical := string(canonicalBytes)
			if err != nil {
				canonical = reflect.TypeOf(childShape).String()
			}
			if _, ok := seen[canonical]; !ok {
				seen[canonical] = childShape
				keys = append(keys, canonical)
			}
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, seen[k])
		}
		return out
	case string:
		return "string"
	case float64, int, int32, int64, uint32, uint64:
		return "number"
	case bool:
		return "boolean"
	default:
		return reflect.TypeOf(v).String()
	}
}

func TestExtractJSONShapeNormalizesArrayLengthWithoutHidingDrift(t *testing.T) {
	oneItem := map[string]any{
		"items": []any{map[string]any{"id": "a", "availableQuantity": float64(5)}},
	}
	twoItems := map[string]any{
		"items": []any{
			map[string]any{"id": "a", "availableQuantity": float64(5)},
			map[string]any{"id": "b", "availableQuantity": float64(9)},
		},
	}
	missingField := map[string]any{
		"items": []any{map[string]any{"id": "a"}},
	}
	emptyArr := map[string]any{"items": []any{}}

	if !reflect.DeepEqual(extractJSONShape(oneItem), extractJSONShape(twoItems)) {
		t.Fatalf("expected 1-item and 2-item arrays with identical schema to share shape")
	}
	if reflect.DeepEqual(extractJSONShape(oneItem), extractJSONShape(missingField)) {
		t.Fatalf("expected missing field drift inside array element to differ")
	}
	if reflect.DeepEqual(extractJSONShape(oneItem), extractJSONShape(emptyArr)) {
		t.Fatalf("expected empty array vs non-empty array shape to differ")
	}
}

func TestVerifyContractSnapshotFailsOnMissingFileMissingKeyAndDrift(t *testing.T) {
	t.Setenv("UPDATE_CONTRACT_SNAPSHOTS", "")
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "missing.json")

	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.WriteString(`{"data":{"id":"1"},"message":"ok"}`)

	if err := verifyContractSnapshotEntry(missingPath, "sample_200", "bff-service", "GET /api/v1/sample", nil, rec); err == nil {
		t.Fatal("expected error when snapshot file is missing without UPDATE_CONTRACT_SNAPSHOTS=1")
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("snapshot file must not be auto-created when UPDATE_CONTRACT_SNAPSHOTS is unset")
	}

	validPath := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(validPath, []byte(`{"domain":"sample","snapshots":{"sample_200":{"service":"bff-service","endpoint":"GET /api/v1/sample","status":200,"responseBody":{"data":{"id":"1"},"message":"ok"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := verifyContractSnapshotEntry(validPath, "other_key", "bff-service", "GET /api/v1/sample", nil, rec); err == nil {
		t.Fatal("expected error when snapshot key is missing without UPDATE_CONTRACT_SNAPSHOTS=1")
	}

	drifted := httptest.NewRecorder()
	drifted.WriteHeader(http.StatusOK)
	_, _ = drifted.WriteString(`{"data":{"id":123,"extra":true},"message":"ok"}`)
	if err := verifyContractSnapshotEntry(validPath, "sample_200", "bff-service", "GET /api/v1/sample", nil, drifted); err == nil {
		t.Fatal("expected error on response shape drift")
	}

	if err := verifyContractSnapshotEntry(validPath, "sample_200", "bff-service", "GET /api/v1/sample", nil, rec); err != nil {
		t.Fatalf("expected matching snapshot to pass, got: %v", err)
	}
}

func TestContractSnapshotsBFFStorefrontDomain(t *testing.T) {
	snapshotPath := resolveContractSnapshotPath("bff-storefront.json")

	cat := &fakeCatalogClient{
		searchFn: func(_ context.Context, _ *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
			second := sampleSummary()
			second.Id = "lt-1"
			second.Name = "Laptop AeroBook Pro 14"
			second.Brand = "Aero"
			second.CategoryId = "laptop"
			second.CategoryName = "Laptop"
			second.SkuIds = []string{"lt-1-v1"}
			second.Badge = "new"
			second.ImageHue = []uint32{200, 240}
			return &catalogv1.SearchProductsResponse{
				Items:      []*catalogv1.ProductSummary{sampleSummary(), second},
				Page:       1,
				PageSize:   20,
				TotalItems: 2,
				TotalPages: 1,
			}, nil
		},
		getProductFn: func(_ context.Context, id string) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{
				Id:            id,
				Name:          "Điện thoại Nova X",
				Brand:         "Nova",
				CategoryId:    "dien-thoai",
				CategoryName:  "Điện thoại",
				Description:   "Màn hình OLED 120Hz",
				Price:         money(18990000),
				OriginalPrice: money(21990000),
				MaxPrice:      money(20990000),
				Rating:        4.9,
				ReviewCount:   1284,
				SoldCount:     2310,
				Badge:         "hot",
				Featured:      true,
				Emoji:         "📱",
				ImageHue:      []uint32{220, 260},
				Images:        []string{"https://picsum.photos/seed/dt-1-1/800/800"},
				Variants: []*catalogv1.ProductVariant{
					{SkuId: "dt-1-v1", Label: "Đen 256GB", ColorHex: "#111111", Price: money(18990000), OriginalPrice: money(21990000)},
				},
			}, nil
		},
	}
	inv := &fakeInventoryClient{}
	cartClient := &fakeCartClient{
		getFn: func(_ context.Context, _ string) (*cartv1.GetCartResponse, error) {
			return cartResponse(cartItemFixture()), nil
		},
		getFavoritesFn: func(_ context.Context, _ string) (*cartv1.GetFavoritesResponse, error) {
			return &cartv1.GetFavoritesResponse{
				Favorites: &cartv1.FavoriteList{
					Items:    []*cartv1.FavoriteItem{favoriteItemFixture()},
					Degraded: false,
				},
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewProductsHandler(cat, inv).Register(mux)
	NewCartHandler(cartClient).Register(mux)
	NewFavoritesHandler(cartClient).Register(mux)

	t.Run("storefront_products_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products?page=1&pageSize=20", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-storefront", "storefront_products_200",
			"bff-service", "GET /api/v1/storefront/products", nil, rec)
	})

	t.Run("storefront_product_detail_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products/dt-1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-storefront", "storefront_product_detail_200",
			"bff-service", "GET /api/v1/storefront/products/{id}", nil, rec)
	})

	t.Run("cart_get_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-storefront", "cart_get_200",
			"bff-service", "GET /api/v1/cart", nil, rec)
	})

	t.Run("favorites_get_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/favorites", nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-storefront", "favorites_get_200",
			"bff-service", "GET /api/v1/favorites", nil, rec)
	})

	t.Run("bff_unauthenticated_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-storefront", "bff_unauthenticated_401",
			"bff-service", "GET /api/v1/cart", nil, rec)
	})
}

func TestContractSnapshotsBFFOrdersDomain(t *testing.T) {
	snapshotPath := resolveContractSnapshotPath("bff-orders.json")

	customerDetailWithInternalRefund := func() *orderv1.OrderDetail {
		d := orderDetailFixture()
		d.Status = "CANCELLED"
		d.PaymentStatus = "REFUNDED"
		d.RefundTransId = "MOMO-REF-9001"
		d.RefundedAt = "2026-11-05T10:30:00Z"
		d.RefundLastError = "internal gateway diagnostic"
		d.RefundActorType = "ADMIN"
		d.RefundReason = "internal refund audit note"
		d.StatusHistory = []*orderv1.OrderStatusChange{
			{FromStatus: "PENDING_PAYMENT", ToStatus: "CANCELLED", ActorType: "SYSTEM", CreatedAt: "2026-11-05T10:25:00Z"},
		}
		return d
	}

	mux := http.NewServeMux()
	NewOrdersHandler(&fakeOrderClient{
		createFn: func(_ context.Context, _ *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			d := orderDetailFixture()
			return &orderv1.CreateOrderResponse{
				OrderId:     d.GetOrderId(),
				Status:      d.GetStatus(),
				TotalAmount: d.GetTotalAmount(),
				Order:       d,
				PayUrl:      d.GetPayUrl(),
			}, nil
		},
		listFn: func(_ context.Context, _, _ string, page, pageSize int32) (*orderv1.ListOrdersResponse, error) {
			d := customerDetailWithInternalRefund()
			d.StatusHistory = nil
			return &orderv1.ListOrdersResponse{
				Items:      []*orderv1.OrderDetail{d},
				Page:       page,
				PageSize:   pageSize,
				TotalItems: 1,
				TotalPages: 1,
			}, nil
		},
		getFn: func(_ context.Context, _, orderID string) (*orderv1.GetOrderResponse, error) {
			if orderID == "missing" {
				return nil, &orderclient.ServiceError{GRPCStatus: codes.NotFound, Reason: "ORDER_NOT_FOUND", Message: "Không tìm thấy đơn hàng"}
			}
			return &orderv1.GetOrderResponse{Order: customerDetailWithInternalRefund()}, nil
		},
	}).Register(mux)

	t.Run("orders_create_201", func(t *testing.T) {
		createReqBody := map[string]any{
			"paymentMethod": "MOMO",
			"shippingAddress": map[string]any{
				"recipientName": "Nguyễn Văn A",
				"phone":         "0901234567",
				"provinceCode":  "79",
				"provinceName":  "TP. Hồ Chí Minh",
				"wardCode":      "26740",
				"wardName":      "Phường Bến Nghé",
				"addressLine":   "1 Lê Duẩn",
				"label":         "Nhà riêng",
			},
			"items": []map[string]any{
				{
					"cartItemId":             "ci-1",
					"productId":              "dt-1",
					"skuId":                  "dt-1-v1",
					"quantity":               2,
					"unitPrice":              150000,
					"expectedName":           "Nova X15",
					"variantLabel":           "Đen 256GB",
					"expectedProductVersion": 3,
				},
			},
		}
		rawReq, err := json.Marshal(createReqBody)
		if err != nil {
			t.Fatalf("marshal create order req: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(string(rawReq)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-Id", "user-1")
		req.Header.Set("Idempotency-Key", "idem-snapshot-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-orders", "orders_create_201",
			"bff-service", "POST /api/v1/orders", createReqBody, rec)
	})

	t.Run("orders_list_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?page=1&pageSize=10", nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		raw := rec.Body.String()
		for _, forbidden := range []string{"refundLastError", "refundActorType", "refundReason"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("customer orders list leaked internal field %q: %s", forbidden, raw)
			}
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-orders", "orders_list_200",
			"bff-service", "GET /api/v1/orders", nil, rec)
	})

	t.Run("orders_detail_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/ord-123", nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		raw := rec.Body.String()
		for _, forbidden := range []string{"refundLastError", "refundActorType", "refundReason"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("customer order detail leaked internal field %q: %s", forbidden, raw)
			}
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-orders", "orders_detail_200",
			"bff-service", "GET /api/v1/orders/{id}", nil, rec)
	})

	t.Run("orders_detail_not_found_404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/missing", nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-orders", "orders_detail_not_found_404",
			"bff-service", "GET /api/v1/orders/{id}", nil, rec)
	})
}

func TestContractSnapshotsBFFAdminDomain(t *testing.T) {
	snapshotPath := resolveContractSnapshotPath("bff-admin.json")

	cat := &fakeCatalogClient{
		listAdminFn: func(_ context.Context, _ *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error) {
			return &catalogv1.ListAdminProductsResponse{
				Items: []*catalogv1.AdminProductSummary{
					{
						Id:           "dt-1",
						Name:         "Điện thoại Nova X",
						Brand:        "Nova",
						CategoryId:   "dien-thoai",
						CategoryName: "Điện thoại",
						Price:        money(18990000),
						MaxPrice:     money(20990000),
						SkuIds:       []string{"dt-1-v1"},
						VariantCount: 1,
						VideoCount:   0,
					},
				},
				Page:         1,
				PageSize:     20,
				TotalItems:   1,
				TotalPages:   1,
				ActiveCount:  1,
				DeletedCount: 0,
				Categories: []*catalogv1.CategoryMetric{
					{CategoryId: "dien-thoai", CategoryName: "Điện thoại", ActiveProductCount: 1},
				},
			}, nil
		},
		getAdminProductFn: func(_ context.Context, _, id string) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{
				Id:            id,
				Name:          "Điện thoại Nova X",
				Brand:         "Nova",
				CategoryId:    "dien-thoai",
				CategoryName:  "Điện thoại",
				Description:   "Màn hình OLED 120Hz",
				Price:         money(18990000),
				OriginalPrice: money(21990000),
				MaxPrice:      money(20990000),
				Rating:        4.9,
				ReviewCount:   1284,
				SoldCount:     2310,
				Badge:         "hot",
				Featured:      true,
				Emoji:         "📱",
				ImageHue:      []uint32{220, 260},
				Images:        []string{"https://picsum.photos/seed/dt-1-1/800/800"},
				Variants: []*catalogv1.ProductVariant{
					{SkuId: "dt-1-v1", Label: "Đen 256GB", ColorHex: "#111111", Price: money(18990000), OriginalPrice: money(21990000)},
				},
			}, nil
		},
		createMediaFn: func(_ context.Context, _ *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
			return &catalogv1.CreateProductMediaUploadResponse{
				UploadId:         "u-123",
				MediaType:        "IMAGE",
				UploadUrl:        "http://localhost:9000/product-media/products/2026/11/hero.png?X-Amz-Signature=mock",
				PublicUrl:        "http://localhost:9000/product-media/products/2026/11/hero.png",
				ObjectKey:        "products/2026/11/hero.png",
				ExpiresInSeconds: 300,
			}, nil
		},
		listInventoryFn: func(_ context.Context, _ *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
			return &catalogv1.ListInventoryCatalogItemsResponse{
				Items: []*catalogv1.InventoryCatalogItem{
					{
						Id: "dt-1", Name: "Nova X15", Brand: "Nova", CategoryId: "dien-thoai", CategoryName: "Điện thoại",
						Variants: []*catalogv1.ProductVariant{
							{SkuId: "dt-1-v1", Label: "Đen 256GB", ColorHex: "#111827", Price: money(18990000), OriginalPrice: money(21990000)},
						},
					},
				},
			}, nil
		},
	}
	inv := &fakeInventoryClient{
		batchGetFn: func(_ context.Context, _ string, _ []string) ([]*inventoryv1.StockItem, error) {
			return []*inventoryv1.StockItem{
				{SkuId: "dt-1-v1", ProductId: "dt-1", Quantity: 20, Reserved: 2, AvailableQuantity: 18},
			}, nil
		},
		listStockFn: func(_ context.Context, _ string) ([]*inventoryv1.StockItem, error) {
			return []*inventoryv1.StockItem{
				{SkuId: "dt-1-v1", ProductId: "dt-1", Quantity: 20, Reserved: 2, AvailableQuantity: 18},
			}, nil
		},
	}

	adminOrderFixture := func() *orderv1.OrderDetail {
		d := orderDetailFixture()
		d.Status = "CANCELLED"
		d.PaymentStatus = "REFUND_FAILED"
		d.RefundLastError = "momo timeout 504"
		d.RefundActorType = "ADMIN"
		d.RefundReason = "manual retry requested"
		d.RefundTransId = "MOMO-REF-9001"
		d.RefundedAt = "2026-11-05T10:30:00Z"
		d.StatusHistory = []*orderv1.OrderStatusChange{
			{FromStatus: "CONFIRMED", ToStatus: "CANCELLED", ActorType: "ADMIN", CreatedAt: "2026-11-05T10:25:00Z"},
		}
		return d
	}

	ord := &fakeOrderClient{
		adminListFn: func(_ context.Context, _, _, _ string, page, pageSize int32) (*orderv1.AdminListOrdersResponse, error) {
			return &orderv1.AdminListOrdersResponse{
				Items:        []*orderv1.OrderDetail{adminOrderFixture()},
				Page:         page,
				PageSize:     pageSize,
				TotalItems:   1,
				TotalPages:   1,
				StatusCounts: map[string]int64{"CANCELLED": 1},
			}, nil
		},
		adminGetFn: func(_ context.Context, _, _ string) (*orderv1.AdminGetOrderResponse, error) {
			return &orderv1.AdminGetOrderResponse{
				Order:               adminOrderFixture(),
				AllowedNextStatuses: []string{},
			}, nil
		},
		adminRetryRefundFn: func(_ context.Context, req *orderv1.AdminRetryRefundRequest) (*orderv1.AdminRetryRefundResponse, error) {
			refunded := adminOrderFixture()
			refunded.PaymentStatus = "REFUNDED"
			refunded.RefundLastError = ""
			refunded.RefundActorType = "ADMIN"
			refunded.RefundReason = req.GetReason()
			refunded.RefundTransId = "MOMO-REF-9002"
			refunded.RefundedAt = "2026-11-26T10:30:00Z"
			return &orderv1.AdminRetryRefundResponse{
				Order:               refunded,
				AllowedNextStatuses: []string{},
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewAdminProductsHandler(cat, inv).Register(mux)
	NewAdminInventoryHandler(cat, inv).Register(mux)
	NewAdminOrdersHandler(ord).Register(mux)

	t.Run("admin_products_list_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products?page=1&pageSize=20", nil)
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_products_list_200",
			"bff-service", "GET /api/v1/admin/products", nil, rec)
	})

	t.Run("admin_product_detail_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/dt-1", nil)
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeBody(t, rec)["data"].(map[string]any)
		if _, leaked := data["status"]; leaked {
			t.Fatalf("admin product detail must not emit storefront status field: %s", rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_product_detail_200",
			"bff-service", "GET /api/v1/admin/products/{id}", nil, rec)
	})

	t.Run("admin_product_media_presign_200", func(t *testing.T) {
		presignReq := map[string]any{
			"filename":    "hero.png",
			"contentType": "image/png",
			"sizeBytes":   4096,
		}
		rawReq, err := json.Marshal(presignReq)
		if err != nil {
			t.Fatalf("marshal presign req: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/media/presign", strings.NewReader(string(rawReq)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_product_media_presign_200",
			"bff-service", "POST /api/v1/admin/products/media/presign", presignReq, rec)
	})

	t.Run("admin_inventory_list_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory", nil)
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_inventory_list_200",
			"bff-service", "GET /api/v1/admin/inventory", nil, rec)
	})

	t.Run("admin_orders_list_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?page=1&pageSize=20", nil)
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_orders_list_200",
			"bff-service", "GET /api/v1/admin/orders", nil, rec)
	})

	t.Run("admin_order_detail_200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/ord-123", nil)
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_order_detail_200",
			"bff-service", "GET /api/v1/admin/orders/{id}", nil, rec)
	})

	t.Run("admin_order_retry_refund_200", func(t *testing.T) {
		retryReq := map[string]any{
			"reason": "MoMo đã hoạt động trở lại",
		}
		rawReq, err := json.Marshal(retryReq)
		if err != nil {
			t.Fatalf("marshal retry refund req: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/ord-123/refund", strings.NewReader(string(rawReq)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-Id", "admin-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertOrRecordContractSnapshot(t, snapshotPath, "bff-admin", "admin_order_retry_refund_200",
			"bff-service", "POST /api/v1/admin/orders/{id}/refund", retryReq, rec)
	})
}
