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
					{Id: "dt-1", Name: "Nova X15", Brand: "Nova", CategoryId: "dien-thoai", CategoryName: "Điện thoại"},
					{Id: "lt-1", Name: "Laptop M4", Brand: "Apex", CategoryId: "laptop", CategoryName: "Laptop"},
					{Id: "tn-1", Name: "Tai nghe ẩn", Brand: "Sound", CategoryId: "tai-nghe", CategoryName: "Tai nghe", Deleted: true},
				},
			}, nil
		},
		getProductFn: func(_ context.Context, id string) (*catalogv1.GetProductResponse, error) {
			if id == "missing" {
				return nil, catalog.ErrProductNotFound
			}
			return &catalogv1.GetProductResponse{Id: id, Name: "Product"}, nil
		},
	}

	inv := &fakeInventoryClient{
		listStockFn: func(_ context.Context, _ string) ([]*inventoryv1.StockItem, error) {
			return []*inventoryv1.StockItem{
				{ProductId: "dt-1", Quantity: 20, Reserved: 2, AvailableQuantity: 18},
				{ProductId: "lt-1", Quantity: 4, Reserved: 1, AvailableQuantity: 3},
				// tn-1 intentionally omitted from stock table -> defaults to 0/0/0 (out_of_stock)
			}, nil
		},
		setStockFn: func(_ context.Context, _, productID string, quantity int32) (*inventoryv1.StockItem, error) {
			if quantity < 2 {
				return nil, inventory.ErrStockBelowReserved
			}
			return &inventoryv1.StockItem{ProductId: productID, Quantity: quantity, Reserved: 2, AvailableQuantity: quantity - 2}, nil
		},
		adjStockFn: func(_ context.Context, _, productID string, delta int32) (*inventoryv1.StockItem, error) {
			return &inventoryv1.StockItem{ProductId: productID, Quantity: 20 + delta, Reserved: 2, AvailableQuantity: 18 + delta}, nil
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
	if len(items) != 1 || items[0].(map[string]any)["productId"] != "lt-1" {
		t.Fatalf("expected 1 low_stock item (lt-1), got %+v", items)
	}
	summary := data["summary"].(map[string]any)
	if summary["totalTrackedProducts"].(float64) != 3 ||
		summary["inStockCount"].(float64) != 1 ||
		summary["lowStockCount"].(float64) != 1 ||
		summary["outOfStockCount"].(float64) != 1 {
		t.Fatalf("unexpected KPI summary: %+v", summary)
	}

	// 2. PUT /api/v1/admin/inventory/missing -> 404 PRODUCT_NOT_FOUND
	missReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/missing", strings.NewReader(`{"quantity":10}`))
	missReq.Header.Set("X-User-Id", "admin-1")
	missRec := httptest.NewRecorder()
	mux.ServeHTTP(missRec, missReq)
	if missRec.Code != http.StatusNotFound || decodeBody(t, missRec)["code"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected 404 PRODUCT_NOT_FOUND, got %d %s", missRec.Code, missRec.Body.String())
	}

	// 3. PUT /api/v1/admin/inventory/dt-1 below reserved -> 409 STOCK_BELOW_RESERVED
	conflictReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/inventory/dt-1", strings.NewReader(`{"quantity":1}`))
	conflictReq.Header.Set("X-User-Id", "admin-1")
	conflictRec := httptest.NewRecorder()
	mux.ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict || decodeBody(t, conflictRec)["code"] != "STOCK_BELOW_RESERVED" {
		t.Fatalf("expected 409 STOCK_BELOW_RESERVED, got %d %s", conflictRec.Code, conflictRec.Body.String())
	}

	// 4. POST /api/v1/admin/inventory/dt-1/adjust -> 200 OK
	adjReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory/dt-1/adjust", strings.NewReader(`{"delta":5}`))
	adjReq.Header.Set("X-User-Id", "admin-1")
	adjRec := httptest.NewRecorder()
	mux.ServeHTTP(adjRec, adjReq)
	if adjRec.Code != http.StatusOK {
		t.Fatalf("POST /adjust status = %d: %s", adjRec.Code, adjRec.Body.String())
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
