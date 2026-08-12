package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cartclient "ecommerce.local/bff-service/internal/cart"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeCartClient struct {
	getFn    func(context.Context, string) (*cartv1.GetCartResponse, error)
	addFn    func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error)
	updateFn func(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error)
	removeFn func(context.Context, string, string) (*cartv1.RemoveItemResponse, error)
	clearFn  func(context.Context, string) (*cartv1.ClearCartResponse, error)
}

func (f *fakeCartClient) GetCart(ctx context.Context, userID string) (*cartv1.GetCartResponse, error) {
	return f.getFn(ctx, userID)
}

func (f *fakeCartClient) AddItem(ctx context.Context, userID, productID, variantLabel string, quantity int32) (*cartv1.AddItemResponse, error) {
	return f.addFn(ctx, userID, productID, variantLabel, quantity)
}

func (f *fakeCartClient) UpdateItemQuantity(ctx context.Context, userID, itemID string, quantity int32) (*cartv1.UpdateItemQuantityResponse, error) {
	return f.updateFn(ctx, userID, itemID, quantity)
}

func (f *fakeCartClient) RemoveItem(ctx context.Context, userID, itemID string) (*cartv1.RemoveItemResponse, error) {
	return f.removeFn(ctx, userID, itemID)
}

func (f *fakeCartClient) ClearCart(ctx context.Context, userID string) (*cartv1.ClearCartResponse, error) {
	return f.clearFn(ctx, userID)
}

func newCartTestMux(t *testing.T, client cartclient.Client) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	NewCartHandler(client).Register(mux)
	return mux
}

func cartResponse(item *cartv1.CartItem) *cartv1.GetCartResponse {
	return &cartv1.GetCartResponse{Cart: &cartv1.Cart{Items: []*cartv1.CartItem{item}, Degraded: true}}
}

func cartItemFixture() *cartv1.CartItem {
	return &cartv1.CartItem{
		ItemId: "item-1", ProductId: "dt-1", VariantLabel: "16GB/512GB", Quantity: 2,
		Name: "Nova X", Brand: "Nova", CategoryName: "Phones",
		UnitPrice: &commonv1.Money{AmountMinor: 18990000}, ImageEmoji: "📱",
		ImageHue: []uint32{220, 260}, ImageUrl: "https://example.test/dt-1.png",
		VariantColorHex: "#111111", Status: cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED,
		AvailableQuantity: 3,
	}
}

func TestGetCartMapsItemToExactCamelCaseEnvelope(t *testing.T) {
	client := &fakeCartClient{getFn: func(ctx context.Context, userID string) (*cartv1.GetCartResponse, error) {
		if userID != "user-1" {
			t.Errorf("expected caller user-1, got %q", userID)
		}
		return cartResponse(cartItemFixture()), nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	want := `{"data":{"items":[{"itemId":"item-1","productId":"dt-1","variantLabel":"16GB/512GB","quantity":2,"name":"Nova X","brand":"Nova","categoryName":"Phones","unitPrice":18990000,"imageEmoji":"📱","imageHue":[220,260],"imageUrl":"https://example.test/dt-1.png","variantColorHex":"#111111","status":"LIMITED","availableQuantity":3}],"degraded":true},"message":"Cart retrieved."}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected body\n got: %s\nwant: %s", got, want)
	}
}

func TestGetCartMapsUnknownStatusesToUnavailable(t *testing.T) {
	for _, statusValue := range []cartv1.CartItemStatus{
		cartv1.CartItemStatus_CART_ITEM_STATUS_UNSPECIFIED,
		cartv1.CartItemStatus(99),
	} {
		t.Run(statusValue.String(), func(t *testing.T) {
			item := cartItemFixture()
			item.Status = statusValue
			client := &fakeCartClient{getFn: func(context.Context, string) (*cartv1.GetCartResponse, error) {
				return cartResponse(item), nil
			}}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-User-Id", "user-1")
			rec := httptest.NewRecorder()
			newCartTestMux(t, client).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			data := body["data"].(map[string]any)
			items := data["items"].([]any)
			if got := items[0].(map[string]any)["status"]; got != "UNAVAILABLE" {
				t.Fatalf("expected public status UNAVAILABLE, got %v", got)
			}
		})
	}
}

func TestAddCartItemAcceptsPayloadAndForwardsCallerIdentity(t *testing.T) {
	called := false
	client := &fakeCartClient{addFn: func(ctx context.Context, userID, productID, variantLabel string, quantity int32) (*cartv1.AddItemResponse, error) {
		called = true
		if userID != "user-1" || productID != "dt-1" || variantLabel != "16GB/512GB" || quantity != 2 {
			t.Fatalf("unexpected add arguments: %q %q %q %d", userID, productID, variantLabel, quantity)
		}
		return &cartv1.AddItemResponse{Cart: &cartv1.Cart{}}, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"productId":"dt-1","variantLabel":"16GB/512GB","quantity":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected successful add call, got status %d body %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["message"] != "Product added to cart." {
		t.Fatalf("expected add message, got %v", body["message"])
	}
}

func TestAddCartItemRejectsNonPositiveQuantity(t *testing.T) {
	client := &fakeCartClient{addFn: func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) {
		t.Fatal("cart client must not be called")
		return nil, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"productId":"dt-1","variantLabel":"16GB/512GB","quantity":0}`))
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["code"] != "INVALID_QUANTITY" {
		t.Fatalf("expected code INVALID_QUANTITY, got %v", body["code"])
	}
}

func TestCartMutationRejectsInvalidJSONBodies(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   string
		code   string
	}{
		{name: "unknown field", method: http.MethodPost, target: "/items", body: `{"productId":"dt-1","variantLabel":"red","quantity":2,"userId":"forged"}`, code: "INVALID_PARAMETER"},
		{name: "trailing json", method: http.MethodPost, target: "/items", body: `{"productId":"dt-1","variantLabel":"red","quantity":2}{}`, code: "INVALID_PARAMETER"},
		{name: "blank quantity", method: http.MethodPost, target: "/items", body: `{"productId":"dt-1","variantLabel":"red","quantity":""}`, code: "INVALID_QUANTITY"},
		{name: "omitted quantity", method: http.MethodPatch, target: "/items/item-1", body: `{}`, code: "INVALID_QUANTITY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCartClient{
				addFn: func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) {
					t.Fatal("AddItem must not be called")
					return nil, nil
				},
				updateFn: func(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error) {
					t.Fatal("UpdateItemQuantity must not be called")
					return nil, nil
				},
			}
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.Header.Set("X-User-Id", "user-1")
			rec := httptest.NewRecorder()
			newCartTestMux(t, client).ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body["code"] != tc.code {
				t.Fatalf("expected code %s, got %v", tc.code, body["code"])
			}
		})
	}
}

func TestUpdateCartItemQuantitySuccess(t *testing.T) {
	called := false
	client := &fakeCartClient{updateFn: func(ctx context.Context, userID, itemID string, quantity int32) (*cartv1.UpdateItemQuantityResponse, error) {
		called = true
		if userID != "user-1" || itemID != "item-1" || quantity != 4 {
			t.Fatalf("unexpected update arguments: %q %q %d", userID, itemID, quantity)
		}
		return &cartv1.UpdateItemQuantityResponse{Cart: &cartv1.Cart{}}, nil
	}}
	req := httptest.NewRequest(http.MethodPatch, "/items/item-1", strings.NewReader(`{"quantity":4}`))
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected successful update, got status %d body %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["message"] != "Cart quantity updated." {
		t.Fatalf("expected update message, got %v", body["message"])
	}
}

func TestRemoveCartItemSuccess(t *testing.T) {
	called := false
	client := &fakeCartClient{removeFn: func(ctx context.Context, userID, itemID string) (*cartv1.RemoveItemResponse, error) {
		called = true
		if userID != "user-1" || itemID != "item-1" {
			t.Fatalf("unexpected remove arguments: %q %q", userID, itemID)
		}
		return &cartv1.RemoveItemResponse{Cart: &cartv1.Cart{}}, nil
	}}
	req := httptest.NewRequest(http.MethodDelete, "/items/item-1", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected successful remove, got status %d body %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["message"] != "Product removed from cart." {
		t.Fatalf("expected remove message, got %v", body["message"])
	}
}

func TestClearCartSuccess(t *testing.T) {
	called := false
	client := &fakeCartClient{clearFn: func(ctx context.Context, userID string) (*cartv1.ClearCartResponse, error) {
		called = true
		if userID != "user-1" {
			t.Fatalf("unexpected clear user id: %q", userID)
		}
		return &cartv1.ClearCartResponse{Cart: &cartv1.Cart{}}, nil
	}}
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected successful clear, got status %d body %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["message"] != "Cart cleared." {
		t.Fatalf("expected clear message, got %v", body["message"])
	}
}

func TestCartRouteWithoutCallerReturnsUnauthorized(t *testing.T) {
	client := &fakeCartClient{getFn: func(context.Context, string) (*cartv1.GetCartResponse, error) {
		t.Fatal("cart client must not be called")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["code"] != "UNAUTHORIZED" {
		t.Fatalf("expected code UNAUTHORIZED, got %v", body["code"])
	}
}

func TestCartItemNotFoundMapsToStableError(t *testing.T) {
	client := &fakeCartClient{removeFn: func(context.Context, string, string) (*cartv1.RemoveItemResponse, error) {
		return nil, cartclient.ErrCartItemNotFound
	}}
	req := httptest.NewRequest(http.MethodDelete, "/items/item-1", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["code"] != "CART_ITEM_NOT_FOUND" {
		t.Fatalf("expected code CART_ITEM_NOT_FOUND, got %v", body["code"])
	}
}

func TestCartUnavailableMapsToBadGateway(t *testing.T) {
	client := &fakeCartClient{getFn: func(context.Context, string) (*cartv1.GetCartResponse, error) {
		return nil, status.Error(codes.Unavailable, "cart down")
	}}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["code"] != "CART_UNAVAILABLE" {
		t.Fatalf("expected code CART_UNAVAILABLE, got %v", body["code"])
	}
}
