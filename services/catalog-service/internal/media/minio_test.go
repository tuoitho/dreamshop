package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPresignPutLocalAndRelativePublicBaseURL(t *testing.T) {
	fixedNow := time.Date(2026, 10, 27, 5, 0, 0, 0, time.UTC)

	localClient, err := NewMinIOClient(Config{
		Endpoint:      "localhost:9000",
		AccessKey:     "minioadmin",
		SecretKey:     "minioadmin",
		Bucket:        "product-media",
		PublicBaseURL: "http://localhost:9000/product-media",
		UseSSL:        false,
	})
	if err != nil {
		t.Fatalf("NewMinIOClient(local): %v", err)
	}
	localClient.now = func() time.Time { return fixedNow }

	signedLocal, err := localClient.PresignPut("products/2026/10/test-id.webp", "image/webp", 12000, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut(local): %v", err)
	}
	if signedLocal.PublicURL != "http://localhost:9000/product-media/products/2026/10/test-id.webp" {
		t.Fatalf("unexpected local PublicURL: %s", signedLocal.PublicURL)
	}
	parsedLocal, err := url.Parse(signedLocal.UploadURL)
	if err != nil {
		t.Fatalf("parse local UploadURL: %v", err)
	}
	if parsedLocal.Host != "localhost:9000" || parsedLocal.Path != "/product-media/products/2026/10/test-id.webp" {
		t.Fatalf("unexpected local UploadURL: %s", signedLocal.UploadURL)
	}
	if got := parsedLocal.Query().Get("X-Amz-SignedHeaders"); got != "content-length;content-type;host" {
		t.Fatalf("X-Amz-SignedHeaders = %q, want content-length;content-type;host", got)
	}
	if got := parsedLocal.Query().Get("X-Amz-Expires"); got != "300" {
		t.Fatalf("X-Amz-Expires = %q, want 300", got)
	}

	prodClient, err := NewMinIOClient(Config{
		Endpoint:      "minio:9000",
		AccessKey:     "prod-access",
		SecretKey:     "prod-secret",
		Bucket:        "product-media",
		PublicBaseURL: "/product-media",
		UseSSL:        false,
	})
	if err != nil {
		t.Fatalf("NewMinIOClient(prod): %v", err)
	}
	prodClient.now = func() time.Time { return fixedNow }

	signedProd, err := prodClient.PresignPut("products/2026/10/clip.mp4", "video/mp4", 480000, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut(prod): %v", err)
	}
	if signedProd.PublicURL != "/product-media/products/2026/10/clip.mp4" {
		t.Fatalf("unexpected prod PublicURL: %s", signedProd.PublicURL)
	}
	if !strings.HasPrefix(signedProd.UploadURL, "/product-media/products/2026/10/clip.mp4?") {
		t.Fatalf("expected path-relative UploadURL for Kong ingress, got %s", signedProd.UploadURL)
	}
}

func TestEnsureBucketAndRemoveObjectAgainstS3Server(t *testing.T) {
	var methods []string
	bucketExists := false
	deleteStatus := http.StatusNoContent

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.RequestURI())
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("missing SigV4 Authorization header on %s %s", r.Method, r.URL.RequestURI())
		}
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/product-media":
			if !bucketExists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/product-media" && r.URL.RawQuery == "":
			bucketExists = true
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/product-media" && strings.Contains(r.URL.RawQuery, "policy"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/product-media/products/2026/10/orphan.webp":
			w.WriteHeader(deleteStatus)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	endpoint := strings.TrimPrefix(srv.URL, "http://")
	client, err := NewMinIOClient(Config{
		Endpoint:      endpoint,
		AccessKey:     "minioadmin",
		SecretKey:     "minioadmin",
		Bucket:        "product-media",
		PublicBaseURL: srv.URL + "/product-media",
		HTTPClient:    srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewMinIOClient: %v", err)
	}

	if err := client.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if len(methods) != 3 {
		t.Fatalf("expected HEAD + PUT bucket + PUT policy, got %v", methods)
	}

	if err := client.RemoveObject(context.Background(), "products/2026/10/orphan.webp"); err != nil {
		t.Fatalf("RemoveObject(204): %v", err)
	}

	deleteStatus = http.StatusNotFound
	if err := client.RemoveObject(context.Background(), "products/2026/10/orphan.webp"); err != nil {
		t.Fatalf("RemoveObject(404 idempotent): %v", err)
	}

	deleteStatus = http.StatusInternalServerError
	if err := client.RemoveObject(context.Background(), "products/2026/10/orphan.webp"); err == nil {
		t.Fatal("expected RemoveObject(500) to return error")
	}

	// Subsequent EnsureBucket calls must short-circuit once bootstrap succeeded.
	beforeCalls := len(methods)
	if err := client.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("EnsureBucket(cached): %v", err)
	}
	if len(methods) != beforeCalls {
		t.Fatalf("expected cached EnsureBucket to make 0 HTTP calls, got %d new calls", len(methods)-beforeCalls)
	}
}
