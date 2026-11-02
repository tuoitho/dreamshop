package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
)

func TestAdminInventoryListMergeSummaryAndMutations(t *testing.T) {
	cat := &fakeCatalogClient{
		listInventoryFn: func(_ context.Context, _ *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
			return &catalogv1.ListInventoryCatalogItemsResponse{
				Items: []*catalogv1.InventoryCatalogItem{
					{
						Id: "dt-1", Name: "Nova X15", Brand: "Nova", CategoryId: "dien-thoai", CategoryName: "Điện thoại",
						Variants: []*catalogv1.ProductVariant{
							{SkuId: "dt-1-v1", Label: "Đen 256GB", ColorHex: "#111827", Price: money(18990000), OriginalPrice: money(21990000)},
							{SkuId: "dt-1-v2", Label: "Trắng 512GB", ColorHex: "#f3f4f6", Price: money(20990000), OriginalPrice: money(23990000)},
						},
					},
					{
						Id: "lt-1", Name: "Laptop M4", Brand: "Apex", CategoryId: "laptop", CategoryName: "Laptop",
						Variants: []*catalogv1.ProductVariant{
							{SkuId: "lt-1-v1", Label: "16GB/512GB", Price: money(32990000)},
						},
					},
					{
						Id: "tn-1", Name: "Tai nghe ẩn", Brand: "Sound", CategoryId: "tai-nghe", CategoryName: "Tai nghe", Deleted: true,
						Variants: []*catalogv1.ProductVariant{
							{SkuId: "tn-1-v1", Label: "Tiêu chuẩn", Price: money(2990000)},
						},
					},
				},
			}, nil
		},
		getProductFn: func(_ context.Context, id string) (*catalogv1.GetProductResponse, error) {
			if id == "missing" {
				return nil, catalog.ErrProductNotFound
			}
			return &catalogv1.GetProductResponse{
				Id:   id,
				Name: "Nova X15",
				Variants: []*catalogv1.ProductVariant{
					{SkuId: "dt-1-v1", Label: "Đen 256GB", Price: money(18990000)},
					{SkuId: "dt-1-v2", Label: "Trắng 512GB", Price: money(20990000)},
				},
			}, nil
		},
	}

	inv := &fakeInventoryClient{
		listStockFn: func(_ context.Context, _ string) ([]*inventoryv1.StockItem, error) {
			return []*inventoryv1.StockItem{
				{SkuId: "dt-1-v1", ProductId: "dt-1", Quantity: 20, Reserved: 2, AvailableQuantity: 18},
				{SkuId: "dt-1-v2", ProductId: "dt-1", Quantity: 10, Reserved: 0, AvailableQuantity: 10},
				{SkuId: "lt-1-v1", ProductId: "lt-1", Quantity: 4, Reserved: 1, AvailableQuantity: 3},
				// tn-1-v1 intentionally omitted from stock table -> defaults to 0/0/0 (out_of_stock)
			}, nil
		},
		setStockFn: func(_ context.Context, _, skuID, productID string, quantity int32) (*inventoryv1.StockItem, error) {
			if quantity < 2 {
				return nil, inventory.ErrStockBelowReserved
			}
			return &inventoryv1.StockItem{SkuId: skuID, ProductId: productID, Quantity: quantity, Reserved: 2, AvailableQuantity: quantity - 2}, nil
		},
		adjStockFn: func(_ context.Context, _, skuID, productID string, delta int32) (*inventoryv1.StockItem, error) {
			return &inventoryv1.StockItem{SkuId: skuID, ProductId: productID, Quantity: 20 + delta, Reserved: 2, AvailableQuantity: 18 + delta}, nil
		},
	}

	mux := http.NewServeMux()
	NewAdminInventoryHandler(cat, inv).Register(mux)

	// 1. GET /api/v1/admin/inventory?stockStatus=low_stock
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory?stockStatus=low_stock", nil)
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/inventory status = %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec)["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["skuId"] != "lt-1-v1" || items[0].(map[string]any)["productId"] != "lt-1" {
		t.Fatalf("expected 1 low_stock SKU item (lt-1-v1), got %+v", items)
	}
	summary := data["summary"].(map[string]any)
	if summary["totalTrackedProducts"].(float64) != 3 ||
		summary["totalTrackedSkus"].(float64) != 4 ||
		summary["inStockCount"].(float64) != 2 ||
		summary["lowStockCount"].(float64) != 1 ||
		summary["outOfStockCount"].(float64) != 1 {
		t.Fatalf("unexpected KPI summary: %+v", summary)
	}

	// 2. PUT /api/v1/admin/inventory/skus/dt-1-v1 with missing product -> 404 PRODUCT_NOT_FOUND
	missReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/skus/dt-1-v1", strings.NewReader(`{"productId":"missing","quantity":10}`))
	missReq.Header.Set("X-User-Id", "admin-1")
	missRec := httptest.NewRecorder()
	mux.ServeHTTP(missRec, missReq)
	if missRec.Code != http.StatusNotFound || decodeBody(t, missRec)["code"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected 404 PRODUCT_NOT_FOUND, got %d %s", missRec.Code, missRec.Body.String())
	}

	// 2b. PUT /api/v1/admin/inventory/skus/unknown-sku -> 404 SKU_NOT_FOUND
	skuMissReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/skus/unknown-sku", strings.NewReader(`{"productId":"dt-1","quantity":10}`))
	skuMissReq.Header.Set("X-User-Id", "admin-1")
	skuMissRec := httptest.NewRecorder()
	mux.ServeHTTP(skuMissRec, skuMissReq)
	if skuMissRec.Code != http.StatusNotFound || decodeBody(t, skuMissRec)["code"] != "SKU_NOT_FOUND" {
		t.Fatalf("expected 404 SKU_NOT_FOUND, got %d %s", skuMissRec.Code, skuMissRec.Body.String())
	}

	// 2c. Legacy product-level route /api/v1/admin/inventory/dt-1 is gone
	legacyReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/dt-1", strings.NewReader(`{"quantity":10}`))
	legacyReq.Header.Set("X-User-Id", "admin-1")
	legacyRec := httptest.NewRecorder()
	mux.ServeHTTP(legacyRec, legacyReq)
	if legacyRec.Code != http.StatusNotFound && legacyRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected legacy /api/v1/admin/inventory/dt-1 to return 404/405, got %d", legacyRec.Code)
	}

	// 3. PUT /api/v1/admin/inventory/skus/dt-1-v1 below reserved -> 409 STOCK_BELOW_RESERVED
	conflictReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/skus/dt-1-v1", strings.NewReader(`{"productId":"dt-1","quantity":1}`))
	conflictReq.Header.Set("X-User-Id", "admin-1")
	conflictRec := httptest.NewRecorder()
	mux.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict || decodeBody(t, conflictRec)["code"] != "STOCK_BELOW_RESERVED" {
		t.Fatalf("expected 409 STOCK_BELOW_RESERVED, got %d %s", conflictRec.Code, conflictRec.Body.String())
	}

	// 4. POST /api/v1/admin/inventory/skus/dt-1-v1/adjust -> 200 OK
	adjReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory/skus/dt-1-v1/adjust", strings.NewReader(`{"productId":"dt-1","delta":5}`))
	adjReq.Header.Set("X-User-Id", "admin-1")
	adjRec := httptest.NewRecorder()
	mux.ServeHTTP(adjRec, adjReq)
	if adjRec.Code != http.StatusOK {
		t.Fatalf("POST /adjust status = %d: %s", adjRec.Code, adjRec.Body.String())
	}
	adjData := decodeBody(t, adjRec)["data"].(map[string]any)
	if adjData["skuId"] != "dt-1-v1" || adjData["productId"] != "dt-1" {
		t.Fatalf("unexpected adjust response data: %+v", adjData)
	}

	// 5. Forbidden when ListStocks returns ErrPermissionDenied
	inv.listStockFn = func(context.Context, string) ([]*inventoryv1.StockItem, error) {
		return nil, inventory.ErrPermissionDenied
	}
	forbReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory", nil)
	forbReq.Header.Set("X-User-Id", "customer-1")
	forbRec := httptest.NewRecorder()
	mux.ServeHTTP(forbRec, forbReq)
	if forbRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d %s", forbRec.Code, forbRec.Body.String())
	}
}

func TestAdminInventoryListFiltersVariantBySkuQuery(t *testing.T) {
	cat := &fakeCatalogClient{
		listInventoryFn: func(_ context.Context, _ *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
			return &catalogv1.ListInventoryCatalogItemsResponse{
				Items: []*catalogv1.InventoryCatalogItem{
					{
						Id: "dt-1", Name: "Nova X15", Brand: "Apex", CategoryId: "dien-thoai", CategoryName: "Điện thoại",
						Variants: []*catalogv1.ProductVariant{
							{SkuId: "dt-1-v1", Label: "Đen 256GB", Price: money(18990000)},
							{SkuId: "dt-1-v2", Label: "Trắng 512GB", Price: money(20990000)},
						},
					},
				},
			}, nil
		},
	}
	inv := &fakeInventoryClient{
		listStockFn: func(context.Context, string) ([]*inventoryv1.StockItem, error) {
			return []*inventoryv1.StockItem{
				{SkuId: "dt-1-v1", ProductId: "dt-1", Quantity: 10, AvailableQuantity: 10},
				{SkuId: "dt-1-v2", ProductId: "dt-1", Quantity: 5, AvailableQuantity: 5},
			}, nil
		},
	}

	mux := http.NewServeMux()
	NewAdminInventoryHandler(cat, inv).Register(mux)

	skuReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory?q=dt-1-v2", nil)
	skuReq.Header.Set("X-User-Id", "admin-1")
	skuRec := httptest.NewRecorder()
	mux.ServeHTTP(skuRec, skuReq)
	skuData := decodeBody(t, skuRec)["data"].(map[string]any)
	skuItems := skuData["items"].([]any)
	if len(skuItems) != 1 || skuItems[0].(map[string]any)["skuId"] != "dt-1-v2" {
		t.Fatalf("expected ?q=dt-1-v2 to return only dt-1-v2, got %+v", skuItems)
	}

	brandReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory?q=Apex", nil)
	brandReq.Header.Set("X-User-Id", "admin-1")
	brandRec := httptest.NewRecorder()
	mux.ServeHTTP(brandRec, brandReq)
	brandItems := decodeBody(t, brandRec)["data"].(map[string]any)["items"].([]any)
	if len(brandItems) != 2 {
		t.Fatalf("expected ?q=Apex to return all 2 variants of Apex, got %+v", brandItems)
	}
}
