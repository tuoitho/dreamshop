package http

import (
	"errors"
	"net/http"
	"strings"

	cartclient "ecommerce.local/bff-service/internal/cart"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxFavoriteProductIDLength = 128

type favoriteData struct {
	Items    []favoriteItem `json:"items"`
	Degraded bool           `json:"degraded"`
}

type favoriteVariant struct {
	Label    string `json:"label"`
	ColorHex string `json:"colorHex,omitempty"`
}

type favoriteItem struct {
	ItemID            string            `json:"itemId"`
	ProductID         string            `json:"productId"`
	Name              string            `json:"name"`
	Brand             string            `json:"brand"`
	CategoryID        string            `json:"categoryId"`
	CategoryName      string            `json:"categoryName"`
	Price             int64             `json:"price"`
	OriginalPrice     int64             `json:"originalPrice"`
	Rating            float64           `json:"rating"`
	ReviewCount       int64             `json:"reviewCount"`
	SoldCount         int64             `json:"soldCount"`
	ImageURL          string            `json:"imageUrl"`
	ImageEmoji        string            `json:"imageEmoji"`
	ImageHue          []uint32          `json:"imageHue"`
	Badge             string            `json:"badge"`
	Variants          []favoriteVariant `json:"variants"`
	Status            string            `json:"status"`
	AvailableQuantity int32             `json:"availableQuantity"`
}

type favoriteMutationRequest struct {
	ProductID string `json:"productId"`
}

type FavoritesHandler struct {
	cart cartclient.Client
}

func NewFavoritesHandler(client cartclient.Client) *FavoritesHandler {
	return &FavoritesHandler{cart: client}
}

func (h *FavoritesHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/favorites", correlateTrace(RequireCaller(http.HandlerFunc(h.get))))
	mux.Handle("POST /api/v1/favorites/items", correlateTrace(RequireCaller(http.HandlerFunc(h.add))))
	mux.Handle("DELETE /api/v1/favorites/items/{productId}", correlateTrace(RequireCaller(http.HandlerFunc(h.remove))))
	mux.Handle("DELETE /api/v1/favorites", correlateTrace(RequireCaller(http.HandlerFunc(h.clear))))
}

func (h *FavoritesHandler) get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.cart.GetFavorites(r.Context(), callerID(r))
	if err != nil {
		h.writeFavoriteError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newFavoriteData(resp.GetFavorites()), "Favorites retrieved.")
}

func (h *FavoritesHandler) add(w http.ResponseWriter, r *http.Request) {
	var input favoriteMutationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid request body.", nil)
		return
	}
	productID, ok := normalizeFavoriteProductID(input.ProductID)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Product ID is required.", nil)
		return
	}
	resp, err := h.cart.AddFavorite(r.Context(), callerID(r), productID)
	if err != nil {
		h.writeFavoriteError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newFavoriteData(resp.GetFavorites()), "Product added to favorites.")
}

func (h *FavoritesHandler) remove(w http.ResponseWriter, r *http.Request) {
	productID, ok := normalizeFavoriteProductID(r.PathValue("productId"))
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Product ID is required.", nil)
		return
	}
	resp, err := h.cart.RemoveFavorite(r.Context(), callerID(r), productID)
	if err != nil {
		h.writeFavoriteError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newFavoriteData(resp.GetFavorites()), "Product removed from favorites.")
}

func (h *FavoritesHandler) clear(w http.ResponseWriter, r *http.Request) {
	resp, err := h.cart.ClearFavorites(r.Context(), callerID(r))
	if err != nil {
		h.writeFavoriteError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newFavoriteData(resp.GetFavorites()), "Favorites cleared.")
}

func normalizeFavoriteProductID(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxFavoriteProductIDLength {
		return "", false
	}
	return trimmed, true
}

func (h *FavoritesHandler) writeFavoriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, cartclient.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to manage favorites.", nil)
	case errors.Is(err, cartclient.ErrProductNotFound):
		writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
	case errors.Is(err, cartclient.ErrProductUnavailable):
		writeError(w, r, http.StatusConflict, "PRODUCT_UNAVAILABLE", "Product is no longer available.", nil)
	case errors.Is(err, cartclient.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid favorites request.", nil)
	default:
		writeError(w, r, http.StatusBadGateway, "FAVORITES_UNAVAILABLE", "Favorites service is unavailable.", nil)
	}
}

func newFavoriteData(list *cartv1.FavoriteList) favoriteData {
	if list == nil {
		return favoriteData{Items: []favoriteItem{}}
	}
	items := make([]favoriteItem, 0, len(list.GetItems()))
	for _, item := range list.GetItems() {
		items = append(items, newFavoriteItem(item))
	}
	return favoriteData{Items: items, Degraded: list.GetDegraded()}
}

func newFavoriteItem(item *cartv1.FavoriteItem) favoriteItem {
	variants := make([]favoriteVariant, 0, len(item.GetVariants()))
	for _, variant := range item.GetVariants() {
		variants = append(variants, favoriteVariant{
			Label:    variant.GetLabel(),
			ColorHex: variant.GetColorHex(),
		})
	}
	return favoriteItem{
		ItemID:            item.GetItemId(),
		ProductID:         item.GetProductId(),
		Name:              item.GetName(),
		Brand:             item.GetBrand(),
		CategoryID:        item.GetCategoryId(),
		CategoryName:      item.GetCategoryName(),
		Price:             amountMinor(item.GetPrice()),
		OriginalPrice:     amountMinor(item.GetOriginalPrice()),
		Rating:            item.GetRating(),
		ReviewCount:       item.GetReviewCount(),
		SoldCount:         item.GetSoldCount(),
		ImageURL:          item.GetImageUrl(),
		ImageEmoji:        item.GetImageEmoji(),
		ImageHue:          uintsOrEmpty(item.GetImageHue()),
		Badge:             item.GetBadge(),
		Variants:          variants,
		Status:            publicFavoriteStatus(item.GetStatus()),
		AvailableQuantity: item.GetAvailableQuantity(),
	}
}

func publicFavoriteStatus(value cartv1.FavoriteItemStatus) string {
	switch value {
	case cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_AVAILABLE:
		return "AVAILABLE"
	case cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_LIMITED:
		return "LIMITED"
	case cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_OUT_OF_STOCK:
		return "OUT_OF_STOCK"
	case cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNKNOWN:
		return "UNKNOWN"
	default:
		return "UNAVAILABLE"
	}
}
