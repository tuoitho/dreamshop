package http

import (
	"errors"
	"net/http"

	"ecommerce.local/bff-service/internal/catalog"
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
	mux.HandleFunc("POST /products/{id}/delete", h.softDelete)
}

func (h *AdminProductsHandler) softDelete(w http.ResponseWriter, r *http.Request) {
	var input softDeleteRequest
	if err := decodeJSON(r, &input); err != nil || input.Deleted == nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.", nil)
		return
	}

	deleted := *input.Deleted
	if err := h.catalog.SoftDeleteProduct(r.Context(), r.PathValue("id"), deleted); err != nil {
		switch {
		case errors.Is(err, catalog.ErrProductNotFound):
			writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
		}
		return
	}

	writeEnvelope(w, http.StatusOK, softDeleteData{ID: r.PathValue("id"), Deleted: deleted}, "Product availability updated.")
}
