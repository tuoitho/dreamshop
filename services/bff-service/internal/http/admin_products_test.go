package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce.local/bff-service/internal/catalog"
)

func newAdminProductsTestMux(t *testing.T, client catalog.Client) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	NewAdminProductsHandler(client).Register(mux)
	return mux
}

func TestAdminSoftDeleteProductSuccess(t *testing.T) {
	called := false
	client := &fakeCatalogClient{
		softDeleteFn: func(ctx context.Context, id string, deleted bool) error {
			called = true
			if id != "dt-1" || !deleted {
				t.Fatalf("unexpected soft-delete arguments: %q %v", id, deleted)
			}
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	req.Header.Set("Content-Type", "application/json")
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCatalogClient{
				softDeleteFn: func(context.Context, string, bool) error {
					t.Fatal("catalog client must not be called")
					return nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(tc.body))
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

func TestAdminSoftDeleteProductNotFoundMapsToStableError(t *testing.T) {
	client := &fakeCatalogClient{
		softDeleteFn: func(context.Context, string, bool) error {
			return catalog.ErrProductNotFound
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/products/dt-1/delete", strings.NewReader(`{"deleted":true}`))
	rec := httptest.NewRecorder()

	newAdminProductsTestMux(t, client).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["code"] != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected code PRODUCT_NOT_FOUND, got %v", body["code"])
	}
}
