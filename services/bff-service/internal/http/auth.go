package http

import (
	"context"
	"net/http"
	"strings"
)

type callerKey struct{}

func RequireCaller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
		if userID == "" {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, userID)))
	})
}

func callerID(r *http.Request) string {
	value, _ := r.Context().Value(callerKey{}).(string)
	return value
}
