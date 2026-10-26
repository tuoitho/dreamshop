package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce.local/bff-service/internal/cart"
	"ecommerce.local/bff-service/internal/catalog"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
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

var _ cart.Client = fakeCartClient{}
var _ catalog.Client = fakeCatalogClient{}

func TestBuildMuxMakesAuthenticatedCartRouteReachable(t *testing.T) {
	mux := buildMux(fakeCartClient{}, fakeCatalogClient{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cart route to reach handler, got %d: %s", rec.Code, rec.Body.String())
	}
}
