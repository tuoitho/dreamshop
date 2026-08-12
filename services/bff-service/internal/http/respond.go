package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

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

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, errorBody{
		Code:    code,
		Message: message,
		Errors:  fields,
		TraceID: requestTraceID(r),
	})
}

// requestTraceID returns the caller-provided correlation id when present,
// otherwise generates a short random hex id for the response.
func requestTraceID(r *http.Request) string {
	if id := r.Header.Get("X-Trace-Id"); id != "" {
		return id
	}
	return newTraceID()
}

func newTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
