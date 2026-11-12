package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	"ecommerce.local/bff-service/internal/order"
	"ecommerce.local/bff-service/internal/payment"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	orderv1 "ecommerce.local/contracts/go/order/v1"
	paymentv1 "ecommerce.local/contracts/go/payment/v1"
)

type fakeCartClient struct{}

func (fakeCartClient) GetCart(context.Context, string) (*cartv1.GetCartResponse, error) {
	return &cartv1.GetCartResponse{Cart: &cartv1.Cart{}}, nil
}
func (fakeCartClient) AddItem(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) {
	return &cartv1.AddItemResponse{Cart: &cartv1.Cart{}}, nil
}
func (fakeCartClient) UpdateItemQuantity(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error) {
	return &cartv1.UpdateItemQuantityResponse{Cart: &cartv1.Cart{}}, nil
}
func (fakeCartClient) RemoveItem(context.Context, string, string) (*cartv1.RemoveItemResponse, error) {
	return &cartv1.RemoveItemResponse{Cart: &cartv1.Cart{}}, nil
}
func (fakeCartClient) ClearCart(context.Context, string) (*cartv1.ClearCartResponse, error) {
	return &cartv1.ClearCartResponse{Cart: &cartv1.Cart{}}, nil
}
func (fakeCartClient) GetFavorites(context.Context, string) (*cartv1.GetFavoritesResponse, error) {
	return &cartv1.GetFavoritesResponse{Favorites: &cartv1.FavoriteList{}}, nil
}
func (fakeCartClient) AddFavorite(context.Context, string, string) (*cartv1.AddFavoriteResponse, error) {
	return &cartv1.AddFavoriteResponse{Favorites: &cartv1.FavoriteList{}}, nil
}
func (fakeCartClient) RemoveFavorite(context.Context, string, string) (*cartv1.RemoveFavoriteResponse, error) {
	return &cartv1.RemoveFavoriteResponse{Favorites: &cartv1.FavoriteList{}}, nil
}
func (fakeCartClient) ClearFavorites(context.Context, string) (*cartv1.ClearFavoritesResponse, error) {
	return &cartv1.ClearFavoritesResponse{Favorites: &cartv1.FavoriteList{}}, nil
}

type fakeCatalogClient struct{}

func (fakeCatalogClient) GetProduct(context.Context, string) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}
func (fakeCatalogClient) SearchProducts(context.Context, *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
	return &catalogv1.SearchProductsResponse{}, nil
}
func (fakeCatalogClient) SoftDeleteProduct(context.Context, string, string, bool) error {
	return nil
}
func (fakeCatalogClient) ListAdminProducts(context.Context, *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error) {
	return &catalogv1.ListAdminProductsResponse{}, nil
}
func (fakeCatalogClient) ListInventoryCatalogItems(context.Context, *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
	return &catalogv1.ListInventoryCatalogItemsResponse{}, nil
}
func (fakeCatalogClient) GetAdminProduct(context.Context, string, string) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}
func (fakeCatalogClient) CreateProduct(context.Context, *catalogv1.CreateProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}
func (fakeCatalogClient) UpdateProduct(context.Context, *catalogv1.UpdateProductRequest) (*catalogv1.GetProductResponse, error) {
	return &catalogv1.GetProductResponse{}, nil
}
func (fakeCatalogClient) CreateProductMediaUpload(context.Context, *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
	return &catalogv1.CreateProductMediaUploadResponse{}, nil
}

type fakeInventoryClient struct{}

func (fakeInventoryClient) GetAvailability(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
	return &inventoryv1.GetAvailabilityResponse{AvailableQuantity: 10}, nil
}
func (fakeInventoryClient) BatchGetAvailability(context.Context, []string) ([]*inventoryv1.AvailabilityItem, error) {
	return nil, nil
}
func (fakeInventoryClient) GetStock(context.Context, string, string) (*inventoryv1.StockItem, error) {
	return &inventoryv1.StockItem{}, nil
}
func (fakeInventoryClient) BatchGetStock(context.Context, string, []string) ([]*inventoryv1.StockItem, error) {
	return nil, nil
}
func (fakeInventoryClient) ListStocks(context.Context, string) ([]*inventoryv1.StockItem, error) {
	return nil, nil
}
func (fakeInventoryClient) SetStock(context.Context, string, string, string, int32) (*inventoryv1.StockItem, error) {
	return &inventoryv1.StockItem{}, nil
}
func (fakeInventoryClient) AdjustStock(context.Context, string, string, string, int32) (*inventoryv1.StockItem, error) {
	return &inventoryv1.StockItem{}, nil
}
func (fakeInventoryClient) DeleteStocks(context.Context, string, []string, bool) error {
	return nil
}

type fakeOrderClient struct{}

func (fakeOrderClient) CreateOrder(context.Context, *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	return &orderv1.CreateOrderResponse{Order: &orderv1.OrderDetail{OrderId: "ord-1"}}, nil
}
func (fakeOrderClient) GetOrder(context.Context, string, string) (*orderv1.GetOrderResponse, error) {
	return &orderv1.GetOrderResponse{Order: &orderv1.OrderDetail{OrderId: "ord-1"}}, nil
}
func (fakeOrderClient) ListOrders(context.Context, string, string, int32, int32) (*orderv1.ListOrdersResponse, error) {
	return &orderv1.ListOrdersResponse{}, nil
}
func (fakeOrderClient) CancelOrder(context.Context, string, string, string) (*orderv1.CancelOrderResponse, error) {
	return &orderv1.CancelOrderResponse{Order: &orderv1.OrderDetail{OrderId: "ord-1"}}, nil
}
func (fakeOrderClient) AdminListOrders(context.Context, string, string, string, int32, int32) (*orderv1.AdminListOrdersResponse, error) {
	return &orderv1.AdminListOrdersResponse{}, nil
}
func (fakeOrderClient) AdminGetOrder(context.Context, string, string) (*orderv1.AdminGetOrderResponse, error) {
	return &orderv1.AdminGetOrderResponse{Order: &orderv1.OrderDetail{OrderId: "ord-1"}}, nil
}
func (fakeOrderClient) AdminUpdateOrderStatus(context.Context, *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error) {
	return &orderv1.AdminUpdateOrderStatusResponse{Order: &orderv1.OrderDetail{OrderId: "ord-1"}}, nil
}

type fakePaymentClient struct{}

func (fakePaymentClient) HandleMomoCallback(context.Context, *paymentv1.HandleMomoCallbackRequest) (*paymentv1.HandleMomoCallbackResponse, error) {
	return &paymentv1.HandleMomoCallbackResponse{OrderId: "ord-1", PaymentStatus: "SUCCEEDED", RedirectResult: "success"}, nil
}

var _ cart.Client = fakeCartClient{}
var _ catalog.Client = fakeCatalogClient{}
var _ inventory.Client = fakeInventoryClient{}
var _ order.Client = fakeOrderClient{}
var _ payment.Client = fakePaymentClient{}

func TestBuildMuxMakesCanonicalRoutesReachableAndRejectsLegacyPaths(t *testing.T) {
	mux := buildMux(fakeCartClient{}, fakeCatalogClient{}, fakeInventoryClient{}, fakeOrderClient{}, fakePaymentClient{}, "http://localhost:5173")

	for _, tc := range []struct {
		method string
		path   string
		caller string
		body   string
		status int
	}{
		{method: http.MethodGet, path: "/healthz", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/storefront/products", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/cart", caller: "user-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/favorites", caller: "user-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/favorites", status: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/v1/orders", caller: "user-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/orders", status: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/v1/payments/momo-ipn", body: `{"orderId":"ord-1","signature":"sig"}`, status: http.StatusNoContent},
		{method: http.MethodGet, path: "/api/v1/admin/products", caller: "admin-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/admin/inventory", caller: "admin-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/admin/orders", caller: "admin-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/admin/orders", status: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/", caller: "user-1", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/products", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/v1/cart/unknown", caller: "user-1", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/v1/favorites/unknown", caller: "user-1", status: http.StatusNotFound},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.caller != "" {
			req.Header.Set("X-User-Id", tc.caller)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
	}
}
