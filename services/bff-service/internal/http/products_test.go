package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"

	"ecommerce.local/bff-service/internal/catalog"
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

func newTestMux(t *testing.T, client catalog.Client) http.Handler {
	t.Helper()
	h := NewProductsHandler(client)
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

	rec := doRequest(mux, http.MethodGet, "/products")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	want := `{"data":{"items":[{"id":"dt-1","name":"Điện thoại Nova X","brand":"Nova","categoryId":"dien-thoai","categoryName":"Điện thoại","price":18990000,"originalPrice":21990000,"rating":4.9,"reviewCount":1284,"soldCount":2310,"images":["https://picsum.photos/seed/dt-1-1/800/800"],"emoji":"📱","imageHue":[220,260],"isNew":false,"badge":"hot","featured":true}],"page":1,"pageSize":20,"totalItems":1,"totalPages":1},"message":"Products retrieved."}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected body\n got: %s\nwant: %s", got, want)
	}
}

func TestSearchProductsEmptyItemsIsArray(t *testing.T) {
	client := &fakeCatalogClient{}
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/products")

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
			target: "/products?q=iphone&danh-muc=dien-thoai&gia=500000-2000000&giam-gia=1&danh-gia=4&sap-xep=gia-cao&page=3&pageSize=25",
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
			target: "/products?gia=10000000-0",
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
			target: "/products",
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
		{"bad gia format", "/products?gia=abc-def", "gia"},
		{"gia missing max", "/products?gia=500-", "gia"},
		{"negative gia min", "/products?gia=-100-500", "gia"},
		{"negative gia max", "/products?gia=500--100", "gia"},
		{"extra dash in gia", "/products?gia=1-2-3", "gia"},
		{"page zero", "/products?page=0", "page"},
		{"page negative", "/products?page=-1", "page"},
		{"page non-numeric", "/products?page=abc", "page"},
		{"pageSize zero", "/products?pageSize=0", "pageSize"},
		{"pageSize too large", "/products?pageSize=51", "pageSize"},
		{"pageSize non-numeric", "/products?pageSize=abc", "pageSize"},
		{"rating too low", "/products?danh-gia=0", "danh-gia"},
		{"rating too high", "/products?danh-gia=6", "danh-gia"},
		{"rating non-numeric", "/products?danh-gia=abc", "danh-gia"},
		{"sort invalid", "/products?sap-xep=bogus", "sap-xep"},
		{"giam-gia invalid", "/products?giam-gia=2", "giam-gia"},
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

			rec := doRequest(mux, http.MethodGet, "/products")

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

	rec := doRequest(mux, http.MethodGet, "/products")

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
	mux := newTestMux(t, client)

	rec := doRequest(mux, http.MethodGet, "/products/dt-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	want := `{"data":{"id":"dt-1","name":"Điện thoại Nova X","brand":"Nova","categoryId":"dien-thoai","categoryName":"Điện thoại","price":18990000,"originalPrice":21990000,"rating":4.9,"reviewCount":1284,"soldCount":2310,"images":["https://picsum.photos/seed/dt-1-1/800/800"],"emoji":"📱","imageHue":[220,260],"isNew":false,"badge":"hot","featured":true,"shortDescription":"Mô tả ngắn gọn.","description":"Mô tả chi tiết sản phẩm.","details":["Hỗ trợ 5G","Pin 5000 mAh"],"variants":[{"label":"Đen","colorHex":"#1f2937"}],"deleted":true},"message":"Product retrieved."}` + "\n"
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

	rec := doRequest(mux, http.MethodGet, "/products/missing")

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

	rec := doRequest(mux, http.MethodGet, "/products/dt-1")

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

	req := httptest.NewRequest(http.MethodGet, "/products?page=0", nil)
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
