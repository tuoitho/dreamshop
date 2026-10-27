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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newFavoritesTestMux(t *testing.T, client cartclient.Client) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	NewFavoritesHandler(client).Register(mux)
	return mux
}

func favoriteItemFixture() *cartv1.FavoriteItem {
	return &cartv1.FavoriteItem{
		ItemId:            "fav-1",
		ProductId:         "dt-1",
		Name:              "iPhone 16 Pro",
		Brand:             "Apple",
		CategoryId:        "dien-thoai",
		CategoryName:      "Điện thoại",
		Price:             &commonv1.Money{AmountMinor: 26990000},
		OriginalPrice:     &commonv1.Money{AmountMinor: 29990000},
		Rating:            4.9,
		ReviewCount:       128,
		SoldCount:         540,
		ImageUrl:          "https://example.test/dt-1.png",
		ImageEmoji:        "📱",
		ImageHue:          []uint32{210, 250},
		Badge:             "hot",
		Variants:          []*cartv1.FavoriteVariant{{Label: "Đen Titan 256GB", ColorHex: "#1c1c1e"}},
		Status:            cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_AVAILABLE,
		AvailableQuantity: 12,
	}
}

func TestGetFavoritesMapsExactCamelCaseEnvelopeAndStatuses(t *testing.T) {
	unknownItem := favoriteItemFixture()
	unknownItem.ItemId = "fav-2"
	unknownItem.ProductId = "dt-2"
	unknownItem.ImageHue = nil
	unknownItem.Variants = nil
	unknownItem.Status = cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNKNOWN
	unknownItem.AvailableQuantity = 0

	client := &fakeCartClient{
		getFavoritesFn: func(_ context.Context, userID string) (*cartv1.GetFavoritesResponse, error) {
			if userID != "user-1" {
				t.Fatalf("userID = %q, want user-1", userID)
			}
			return &cartv1.GetFavoritesResponse{
				Favorites: &cartv1.FavoriteList{
					Items:    []*cartv1.FavoriteItem{favoriteItemFixture(), unknownItem},
					Degraded: true,
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/favorites", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	newFavoritesTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["message"] != "Favorites retrieved." {
		t.Fatalf("message = %v, want Favorites retrieved.", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["degraded"] != true {
		t.Fatalf("degraded = %v, want true", data["degraded"])
	}
	items := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	first := items[0].(map[string]any)
	if first["itemId"] != "fav-1" ||
		first["productId"] != "dt-1" ||
		first["name"] != "iPhone 16 Pro" ||
		first["brand"] != "Apple" ||
		first["categoryId"] != "dien-thoai" ||
		first["categoryName"] != "Điện thoại" ||
		first["price"] != float64(26990000) ||
		first["originalPrice"] != float64(29990000) ||
		first["rating"] != 4.9 ||
		first["reviewCount"] != float64(128) ||
		first["soldCount"] != float64(540) ||
		first["imageUrl"] != "https://example.test/dt-1.png" ||
		first["imageEmoji"] != "📱" ||
		first["badge"] != "hot" ||
		first["status"] != "AVAILABLE" ||
		first["availableQuantity"] != float64(12) {
		t.Fatalf("first item JSON mismatch: %#v", first)
	}
	variants := first["variants"].([]any)
	if len(variants) != 1 || variants[0].(map[string]any)["label"] != "Đen Titan 256GB" || variants[0].(map[string]any)["colorHex"] != "#1c1c1e" {
		t.Fatalf("variants JSON mismatch: %#v", variants)
	}
	second := items[1].(map[string]any)
	if second["status"] != "UNKNOWN" {
		t.Fatalf("second item status = %v, want UNKNOWN", second["status"])
	}
	if _, ok := second["imageHue"].([]any); !ok {
		t.Fatalf("imageHue = %#v, want empty JSON array", second["imageHue"])
	}
	if _, ok := second["variants"].([]any); !ok {
		t.Fatalf("variants = %#v, want empty JSON array", second["variants"])
	}
}

func TestFavoritesMutationsSucceedWithExactMessages(t *testing.T) {
	var addedProduct, removedProduct, clearedUser string
	client := &fakeCartClient{
		addFavoriteFn: func(_ context.Context, userID, productID string) (*cartv1.AddFavoriteResponse, error) {
			if userID != "user-1" {
				t.Fatalf("AddFavorite userID = %q", userID)
			}
			addedProduct = productID
			return &cartv1.AddFavoriteResponse{Favorites: &cartv1.FavoriteList{Items: []*cartv1.FavoriteItem{favoriteItemFixture()}}}, nil
		},
		removeFavoriteFn: func(_ context.Context, userID, productID string) (*cartv1.RemoveFavoriteResponse, error) {
			if userID != "user-1" {
				t.Fatalf("RemoveFavorite userID = %q", userID)
			}
			removedProduct = productID
			return &cartv1.RemoveFavoriteResponse{Favorites: &cartv1.FavoriteList{}}, nil
		},
		clearFavoritesFn: func(_ context.Context, userID string) (*cartv1.ClearFavoritesResponse, error) {
			clearedUser = userID
			return &cartv1.ClearFavoritesResponse{Favorites: &cartv1.FavoriteList{}}, nil
		},
	}
	mux := newFavoritesTestMux(t, client)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/favorites/items", strings.NewReader(`{"productId":"  dt-1  "}`))
	addReq.Header.Set("X-User-Id", "user-1")
	addRec := httptest.NewRecorder()
	mux.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK || addedProduct != "dt-1" || decodeBody(t, addRec)["message"] != "Product added to favorites." {
		t.Fatalf("POST /api/v1/favorites/items = %d %s (product=%q)", addRec.Code, addRec.Body.String(), addedProduct)
	}

	remReq := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/items/dt-1", nil)
	remReq.Header.Set("X-User-Id", "user-1")
	remRec := httptest.NewRecorder()
	mux.ServeHTTP(remRec, remReq)
	if remRec.Code != http.StatusOK || removedProduct != "dt-1" || decodeBody(t, remRec)["message"] != "Product removed from favorites." {
		t.Fatalf("DELETE /api/v1/favorites/items/dt-1 = %d %s (product=%q)", remRec.Code, remRec.Body.String(), removedProduct)
	}

	clrReq := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites", nil)
	clrReq.Header.Set("X-User-Id", "user-1")
	clrRec := httptest.NewRecorder()
	mux.ServeHTTP(clrRec, clrReq)
	if clrRec.Code != http.StatusOK || clearedUser != "user-1" || decodeBody(t, clrRec)["message"] != "Favorites cleared." {
		t.Fatalf("DELETE /api/v1/favorites = %d %s (user=%q)", clrRec.Code, clrRec.Body.String(), clearedUser)
	}
}

func TestFavoritesRoutesRequireCallerAndValidatePayloads(t *testing.T) {
	mux := newFavoritesTestMux(t, &fakeCartClient{
		getFavoritesFn: func(context.Context, string) (*cartv1.GetFavoritesResponse, error) {
			t.Fatal("GetFavorites must not be called")
			return nil, nil
		},
		addFavoriteFn: func(context.Context, string, string) (*cartv1.AddFavoriteResponse, error) {
			t.Fatal("AddFavorite must not be called")
			return nil, nil
		},
		removeFavoriteFn: func(context.Context, string, string) (*cartv1.RemoveFavoriteResponse, error) {
			t.Fatal("RemoveFavorite must not be called")
			return nil, nil
		},
		clearFavoritesFn: func(context.Context, string) (*cartv1.ClearFavoritesResponse, error) {
			t.Fatal("ClearFavorites must not be called")
			return nil, nil
		},
	})

	for _, unauth := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/favorites"},
		{http.MethodPost, "/api/v1/favorites/items"},
		{http.MethodDelete, "/api/v1/favorites/items/dt-1"},
		{http.MethodDelete, "/api/v1/favorites"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(unauth.method, unauth.path, nil))
		if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["code"] != "UNAUTHORIZED" {
			t.Fatalf("%s %s without caller = %d %s", unauth.method, unauth.path, rec.Code, rec.Body.String())
		}
	}

	overlong := strings.Repeat("a", 129)
	for _, bad := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"malformed json", http.MethodPost, "/api/v1/favorites/items", `{"productId":`},
		{"unknown field", http.MethodPost, "/api/v1/favorites/items", `{"productId":"dt-1","userId":"forged"}`},
		{"trailing json", http.MethodPost, "/api/v1/favorites/items", `{"productId":"dt-1"}{}`},
		{"blank productId", http.MethodPost, "/api/v1/favorites/items", `{"productId":"   "}`},
		{"overlong body productId", http.MethodPost, "/api/v1/favorites/items", `{"productId":"` + overlong + `"}`},
		{"blank path productId", http.MethodDelete, "/api/v1/favorites/items/%20%20", ""},
		{"overlong path productId", http.MethodDelete, "/api/v1/favorites/items/" + overlong, ""},
	} {
		t.Run(bad.name, func(t *testing.T) {
			req := httptest.NewRequest(bad.method, bad.path, strings.NewReader(bad.body))
			req.Header.Set("X-User-Id", "user-1")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["code"] != "INVALID_PARAMETER" {
				t.Fatalf("%s = %d %s, want 400 INVALID_PARAMETER", bad.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFavoritesErrorMappingAndTracePropagation(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"permission denied", cartclient.ErrPermissionDenied, http.StatusForbidden, "FORBIDDEN"},
		{"product not found", cartclient.ErrProductNotFound, http.StatusNotFound, "PRODUCT_NOT_FOUND"},
		{"product unavailable", cartclient.ErrProductUnavailable, http.StatusConflict, "PRODUCT_UNAVAILABLE"},
		{"auth unavailable", cartclient.ErrAuthorizationUnavailable, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE"},
		{"invalid argument", status.Error(codes.InvalidArgument, "bad user"), http.StatusBadRequest, "INVALID_PARAMETER"},
		{"downstream unavailable", status.Error(codes.Unavailable, "cart down"), http.StatusBadGateway, "FAVORITES_UNAVAILABLE"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newFavoritesTestMux(t, &fakeCartClient{
				addFavoriteFn: func(context.Context, string, string) (*cartv1.AddFavoriteResponse, error) {
					return nil, tc.err
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/favorites/items", strings.NewReader(`{"productId":"dt-1"}`))
			req.Header.Set("X-User-Id", "user-1")
			req.Header.Set("X-Trace-Id", "fav-trace-999")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			body := decodeBody(t, rec)
			if rec.Code != tc.wantStatus || body["code"] != tc.wantCode || body["traceId"] != "fav-trace-999" {
				t.Fatalf("%s = %d %s, want %d %s with trace fav-trace-999", tc.name, rec.Code, rec.Body.String(), tc.wantStatus, tc.wantCode)
			}
		})
	}

	server := &traceRecordingFavoriteServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/favorites", nil)
	req.Header.Set("X-User-Id", "user-1")
	req.Header.Set("X-Trace-Id", "fav-grpc-trace")
	rec := httptest.NewRecorder()
	newFavoritesTestMux(t, newTraceRecordingFavoriteClient(t, server)).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || server.traceID != "fav-grpc-trace" {
		t.Fatalf("trace propagation = status %d serverTrace=%q, want 503 fav-grpc-trace", rec.Code, server.traceID)
	}
}

type traceRecordingFavoriteServer struct {
	cartv1.UnimplementedCartServiceServer
	traceID string
}

func (s *traceRecordingFavoriteServer) GetFavorites(ctx context.Context, _ *cartv1.GetFavoritesRequest) (*cartv1.GetFavoritesResponse, error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(values) == 1 {
		s.traceID = values[0]
	}
	base := status.New(codes.Unavailable, "auth down")
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.cart.v1",
		Reason: "AUTHORIZATION_UNAVAILABLE",
	})
	if err != nil {
		return nil, base.Err()
	}
	return nil, withDetails.Err()
}

func newTraceRecordingFavoriteClient(t *testing.T, server *traceRecordingFavoriteServer) cartclient.Client {
	t.Helper()
	return newTraceRecordingCartServerClient(t, server)
}
