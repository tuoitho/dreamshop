package http

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newAdminProductsTestMux(t *testing.T, client catalog.Client) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	NewAdminProductsHandler(client).Register(mux)
	return mux
}

func TestAdminSoftDeleteProductWithoutCallerReturnsUnauthorized(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			t.Fatal("catalog client must not be called without caller")
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "UNAUTHORIZED" {
		t.Fatalf("expected code UNAUTHORIZED, got %v", body["code"])
	}
}

func TestAdminSoftDeleteProductSuccessForwardsCallerIdentity(t *testing.T) {
	called := false
	client := &fakeCatalogClient{
		softDeleteFn: func(ctx context.Context, userID, id string, deleted bool) error {
			called = true
			if userID != "admin-1" || id != "dt-1" || !deleted {
				t.Fatalf("unexpected soft-delete arguments: user=%q id=%q deleted=%v", userID, id, deleted)
			}
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected successful soft-delete call, got status %d body %s", rec.Code, rec.Body.String())
	}
	want := `{"data":{"id":"dt-1","deleted":true},"message":"Product availability updated."}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected body\n got: %s\nwant: %s", got, want)
	}
}

func TestAdminSoftDeleteProductRejectsMalformedOrMissingDeleted(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"deleted":`},
		{name: "missing deleted", body: `{}`},
		{name: "forged userId field", body: `{"deleted":true,"userId":"forged"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCatalogClient{
				softDeleteFn: func(context.Context, string, string, bool) error {
					t.Fatal("catalog client must not be called")
					return nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(tc.body))
			req.Header.Set("X-User-Id", "admin-1")
			rec := httptest.NewRecorder()

			newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body["code"] != "INVALID_REQUEST" {
				t.Fatalf("expected code INVALID_REQUEST, got %v", body["code"])
			}
		})
	}
}

func TestAdminSoftDeleteProductPermissionDeniedMapsToForbidden(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			return catalog.ErrPermissionDenied
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "customer-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	body := decodeBody(t, rec)
	if rec.Code != http.StatusForbidden || body["code"] != "FORBIDDEN" {
		t.Fatalf("denied response = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSoftDeleteProductAuthorizationUnavailableMapsToServiceUnavailable(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			return catalog.ErrAuthorizationUnavailable
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	body := decodeBody(t, rec)
	if rec.Code != http.StatusServiceUnavailable || body["code"] != "AUTHORIZATION_SERVICE_UNAVAILABLE" {
		t.Fatalf("unavailable response = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSoftDeleteProductNotFoundMapsToStableError(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			return catalog.ErrProductNotFound
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected code PRODUCT_NOT_FOUND, got %v", body["code"])
	}
}

func TestAdminSoftDeleteProductCatalogInvalidArgumentMapsToBadRequest(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			return status.Error(codes.InvalidArgument, "id is required")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "INVALID_REQUEST" {
		t.Fatalf("expected code INVALID_REQUEST, got %v", body["code"])
	}
}

func TestAdminSoftDeleteProductCatalogUnavailableMapsToBadGateway(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, string, bool) error {
			return status.Error(codes.Unavailable, "catalog down")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "CATALOG_UNAVAILABLE" {
		t.Fatalf("expected code CATALOG_UNAVAILABLE, got %v", body["code"])
	}
}

type traceRecordingCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	traceID string
}

func (s *traceRecordingCatalogServer) SoftDeleteProduct(ctx context.Context, _ *catalogv1.SoftDeleteProductRequest) (*catalogv1.SoftDeleteProductResponse, error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(values) == 1 {
		s.traceID = values[0]
	}
	base := status.New(codes.Unavailable, "authorization unavailable")
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.catalog.v1",
		Reason: "AUTHORIZATION_UNAVAILABLE",
	})
	if err != nil {
		return nil, base.Err()
	}
	return nil, withDetails.Err()
}

func TestAdminSoftDeleteProductPropagatesResolvedTraceToCatalogAndErrorEnvelope(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &traceRecordingCatalogServer{}
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	client, err := catalog.NewClient(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("create catalog client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	req.Header.Set("X-Trace-Id", "trace-catalog-789")
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	body := decodeBody(t, rec)
	if rec.Code != http.StatusServiceUnavailable || body["traceId"] != "trace-catalog-789" {
		t.Fatalf("response = %d %s, want 503 with traceId trace-catalog-789", rec.Code, rec.Body.String())
	}
	if server.traceID != "trace-catalog-789" {
		t.Fatalf("Catalog metadata trace = %q, want trace-catalog-789", server.traceID)
	}
}

func TestAdminLegacySoftDeletePathReturnsNotFound(t *testing.T) {
	mux := newAdminProductsTestMux(t, &fakeCatalogClient{})
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("X-User-Id", "admin-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected legacy /products/dt-1/delete to return 404, got %d", rec.Code)
	}
}

func TestAdminProductsListPresignCreateUpdateAndStockDegradation(t *testing.T) {
	cat := &fakeCatalogClient{
		listAdminFn: func(_ context.Context, req *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error) {
			return &catalogv1.ListAdminProductsResponse{
				Items: []*catalogv1.AdminProductSummary{
					{
						Id:           "dt-1",
						Name:         "Nova X15",
						Brand:        "Nova",
						CategoryId:   "dien-thoai",
						CategoryName: "Điện thoại",
						Price:        money(18990000),
						VariantCount: 2,
						VideoCount:   1,
					},
				},
				Page:         1,
				PageSize:     20,
				TotalItems:   1,
				TotalPages:   1,
				ActiveCount:  1,
				DeletedCount: 0,
				Categories: []*catalogv1.CategoryMetric{
					{CategoryId: "dien-thoai", CategoryName: "Điện thoại", ActiveProductCount: 1},
				},
			}, nil
		},
		createMediaFn: func(_ context.Context, req *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
			return &catalogv1.CreateProductMediaUploadResponse{
				UploadId:         "u-123",
				MediaType:        "VIDEO",
				UploadUrl:        "/product-media/products/2026/10/u-123.mp4?X-Amz-Signature=sig",
				PublicUrl:        "/product-media/products/2026/10/u-123.mp4",
				ObjectKey:        "products/2026/10/u-123.mp4",
				ExpiresInSeconds: 300,
			}, nil
		},
		createProductFn: func(_ context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{
				Id:           "nova-x15",
				Name:         req.GetName(),
				Brand:        req.GetBrand(),
				CategoryId:   req.GetCategoryId(),
				CategoryName: req.GetCategoryName(),
				Price:        req.GetPrice(),
				Images:       req.GetImages(),
				Videos:       req.GetVideos(),
				Variants:     req.GetVariants(),
			}, nil
		},
		updateProductFn: func(_ context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{
				Id:     req.GetId(),
				Name:   req.GetName(),
				Price:  req.GetPrice(),
				Images: req.GetImages(),
				Videos: req.GetVideos(),
			}, nil
		},
		getAdminProductFn: func(_ context.Context, _, id string) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{
				Id:     id,
				Name:   "Nova X15",
				Price:  money(18990000),
				Images: []string{"/product-media/img.webp"},
				Videos: []string{"/product-media/vid.mp4"},
			}, nil
		},
	}

	// Degrade stock cleanly when operator lacks inventory.view
	inv := &fakeInventoryClient{
		batchGetFn: func(context.Context, string, []string) ([]*inventoryv1.StockItem, error) {
			return nil, inventory.ErrPermissionDenied
		},
	}

	mux := http.NewServeMux()
	NewAdminProductsHandler(cat, inv).Register(mux)

	// 1. GET /api/v1/admin/products degrades stock without failing product listing (supports pageSize up to 200)
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products?status=all&pageSize=200", nil)
	listReq.Header.Set("X-User-Id", "admin-1")
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /admin/products status = %d: %s", listRec.Code, listRec.Body.String())
	}
	listData := decodeBody(t, listRec)["data"].(map[string]any)
	if listData["stockDegraded"] != true {
		t.Fatalf("expected stockDegraded=true, got %v", listData["stockDegraded"])
	}

	// 2. POST /api/v1/admin/products/media/presign
	presignReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/media/presign",
		strings.NewReader(`{"filename":"clip.mp4","contentType":"video/mp4","sizeBytes":1048576}`))
	presignReq.Header.Set("X-User-Id", "admin-1")
	presignRec := httptest.NewRecorder()
	mux.ServeHTTP(presignRec, presignReq)
	if presignRec.Code != http.StatusOK {
		t.Fatalf("POST /media/presign status = %d: %s", presignRec.Code, presignRec.Body.String())
	}
	presignData := decodeBody(t, presignRec)["data"].(map[string]any)
	if presignData["uploadId"] != "u-123" || presignData["mediaType"] != "VIDEO" {
		t.Fatalf("unexpected presign data: %+v", presignData)
	}

	// 3. POST /api/v1/admin/products -> 201 Created
	createBody := `{
		"name":"Nova X15",
		"brand":"Nova",
		"categoryId":"dien-thoai",
		"categoryName":"Điện thoại",
		"price":18990000,
		"images":["/product-media/img.webp"],
		"videos":["/product-media/vid.mp4"],
		"variants":[{"label":"Đen","colorHex":"#111827"}]
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(createBody))
	createReq.Header.Set("X-User-Id", "admin-1")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /admin/products status = %d: %s", createRec.Code, createRec.Body.String())
	}

	// 4. PUT /api/v1/admin/products/{id} -> 200 OK
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/nova-x15", strings.NewReader(createBody))
	putReq.Header.Set("X-User-Id", "admin-1")
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT /admin/products/nova-x15 status = %d: %s", putRec.Code, putRec.Body.String())
	}

	// 5. GET /api/v1/admin/products/{id} -> 200 OK with videos
	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/nova-x15", nil)
	detailReq.Header.Set("X-User-Id", "admin-1")
	detailRec := httptest.NewRecorder()
	mux.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("GET /admin/products/nova-x15 status = %d: %s", detailRec.Code, detailRec.Body.String())
	}

	// 6. Conflict mappings: PRODUCT_ID_EXISTS & MEDIA_STORAGE_UNAVAILABLE
	cat.createProductFn = func(context.Context, *catalogv1.CreateProductRequest) (*catalogv1.GetProductResponse, error) {
		return nil, catalog.ErrProductExists
	}
	dupReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(createBody))
	dupReq.Header.Set("X-User-Id", "admin-1")
	dupRec := httptest.NewRecorder()
	mux.ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusConflict || decodeBody(t, dupRec)["code"] != "PRODUCT_ID_EXISTS" {
		t.Fatalf("expected 409 PRODUCT_ID_EXISTS, got %d %s", dupRec.Code, dupRec.Body.String())
	}

	cat.createMediaFn = func(context.Context, *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
		return nil, catalog.ErrMediaStorageUnavailable
	}
	downReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/media/presign",
		strings.NewReader(`{"filename":"a.png","contentType":"image/png","sizeBytes":512}`))
	downReq.Header.Set("X-User-Id", "admin-1")
	downRec := httptest.NewRecorder()
	mux.ServeHTTP(downRec, downReq)
	if downRec.Code != http.StatusServiceUnavailable || decodeBody(t, downRec)["code"] != "MEDIA_STORAGE_UNAVAILABLE" {
		t.Fatalf("expected 503 MEDIA_STORAGE_UNAVAILABLE, got %d %s", downRec.Code, downRec.Body.String())
	}
}
