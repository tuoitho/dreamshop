package http

import (
	"encoding/json"
	"net/http"
)

type HealthHandler struct {
	ServiceName string
}

func NewHealthHandler(serviceName string) *HealthHandler {
	return &HealthHandler{ServiceName: serviceName}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/healthz" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": h.ServiceName,
		"status":  "ok",
	})
}
