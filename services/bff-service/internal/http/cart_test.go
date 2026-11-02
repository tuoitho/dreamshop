package http

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cartclient "ecommerce.local/bff-service/internal/cart"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeCartClient struct {
	getFn             func(context.Context, string) (*cartv1.GetCartResponse, error)
	addFn             func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error)
	updateFn          func(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error)
	removeFn          func(context.Context, string, string) (*cartv1.RemoveItemResponse, error)
	clearFn           func(context.Context, string) (*cartv1.ClearCartResponse, error)
	getFavoritesFn    func(context.Context, string) (*cartv1.GetFavoritesResponse, error)
	addFavoriteFn     func(context.Context, string, string) (*cartv1.AddFavoriteResponse, error)
	removeFavoriteFn  func(context.Context, string, string) (*cartv1.RemoveFavoriteResponse, error)
	clearFavoritesFn  func(context.Context, string) (*cartv1.ClearFavoritesResponse, error)
}

func (f *fakeCartClient) GetCart(ctx context.Context, userID string) (*cartv1.GetCartResponse, error) {
	return f.getFn(ctx, userID)
}

func (f *fakeCartClient) AddItem(ctx context.Context, userID, productID, skuID string, quantity int32) (*cartv1.AddItemResponse, error) {
	return f.addFn(ctx, userID, productID, skuID, quantity)
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

func (f *fakeCartClient) GetFavorites(ctx context.Context, userID string) (*cartv1.GetFavoritesResponse, error) {
	return f.getFavoritesFn(ctx, userID)
}

func (f *fakeCartClient) AddFavorite(ctx context.Context, userID, productID string) (*cartv1.AddFavoriteResponse, error) {
	return f.addFavoriteFn(ctx, userID, productID)
}

func (f *fakeCartClient) RemoveFavorite(ctx context.Context, userID, productID string) (*cartv1.RemoveFavoriteResponse, error) {
	return f.removeFavoriteFn(ctx, userID, productID)
}

func (f *fakeCartClient) ClearFavorites(ctx context.Context, userID string) (*cartv1.ClearFavoritesResponse, error) {
	return f.clearFavoritesFn(ctx, userID)
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
		ItemId: "item-1", ProductId: "dt-1", SkuId: "dt-1-v2", VariantLabel: "16GB/512GB", Quantity: 2,
		Name: "Nova X", Brand: "Nova", CategoryName: "Phones",
		UnitPrice:     &commonv1.Money{AmountMinor: 18990000},
		OriginalPrice: &commonv1.Money{AmountMinor: 21990000},
		ImageEmoji:    "📱",
		ImageHue:      []uint32{220, 260}, ImageUrl: "https://example.test/dt-1.png",
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newCartTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	want := `{"data":{"items":[{"itemId":"item-1","productId":"dt-1","skuId":"dt-1-v2","variantLabel":"16GB/512GB","quantity":2,"name":"Nova X","brand":"Nova","categoryName":"Phones","unitPrice":18990000,"originalPrice":21990000,"imageEmoji":"📱","imageHue":[220,260],"imageUrl":"https://example.test/dt-1.png","variantColorHex":"#111111","status":"LIMITED","availableQuantity":3}],"degraded":true},"message":"Cart retrieved."}` + "\n"
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
			req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
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
	client := &fakeCartClient{addFn: func(ctx context.Context, userID, productID, skuID string, quantity int32) (*cartv1.AddItemResponse, error) {
		called = true
		if userID != "user-1" || productID != "dt-1" || skuID != "dt-1-v2" || quantity != 2 {
			t.Fatalf("unexpected add arguments: %q %q %q %d", userID, productID, skuID, quantity)
		}
		return &cartv1.AddItemResponse{Cart: &cartv1.Cart{}}, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", strings.NewReader(`{"productId":"dt-1","skuId":"dt-1-v2","quantity":2}`))
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

type traceRecordingCartServer struct {
	cartv1.UnimplementedCartServiceServer
	traceID string
}

func (s *traceRecordingCartServer) AddItem(ctx context.Context, _ *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(values) == 1 {
		s.traceID = values[0]
	}
	base := status.New(codes.Unavailable, "authorization unavailable")
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.cart.v1",
		Reason: "AUTHORIZATION_UNAVAILABLE",
	})
	if err != nil {
		return nil, base.Err()
	}
	return nil, withDetails.Err()
}

func newTraceRecordingCartClient(t *testing.T, server *traceRecordingCartServer) cartclient.Client {
	t.Helper()
	return newTraceRecordingCartServerClient(t, server)
}

func newTraceRecordingCartServerClient(t *testing.T, server cartv1.CartServiceServer) cartclient.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	client, err := cartclient.NewClient(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("create Cart client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestAddCartItemPropagatesResolvedTraceToCartAndErrorEnvelope(t *testing.T) {
	tests := []struct {
		name          string
		requestTrace  string
		wantRequestID bool
	}{
		{name: "valid caller trace", requestTrace: "trace-abc-123", wantRequestID: true},
		{name: "oversized caller trace", requestTrace: strings.Repeat("a", 65)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &traceRecordingCartServer{}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", strings.NewReader(`{"productId":"dt-1","skuId":"dt-1-v1","quantity":2}`))
			req.Header.Set("X-User-Id", "user-1")
			req.Header.Set("X-Trace-Id", test.requestTrace)
			rec := httptest.NewRecorder()

			newCartTestMux(t, newTraceRecordingCartClient(t, server)).ServeHTTP(rec, req)

			body := decodeBody(t, rec)
			traceID, ok := body["traceId"].(string)
			if rec.Code != http.StatusServiceUnavailable || !ok || traceID == "" {
				t.Fatalf("response = %d %s, want 503 with traceId", rec.Code, rec.Body.String())
			}
			if test.wantRequestID && traceID != test.requestTrace {
				t.Fatalf("response traceId = %q, want request trace %q", traceID, test.requestTrace)
			}
			if !test.wantRequestID && (traceID == test.requestTrace || len(traceID) > 64) {
				t.Fatalf("response traceId = %q, want bounded generated ID", traceID)
			}
			if server.traceID != traceID {
				t.Fatalf("Cart metadata trace = %q, want response trace %q", server.traceID, traceID)
			}
		})
	}
}

func TestAddCartItemRejectsNonPositiveQuantity(t *testing.T) {
	client := &fakeCartClient{addFn: func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) {
		t.Fatal("cart client must not be called")
		return nil, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", strings.NewReader(`{"productId":"dt-1","skuId":"dt-1-v2","quantity":0}`))
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
		{name: "unknown field", method: http.MethodPost, target: "/api/v1/cart/items", body: `{"productId":"dt-1","skuId":"dt-1-v1","quantity":2,"userId":"forged"}`, code: "INVALID_PARAMETER"},
		{name: "legacy variantLabel field", method: http.MethodPost, target: "/api/v1/cart/items", body: `{"productId":"dt-1","variantLabel":"red","quantity":2}`, code: "INVALID_PARAMETER"},
		{name: "missing skuId", method: http.MethodPost, target: "/api/v1/cart/items", body: `{"productId":"dt-1","quantity":2}`, code: "INVALID_PARAMETER"},
		{name: "trailing json", method: http.MethodPost, target: "/api/v1/cart/items", body: `{"productId":"dt-1","skuId":"dt-1-v1","quantity":2}{}`, code: "INVALID_PARAMETER"},
		{name: "blank quantity", method: http.MethodPost, target: "/api/v1/cart/items", body: `{"productId":"dt-1","skuId":"dt-1-v1","quantity":""}`, code: "INVALID_QUANTITY"},
		{name: "omitted quantity", method: http.MethodPatch, target: "/api/v1/cart/items/item-1", body: `{}`, code: "INVALID_QUANTITY"},
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
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/items/item-1", strings.NewReader(`{"quantity":4}`))
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
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/item-1", nil)
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
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cart", nil)
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
	newCartTestMux(t, client).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil))

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
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/item-1", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
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

func TestCartPermissionDeniedMapsToForbidden(t *testing.T) {
	client := &fakeCartClient{
		getFn:    func(context.Context, string) (*cartv1.GetCartResponse, error) { return nil, cartclient.ErrPermissionDenied },
		addFn:    func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) { return nil, cartclient.ErrPermissionDenied },
		updateFn: func(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error) { return nil, cartclient.ErrPermissionDenied },
		removeFn: func(context.Context, string, string) (*cartv1.RemoveItemResponse, error) { return nil, cartclient.ErrPermissionDenied },
		clearFn:  func(context.Context, string) (*cartv1.ClearCartResponse, error) { return nil, cartclient.ErrPermissionDenied },
	}
	mux := newCartTestMux(t, client)
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/cart", ""},
		{http.MethodPost, "/api/v1/cart/items", `{"productId":"dt-1","skuId":"dt-1-v1","quantity":2}`},
		{http.MethodPatch, "/api/v1/cart/items/item-1", `{"quantity":2}`},
		{http.MethodDelete, "/api/v1/cart/items/item-1", ""},
		{http.MethodDelete, "/api/v1/cart", ""},
	}
	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		body := decodeBody(t, rec)
		if rec.Code != http.StatusForbidden || body["code"] != "FORBIDDEN" ||
			body["message"] != "You do not have permission to access or modify the cart." {
			t.Fatalf("%s %s denied response = %d %s", route.method, route.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCartAuthorizationUnavailableMapsToServiceUnavailable(t *testing.T) {
	client := &fakeCartClient{
		getFn:    func(context.Context, string) (*cartv1.GetCartResponse, error) { return nil, cartclient.ErrAuthorizationUnavailable },
		addFn:    func(context.Context, string, string, string, int32) (*cartv1.AddItemResponse, error) { return nil, cartclient.ErrAuthorizationUnavailable },
		updateFn: func(context.Context, string, string, int32) (*cartv1.UpdateItemQuantityResponse, error) { return nil, cartclient.ErrAuthorizationUnavailable },
		removeFn: func(context.Context, string, string) (*cartv1.RemoveItemResponse, error) { return nil, cartclient.ErrAuthorizationUnavailable },
		clearFn:  func(context.Context, string) (*cartv1.ClearCartResponse, error) { return nil, cartclient.ErrAuthorizationUnavailable },
	}
	mux := newCartTestMux(t, client)
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/cart", ""},
		{http.MethodPost, "/api/v1/cart/items", `{"productId":"dt-1","skuId":"dt-1-v1","quantity":2}`},
		{http.MethodPatch, "/api/v1/cart/items/item-1", `{"quantity":2}`},
		{http.MethodDelete, "/api/v1/cart/items/item-1", ""},
		{http.MethodDelete, "/api/v1/cart", ""},
	}
	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		body := decodeBody(t, rec)
		if rec.Code != http.StatusServiceUnavailable || body["code"] != "AUTHORIZATION_SERVICE_UNAVAILABLE" {
			t.Fatalf("%s %s unavailable response = %d %s", route.method, route.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCartUnknownSubroutesAndLegacyRootReturnNotFound(t *testing.T) {
	mux := newCartTestMux(t, &fakeCartClient{})
	for _, path := range []string{"/", "/items", "/api/v1/cart/unknown"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-User-Id", "user-1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %s to return 404, got %d", path, rec.Code)
		}
	}
}
