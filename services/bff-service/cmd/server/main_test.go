package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
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

type fakeInventoryClient struct{}

func (fakeInventoryClient) GetAvailability(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error) {
	return &inventoryv1.GetAvailabilityResponse{AvailableQuantity: 10}, nil
}

var _ cart.Client = fakeCartClient{}
var _ catalog.Client = fakeCatalogClient{}
var _ inventory.Client = fakeInventoryClient{}

func TestBuildMuxMakesCanonicalRoutesReachableAndRejectsLegacyPaths(t *testing.T) {
	mux := buildMux(fakeCartClient{}, fakeCatalogClient{}, fakeInventoryClient{})

	for _, tc := range []struct {
		method string
		path   string
		caller string
		status int
	}{
		{method: http.MethodGet, path: "/healthz", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/storefront/products", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/cart", caller: "user-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/", caller: "user-1", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/products", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/v1/cart/unknown", caller: "user-1", status: http.StatusNotFound},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
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
