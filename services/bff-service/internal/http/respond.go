package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/grpc/metadata"
)

const (
	traceHeader      = "X-Trace-Id"
	traceMetadataKey = "x-trace-id"
	maxTraceIDLength = 64
)

type traceIDKey struct{}

// envelope is the shared success response shape documented in the repository
// REST contract.
type envelope struct {
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// errorBody is the shared error response shape. Errors is omitted when no
// structured field details exist.
type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
	TraceID string            `json:"traceId"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeEnvelope(w http.ResponseWriter, status int, data any, message string) {
	writeJSON(w, status, envelope{Data: data, Message: message})
}

type statusRecorder struct {
	http.ResponseWriter
	status    int
	errorCode string
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) setErrorCode(code string) {
	r.errorCode = code
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, fields map[string]string) {
	if recorder, ok := w.(interface{ setErrorCode(string) }); ok {
		recorder.setErrorCode(code)
	}
	writeJSON(w, status, errorBody{
		Code:    code,
		Message: message,
		Errors:  fields,
		TraceID: requestTraceID(r),
	})
}

func correlateTrace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		traceID := resolveTraceID(r.Header.Get(traceHeader))
		ctx := context.WithValue(r.Context(), traceIDKey{}, traceID)
		outgoing, _ := metadata.FromOutgoingContext(ctx)
		outgoing = outgoing.Copy()
		outgoing.Set(traceMetadataKey, traceID)
		ctx = metadata.NewOutgoingContext(ctx, outgoing)
		w.Header().Set(traceHeader, traceID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"trace_id", traceID,
		}
		if rec.errorCode != "" {
			attrs = append(attrs, "error_code", rec.errorCode)
		}
		switch {
		case rec.status >= http.StatusInternalServerError:
			slog.ErrorContext(ctx, "http request completed", attrs...)
		case rec.status >= http.StatusBadRequest:
			slog.WarnContext(ctx, "http request completed", attrs...)
		default:
			slog.InfoContext(ctx, "http request completed", attrs...)
		}
	})
}

// requestTraceID returns the request's resolved correlation ID. Handlers not
// using correlation middleware still receive a bounded ID in error responses.
func requestTraceID(r *http.Request) string {
	if id, ok := r.Context().Value(traceIDKey{}).(string); ok {
		return id
	}
	return resolveTraceID(r.Header.Get(traceHeader))
}

func resolveTraceID(candidate string) string {
	if validTraceID(candidate) {
		return candidate
	}
	return newTraceID()
}

func validTraceID(candidate string) bool {
	if len(candidate) == 0 || len(candidate) > maxTraceIDLength {
		return false
	}
	for i := range len(candidate) {
		value := candidate[i]
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '-' || value == '_' || value == '.' {
			continue
		}
		return false
	}
	return true
}

func newTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
