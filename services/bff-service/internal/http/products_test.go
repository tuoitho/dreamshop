package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
)

// fakeCatalogClient is a test double for catalog.Client.
type fakeCatalogClient struct {
	searchFn     func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error)
	getProductFn func(ctx context.Context, id string) (*catalogv1.GetProductResponse, error)
	softDeleteFn func(ctx context.Context, userID, id string, deleted bool) error
	searchCalls  int
	lastSearch   *catalogv1.SearchProductsRequest
}

func (f *fakeCatalogClient) SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
	f.searchCalls++
	f.lastSearch = req
	if f.searchFn != nil {
		return f.searchFn(ctx, req)
	}
	return &catalogv1.SearchProductsResponse{}, nil
}

func (f *fakeCatalogClient) GetProduct(ctx context.Context, id string) (*catalogv1.GetProductResponse, error) {
	if f.getProductFn != nil {
		return f.getProductFn(ctx, id)
	}
	return nil, errors.New("unexpected GetProduct call")
}

func (f *fakeCatalogClient) SoftDeleteProduct(ctx context.Context, userID, id string, deleted bool) error {
	if f.softDeleteFn != nil {
		return f.softDeleteFn(ctx, userID, id, deleted)
	}
	return errors.New("unexpected SoftDeleteProduct call")
}

func money(amount int64) *commonv1.Money {
	return &commonv1.Money{Currency: "VND", AmountMinor: amount}
}

func sampleSummary() *catalogv1.ProductSummary {
	return &catalogv1.ProductSummary{
		Id:            "dt-1",
		Name:          "Điện thoại Nova X",
		Brand:         "Nova",
		CategoryId:    "dien-thoai",
		CategoryName:  "Điện thoại",
		Price:         money(18990000),
		OriginalPrice: money(21990000),
		Images:        []string{"https://picsum.photos/seed/dt-1-1/800/800"},
		Rating:        4.9,
		ReviewCount:   1284,
		SoldCount:     2310,
		IsNew:         false,
		Badge:         "hot",
		Featured:      true,
		Emoji:         "📱",
		ImageHue:      []uint32{220, 260},
	}
}

type fakeInventoryClient struct {
	mu      sync.Mutex
	getFn   func(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error)
	called  []string
	traceID string
}

func (f *fakeInventoryClient) GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
	f.mu.Lock()
	f.called = append(f.called, productID)
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			f.traceID = vals[0]
		}
	}
	f.mu.Unlock()
	if f.getFn != nil {
		return f.getFn(ctx, productID)
	}
	return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: 20}, nil
}

func newTestMux(t *testing.T, client catalog.Client) http.Handler {
	t.Helper()
	return newTestMuxWithInventory(t, client, &fakeInventoryClient{})
}

func newTestMuxWithInventory(t *testing.T, client catalog.Client, invClient inventory.Client) http.Handler {
	t.Helper()
	h := NewProductsHandler(client, invClient)
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

func doRequest(h http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	return body
}

var hexTraceID = regexp.MustCompile(`^[0-9a-f]{16}$`)

func TestSearchProductsHappyPathEnvelope(t *testing.T) {
	client := &fakeCatalogClient{
		searchFn: func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
			return &catalogv1.SearchProductsResponse{
				Items:      []*catalogv1.ProductSummary{sampleSummary()},
				Page:       1,
				PageSize:   20,
				TotalItems: 1,
				TotalPages: 1,
			}, nil
		},
	}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	want := `{"data":{"items":[{"id":"dt-1","name":"Điện thoại Nova X","brand":"Nova","categoryId":"dien-thoai","categoryName":"Điện thoại","price":18990000,"originalPrice":21990000,"rating":4.9,"reviewCount":1284,"soldCount":2310,"images":["https://picsum.photos/seed/dt-1-1/800/800"],"emoji":"📱","imageHue":[220,260],"isNew":false,"badge":"hot","featured":true,"status":"AVAILABLE","availableQuantity":20}],"page":1,"pageSize":20,"totalItems":1,"totalPages":1},"message":"Products retrieved."}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected body\n got: %s\nwant: %s", got, want)
	}
}

func TestSearchProductsEmptyItemsIsArray(t *testing.T) {
	client := &fakeCatalogClient{}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	if items, ok := data["items"].([]any); !ok || items == nil || len(items) != 0 {
		t.Fatalf("expected items to be an empty array, got %#v", data["items"])
	}
}

func TestSearchProductsMapsQueryParams(t *testing.T) {
	cases := []struct {
		name   string
		target string
		check  func(t *testing.T, req *catalogv1.SearchProductsRequest)
	}{
		{
			name:   "all params",
			target: "/api/v1/storefront/products?q=iphone&danh-muc=dien-thoai&gia=500000-2000000&giam-gia=1&danh-gia=4&sap-xep=gia-cao&page=3&pageSize=25",
			check: func(t *testing.T, req *catalogv1.SearchProductsRequest) {
				if req.GetQuery() != "iphone" {
					t.Errorf("expected query iphone, got %q", req.GetQuery())
				}
				if req.GetCategoryId() != "dien-thoai" {
					t.Errorf("expected categoryId dien-thoai, got %q", req.GetCategoryId())
				}
				if req.GetPriceMin() != 500000 || req.GetPriceMax() != 2000000 {
					t.Errorf("expected price 500000-2000000, got %d-%d", req.GetPriceMin(), req.GetPriceMax())
				}
				if !req.GetOnSale() {
					t.Error("expected onSale true")
				}
				if req.GetRatingMin() != 4 {
					t.Errorf("expected ratingMin 4, got %d", req.GetRatingMin())
				}
				if req.GetSort() != "gia-cao" {
					t.Errorf("expected sort gia-cao, got %q", req.GetSort())
				}
				if req.GetPage() != 3 {
					t.Errorf("expected page 3, got %d", req.GetPage())
				}
				if req.GetPageSize() != 25 {
					t.Errorf("expected pageSize 25, got %d", req.GetPageSize())
				}
			},
		},
		{
			name:   "unbounded price max",
			target: "/api/v1/storefront/products?gia=10000000-0",
			check: func(t *testing.T, req *catalogv1.SearchProductsRequest) {
				if req.GetPriceMin() != 10000000 {
					t.Errorf("expected priceMin 10000000, got %d", req.GetPriceMin())
				}
				if req.GetPriceMax() != 0 {
					t.Errorf("expected priceMax 0 (unbounded), got %d", req.GetPriceMax())
				}
			},
		},
		{
			name:   "defaults",
			target: "/api/v1/storefront/products",
			check: func(t *testing.T, req *catalogv1.SearchProductsRequest) {
				if req.GetPage() != 1 {
					t.Errorf("expected default page 1, got %d", req.GetPage())
				}
				if req.GetPageSize() != 20 {
					t.Errorf("expected default pageSize 20, got %d", req.GetPageSize())
				}
				if req.GetQuery() != "" || req.GetCategoryId() != "" || req.GetOnSale() {
					t.Errorf("expected empty optional params, got query=%q category=%q onSale=%v", req.GetQuery(), req.GetCategoryId(), req.GetOnSale())
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCatalogClient{}
			mux := newTestMux(t, client)

			rec := doRequest(mux, http.MethodGet, tc.target)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if client.searchCalls != 1 {
				t.Fatalf("expected exactly 1 search call, got %d", client.searchCalls)
			}
			tc.check(t, client.lastSearch)
		})
	}
}

func TestSearchProductsInvalidParams(t *testing.T) {
	cases := []struct {
		name   string
		target string
		field  string
	}{
		{"bad gia format", "/api/v1/storefront/products?gia=abc-def", "gia"},
		{"gia missing max", "/api/v1/storefront/products?gia=500-", "gia"},
		{"negative gia min", "/api/v1/storefront/products?gia=-100-500", "gia"},
		{"negative gia max", "/api/v1/storefront/products?gia=500--100", "gia"},
		{"extra dash in gia", "/api/v1/storefront/products?gia=1-2-3", "gia"},
		{"page zero", "/api/v1/storefront/products?page=0", "page"},
		{"page negative", "/api/v1/storefront/products?page=-1", "page"},
		{"page non-numeric", "/api/v1/storefront/products?page=abc", "page"},
		{"pageSize zero", "/api/v1/storefront/products?pageSize=0", "pageSize"},
		{"pageSize too large", "/api/v1/storefront/products?pageSize=51", "pageSize"},
		{"pageSize non-numeric", "/api/v1/storefront/products?pageSize=abc", "pageSize"},
		{"rating too low", "/api/v1/storefront/products?danh-gia=0", "danh-gia"},
		{"rating too high", "/api/v1/storefront/products?danh-gia=6", "danh-gia"},
		{"rating non-numeric", "/api/v1/storefront/products?danh-gia=abc", "danh-gia"},
		{"sort invalid", "/api/v1/storefront/products?sap-xep=bogus", "sap-xep"},
		{"giam-gia invalid", "/api/v1/storefront/products?giam-gia=2", "giam-gia"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCatalogClient{}
			mux := newTestMux(t, client)

			rec := doRequest(mux, http.MethodGet, tc.target)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["code"] != "INVALID_PARAMETER" {
				t.Errorf("expected code INVALID_PARAMETER, got %v", body["code"])
			}
			fields, ok := body["errors"].(map[string]any)
			if !ok {
				t.Fatalf("expected errors object, got %#v", body["errors"])
			}
			if _, ok := fields[tc.field]; !ok {
				t.Errorf("expected field %q in errors, got %#v", tc.field, fields)
			}
			if traceID, ok := body["traceId"].(string); !ok || traceID == "" {
				t.Errorf("expected non-empty traceId, got %#v", body["traceId"])
			}
			if client.searchCalls != 0 {
				t.Errorf("expected catalog not called on invalid params, got %d calls", client.searchCalls)
			}
		})
	}
}

func TestSearchProductsCatalogUnavailable(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"grpc unavailable", status.Error(codes.Unavailable, "catalog down")},
		{"generic error", errors.New("boom")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCatalogClient{
				searchFn: func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
					return nil, tc.err
				},
			}
			mux := newTestMux(t, client)

			rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products")

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected status 502, got %d: %s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["code"] != "CATALOG_UNAVAILABLE" {
				t.Errorf("expected code CATALOG_UNAVAILABLE, got %v", body["code"])
			}
			if _, exists := body["errors"]; exists {
				t.Errorf("expected no errors field, got %#v", body["errors"])
			}
		})
	}
}

func TestSearchProductsGRPCInvalidArgument(t *testing.T) {
	client := &fakeCatalogClient{
		searchFn: func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "bad page")
		},
	}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != "INVALID_PARAMETER" {
		t.Errorf("expected code INVALID_PARAMETER, got %v", body["code"])
	}
}

func TestGetProductHappyPathEnvelope(t *testing.T) {
	detail := &catalogv1.GetProductResponse{
		Id:               "dt-1",
		Name:             "Điện thoại Nova X",
		Description:      "Mô tả chi tiết sản phẩm.",
		Price:            money(18990000),
		ShortDescription: "Mô tả ngắn gọn.",
		Brand:            "Nova",
		CategoryId:       "dien-thoai",
		CategoryName:     "Điện thoại",
		OriginalPrice:    money(21990000),
		Images:           []string{"https://picsum.photos/seed/dt-1-1/800/800"},
		Rating:           4.9,
		ReviewCount:      1284,
		SoldCount:        2310,
		IsNew:            false,
		Badge:            "hot",
		Featured:         true,
		Emoji:            "📱",
		ImageHue:         []uint32{220, 260},
		Variants:         []*catalogv1.ProductVariant{{Label: "Đen", ColorHex: "#1f2937"}},
		Deleted:          true,
		Details:          []string{"Hỗ trợ 5G", "Pin 5000 mAh"},
	}
	client := &fakeCatalogClient{
		getProductFn: func(ctx context.Context, id string) (*catalogv1.GetProductResponse, error) {
			if id != "dt-1" {
				t.Errorf("expected id dt-1, got %q", id)
			}
			return detail, nil
		},
	}
	inv := &fakeInventoryClient{}
	mux := newTestMuxWithInventory(t, client, inv)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products/dt-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(inv.called) != 0 {
		t.Fatalf("expected inventory lookup skipped for soft-deleted product, got %v", inv.called)
	}

	want := `{"data":{"id":"dt-1","name":"Điện thoại Nova X","brand":"Nova","categoryId":"dien-thoai","categoryName":"Điện thoại","price":18990000,"originalPrice":21990000,"rating":4.9,"reviewCount":1284,"soldCount":2310,"images":["https://picsum.photos/seed/dt-1-1/800/800"],"emoji":"📱","imageHue":[220,260],"isNew":false,"badge":"hot","featured":true,"status":"UNAVAILABLE","availableQuantity":0,"shortDescription":"Mô tả ngắn gọn.","description":"Mô tả chi tiết sản phẩm.","details":["Hỗ trợ 5G","Pin 5000 mAh"],"variants":[{"label":"Đen","colorHex":"#1f2937"}],"deleted":true},"message":"Product retrieved."}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected body\n got: %s\nwant: %s", got, want)
	}
}

func TestGetProductNotFound(t *testing.T) {
	client := &fakeCatalogClient{
		getProductFn: func(ctx context.Context, id string) (*catalogv1.GetProductResponse, error) {
			return nil, fmt.Errorf("lookup %s: %w", id, catalog.ErrProductNotFound)
		},
	}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products/missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != "PRODUCT_NOT_FOUND" {
		t.Errorf("expected code PRODUCT_NOT_FOUND, got %v", body["code"])
	}
	if body["message"] != "Product not found." {
		t.Errorf("expected message %q, got %v", "Product not found.", body["message"])
	}
	if _, exists := body["errors"]; exists {
		t.Errorf("expected no errors field, got %#v", body["errors"])
	}
	if traceID, ok := body["traceId"].(string); !ok || !hexTraceID.MatchString(traceID) {
		t.Errorf("expected generated hex traceId, got %#v", body["traceId"])
	}
}

func TestGetProductCatalogUnavailable(t *testing.T) {
	client := &fakeCatalogClient{
		getProductFn: func(ctx context.Context, id string) (*catalogv1.GetProductResponse, error) {
			return nil, status.Error(codes.Unavailable, "catalog down")
		},
	}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products/dt-1")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != "CATALOG_UNAVAILABLE" {
		t.Errorf("expected code CATALOG_UNAVAILABLE, got %v", body["code"])
	}
}

func TestErrorResponseUsesTraceIDFromHeader(t *testing.T) {
	client := &fakeCatalogClient{}
	mux := newTestMux(t, client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products?page=0", nil)
	req.Header.Set("X-Trace-Id", "trace-abc-123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["traceId"] != "trace-abc-123" {
		t.Errorf("expected traceId from X-Trace-Id header, got %v", body["traceId"])
	}
}

func TestLegacyProductsRouteReturnsNotFound(t *testing.T) {
	mux := newTestMux(t, &fakeCatalogClient{})
	rec := doRequest(mux, http.MethodGet, "/products")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected legacy /products to return 404, got %d", rec.Code)
	}
}

func TestSearchProductsEnrichesAvailabilityAndDegradesPerItem(t *testing.T) {
	mkSummary := func(id string) *catalogv1.ProductSummary {
		s := sampleSummary()
		s.Id = id
		return s
	}

	cat := &fakeCatalogClient{
		searchFn: func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
			return &catalogv1.SearchProductsResponse{
				Items: []*catalogv1.ProductSummary{
					mkSummary("p-avail"),
					mkSummary("p-limited"),
					mkSummary("p-zero"),
					mkSummary("p-negative"),
					mkSummary("p-err"),
					mkSummary("p-avail"), // duplicate ID should only hit inventory once
				},
				Page:       1,
				PageSize:   20,
				TotalItems: 6,
				TotalPages: 1,
			}, nil
		},
	}

	inv := &fakeInventoryClient{
		getFn: func(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
			switch productID {
			case "p-avail":
				return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: 12}, nil
			case "p-limited":
				return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: 5}, nil
			case "p-zero":
				return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: 0}, nil
			case "p-negative":
				return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: -3}, nil
			case "p-err":
				return nil, status.Error(codes.Unavailable, "inventory timeout")
			default:
				return nil, fmt.Errorf("unexpected productID %s", productID)
			}
		},
	}

	mux := newTestMuxWithInventory(t, cat, inv)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products", nil)
	req.Header.Set("X-Trace-Id", "trace-search-01")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(inv.called) != 5 {
		t.Fatalf("expected 5 unique inventory lookups, got %d (%v)", len(inv.called), inv.called)
	}
	if inv.traceID != "trace-search-01" {
		t.Errorf("expected outgoing x-trace-id trace-search-01, got %q", inv.traceID)
	}

	body := decodeBody(t, rec)
	items := body["data"].(map[string]any)["items"].([]any)
	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(items))
	}

	assertItem := func(idx int, wantStatus string, wantQty *float64, wantDegraded bool) {
		t.Helper()
		item := items[idx].(map[string]any)
		if item["status"] != wantStatus {
			t.Errorf("item[%d] status = %v, want %q", idx, item["status"], wantStatus)
		}
		qtyVal, hasQty := item["availableQuantity"]
		if wantQty == nil {
			if hasQty {
				t.Errorf("item[%d] expected availableQuantity omitted, got %v", idx, qtyVal)
			}
		} else if !hasQty || qtyVal.(float64) != *wantQty {
			t.Errorf("item[%d] availableQuantity = %v (present=%v), want %v", idx, qtyVal, hasQty, *wantQty)
		}
		degVal, hasDeg := item["inventoryDegraded"]
		if wantDegraded {
			if !hasDeg || degVal != true {
				t.Errorf("item[%d] expected inventoryDegraded=true, got %v", idx, degVal)
			}
		} else if hasDeg {
			t.Errorf("item[%d] expected inventoryDegraded omitted, got %v", idx, degVal)
		}
	}

	f12, f5, f0 := 12.0, 5.0, 0.0
	assertItem(0, "AVAILABLE", &f12, false)
	assertItem(1, "LIMITED", &f5, false)
	assertItem(2, "OUT_OF_STOCK", &f0, false)
	assertItem(3, "OUT_OF_STOCK", &f0, false)
	assertItem(4, "UNKNOWN", nil, true)
	assertItem(5, "AVAILABLE", &f12, false)
}

func TestSearchProductsBoundsInventoryConcurrencyAndHandlesNilInventory(t *testing.T) {
	items := make([]*catalogv1.ProductSummary, 0, 20)
	for i := 0; i < 20; i++ {
		s := sampleSummary()
		s.Id = fmt.Sprintf("prod-%02d", i)
		items = append(items, s)
	}
	cat := &fakeCatalogClient{
		searchFn: func(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
			return &catalogv1.SearchProductsResponse{
				Items:      items,
				Page:       1,
				PageSize:   20,
				TotalItems: 20,
				TotalPages: 1,
			}, nil
		},
	}

	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	inv := &fakeInventoryClient{
		getFn: func(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
			cur := inFlight.Add(1)
			for {
				prev := maxSeen.Load()
				if cur <= prev || maxSeen.CompareAndSwap(prev, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			inFlight.Add(-1)
			return &inventoryv1.GetAvailabilityResponse{ProductId: productID, AvailableQuantity: 7}, nil
		},
	}

	mux := newTestMuxWithInventory(t, cat, inv)
	rec := doRequest(mux, http.MethodGet, "/api/v1/storefront/products")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if peak := maxSeen.Load(); peak > 8 {
		t.Fatalf("expected inventory concurrency <= 8, saw %d", peak)
	}

	nilMux := newTestMuxWithInventory(t, cat, nil)
	nilRec := doRequest(nilMux, http.MethodGet, "/api/v1/storefront/products")
	if nilRec.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil inventory client, got %d", nilRec.Code)
	}
	nilItems := decodeBody(t, nilRec)["data"].(map[string]any)["items"].([]any)
	first := nilItems[0].(map[string]any)
	if first["status"] != "UNKNOWN" || first["inventoryDegraded"] != true {
		t.Fatalf("expected UNKNOWN + inventoryDegraded with nil inventory client, got %#v", first)
	}
}

func TestGetProductAvailabilityStatesAndDegradation(t *testing.T) {
	baseDetail := func(deleted bool, variants []*catalogv1.ProductVariant) *catalogv1.GetProductResponse {
		return &catalogv1.GetProductResponse{
			Id:       "dt-2",
			Name:     "Điện thoại Aero",
			Price:    money(9990000),
			Variants: variants,
			Deleted:  deleted,
		}
	}
	oneVariant := []*catalogv1.ProductVariant{{Label: "Đen"}}

	t.Run("available", func(t *testing.T) {
		cat := &fakeCatalogClient{getProductFn: func(context.Context, string) (*catalogv1.GetProductResponse, error) {
			return baseDetail(false, oneVariant), nil
		}}
		inv := &fakeInventoryClient{getFn: func(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
			return &inventoryv1.GetAvailabilityResponse{ProductId: "dt-2", AvailableQuantity: 9}, nil
		}}
		rec := doRequest(newTestMuxWithInventory(t, cat, inv), http.MethodGet, "/api/v1/storefront/products/dt-2")
		data := decodeBody(t, rec)["data"].(map[string]any)
		if data["status"] != "AVAILABLE" || data["availableQuantity"].(float64) != 9 {
			t.Fatalf("unexpected data: %#v", data)
		}
	})

	t.Run("limited", func(t *testing.T) {
		cat := &fakeCatalogClient{getProductFn: func(context.Context, string) (*catalogv1.GetProductResponse, error) {
			return baseDetail(false, oneVariant), nil
		}}
		inv := &fakeInventoryClient{getFn: func(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
			return &inventoryv1.GetAvailabilityResponse{ProductId: "dt-2", AvailableQuantity: 2}, nil
		}}
		rec := doRequest(newTestMuxWithInventory(t, cat, inv), http.MethodGet, "/api/v1/storefront/products/dt-2")
		data := decodeBody(t, rec)["data"].(map[string]any)
		if data["status"] != "LIMITED" || data["availableQuantity"].(float64) != 2 {
			t.Fatalf("unexpected data: %#v", data)
		}
	})

	t.Run("out of stock", func(t *testing.T) {
		cat := &fakeCatalogClient{getProductFn: func(context.Context, string) (*catalogv1.GetProductResponse, error) {
			return baseDetail(false, oneVariant), nil
		}}
		inv := &fakeInventoryClient{getFn: func(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
			return &inventoryv1.GetAvailabilityResponse{ProductId: "dt-2", AvailableQuantity: 0}, nil
		}}
		rec := doRequest(newTestMuxWithInventory(t, cat, inv), http.MethodGet, "/api/v1/storefront/products/dt-2")
		data := decodeBody(t, rec)["data"].(map[string]any)
		if data["status"] != "OUT_OF_STOCK" || data["availableQuantity"].(float64) != 0 {
			t.Fatalf("unexpected data: %#v", data)
		}
	})

	t.Run("empty variants skips inventory and returns UNAVAILABLE", func(t *testing.T) {
		cat := &fakeCatalogClient{getProductFn: func(context.Context, string) (*catalogv1.GetProductResponse, error) {
			return baseDetail(false, nil), nil
		}}
		inv := &fakeInventoryClient{}
		rec := doRequest(newTestMuxWithInventory(t, cat, inv), http.MethodGet, "/api/v1/storefront/products/dt-2")
		if len(inv.called) != 0 {
			t.Fatalf("expected inventory skipped when variants empty, got %v", inv.called)
		}
		data := decodeBody(t, rec)["data"].(map[string]any)
		if data["status"] != "UNAVAILABLE" || data["availableQuantity"].(float64) != 0 {
			t.Fatalf("unexpected data: %#v", data)
		}
	})

	t.Run("inventory error degrades gracefully with 200 OK and logs traceId", func(t *testing.T) {
		var logBuf bytes.Buffer
		prevOut := log.Writer()
		log.SetOutput(&logBuf)
		t.Cleanup(func() { log.SetOutput(prevOut) })

		cat := &fakeCatalogClient{getProductFn: func(context.Context, string) (*catalogv1.GetProductResponse, error) {
			return baseDetail(false, oneVariant), nil
		}}
		inv := &fakeInventoryClient{getFn: func(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
			return nil, status.Error(codes.DeadlineExceeded, "inventory slow")
		}}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront/products/dt-2", nil)
		req.Header.Set("X-Trace-Id", "trace-degraded-123")
		rec := httptest.NewRecorder()
		newTestMuxWithInventory(t, cat, inv).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on degraded inventory, got %d", rec.Code)
		}
		data := decodeBody(t, rec)["data"].(map[string]any)
		if data["status"] != "UNKNOWN" || data["inventoryDegraded"] != true {
			t.Fatalf("unexpected degraded data: %#v", data)
		}
		if _, exists := data["availableQuantity"]; exists {
			t.Fatalf("expected availableQuantity omitted on UNKNOWN, got %#v", data["availableQuantity"])
		}
		if !strings.Contains(logBuf.String(), "traceId=trace-degraded-123") {
			t.Fatalf("expected degraded log to include traceId=trace-degraded-123, got %q", logBuf.String())
		}
	})
}
