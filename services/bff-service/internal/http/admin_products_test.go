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
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
			req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(tc.body))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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

	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
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
