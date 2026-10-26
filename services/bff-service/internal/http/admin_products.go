package http

import (
	"errors"
	"net/http"

	"ecommerce.local/bff-service/internal/catalog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type softDeleteRequest struct {
	Deleted *bool `json:"deleted"`
}

type softDeleteData struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type AdminProductsHandler struct {
	catalog catalog.Client
}

func NewAdminProductsHandler(client catalog.Client) *AdminProductsHandler {
	return &AdminProductsHandler{catalog: client}
}

func (h *AdminProductsHandler) Register(mux *http.ServeMux) {
	mux.Handle("POST /products/{id}/delete", correlateTrace(RequireCaller(http.HandlerFunc(h.softDelete))))
}

func (h *AdminProductsHandler) softDelete(w http.ResponseWriter, r *http.Request) {
	var input softDeleteRequest
	if err := decodeJSON(r, &input); err != nil || input.Deleted == nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.", nil)
		return
	}

	deleted := *input.Deleted
	id := r.PathValue("id")
	if err := h.catalog.SoftDeleteProduct(r.Context(), callerID(r), id, deleted); err != nil {
		switch {
		case errors.Is(err, catalog.ErrPermissionDenied):
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to delete or restore products.", nil)
		case errors.Is(err, catalog.ErrAuthorizationUnavailable):
			writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
		case errors.Is(err, catalog.ErrProductNotFound):
			writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
		case status.Code(err) == codes.InvalidArgument:
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request.", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
		}
		return
	}

	writeEnvelope(w, http.StatusOK, softDeleteData{ID: id, Deleted: deleted}, "Product availability updated.")
}
