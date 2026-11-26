package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrelateTraceLogsHTTPRequestsByStatusLevel(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		errorCode     string
		wantLevel     string
		wantErrorCode string
	}{
		{
			name:      "2xx logs at INFO without error_code",
			status:    http.StatusOK,
			wantLevel: "INFO",
		},
		{
			name:          "4xx logs at WARN with error_code",
			status:        http.StatusConflict,
			errorCode:     "OUT_OF_STOCK",
			wantLevel:     "WARN",
			wantErrorCode: "OUT_OF_STOCK",
		},
		{
			name:          "5xx logs at ERROR with error_code",
			status:        http.StatusServiceUnavailable,
			errorCode:     "INVENTORY_UNAVAILABLE",
			wantLevel:     "ERROR",
			wantErrorCode: "INVENTORY_UNAVAILABLE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			handler := correlateTrace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.errorCode != "" {
					writeError(w, r, tc.status, tc.errorCode, "failure", nil)
					return
				}
				writeEnvelope(w, tc.status, map[string]string{"ok": "true"}, "ok")
			}))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
			req.Header.Set("X-Trace-Id", "trace-test-123")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("expected JSON access log entry, got %q (err: %v)", buf.String(), err)
			}
			if got := entry["level"]; got != tc.wantLevel {
				t.Fatalf("level = %v, want %s", got, tc.wantLevel)
			}
			if got := entry["method"]; got != http.MethodPost {
				t.Fatalf("method = %v, want POST", got)
			}
			if got := entry["path"]; got != "/api/v1/orders" {
				t.Fatalf("path = %v, want /api/v1/orders", got)
			}
			if got, ok := entry["status"].(float64); !ok || int(got) != tc.status {
				t.Fatalf("status = %v, want %d", entry["status"], tc.status)
			}
			if got := entry["trace_id"]; got != "trace-test-123" {
				t.Fatalf("trace_id = %v, want trace-test-123", got)
			}
			if _, ok := entry["duration_ms"].(float64); !ok {
				t.Fatalf("expected numeric duration_ms in %v", entry)
			}
			if tc.wantErrorCode == "" {
				if _, exists := entry["error_code"]; exists {
					t.Fatalf("unexpected error_code in %v", entry)
				}
			} else if got := entry["error_code"]; got != tc.wantErrorCode {
				t.Fatalf("error_code = %v, want %s", got, tc.wantErrorCode)
			}
		})
	}
}
