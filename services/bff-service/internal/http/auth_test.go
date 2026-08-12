package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireCallerInvokesNextWithTrimmedCallerID(t *testing.T) {
	called := false
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got = callerID(r)
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", " user-1 ")
	rec := httptest.NewRecorder()

	RequireCaller(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if got != "user-1" {
		t.Fatalf("expected caller id user-1, got %q", got)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestRequireCallerRejectsBlankHeaderWithErrorEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	RequireCaller(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["code"] != "UNAUTHORIZED" {
		t.Fatalf("expected code UNAUTHORIZED, got %v", body["code"])
	}
	if body["message"] != "Authentication is required." {
		t.Fatalf("expected authentication message, got %v", body["message"])
	}
}
