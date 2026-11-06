package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	cartclient "ecommerce.local/bff-service/internal/cart"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cartData struct {
	Items    []cartItem `json:"items"`
	Degraded bool       `json:"degraded"`
}

type cartItem struct {
	ItemID            string   `json:"itemId"`
	ProductID         string   `json:"productId"`
	SkuID             string   `json:"skuId"`
	VariantLabel      string   `json:"variantLabel"`
	Quantity          int32    `json:"quantity"`
	Name              string   `json:"name"`
	Brand             string   `json:"brand"`
	CategoryName      string   `json:"categoryName"`
	UnitPrice         int64    `json:"unitPrice"`
	OriginalPrice     int64    `json:"originalPrice"`
	ImageEmoji        string   `json:"imageEmoji"`
	ImageHue          []uint32 `json:"imageHue"`
	ImageURL          string   `json:"imageUrl"`
	VariantColorHex   string   `json:"variantColorHex,omitempty"`
	Status            string   `json:"status"`
	AvailableQuantity int32    `json:"availableQuantity"`
	ProductVersion    int64    `json:"productVersion"`
}

type cartMutationRequest struct {
	ProductID string          `json:"productId"`
	SkuID     string          `json:"skuId"`
	Quantity  requestQuantity `json:"quantity"`
}

type cartQuantityRequest struct {
	Quantity requestQuantity `json:"quantity"`
}

type requestQuantity int32

func (q *requestQuantity) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			*q = 0
			return nil
		}
		return strconv.ErrSyntax
	}
	var value int32
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*q = requestQuantity(value)
	return nil
}

type CartHandler struct {
	cart cartclient.Client
}

func NewCartHandler(client cartclient.Client) *CartHandler {
	return &CartHandler{cart: client}
}

func (h *CartHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/cart", correlateTrace(RequireCaller(http.HandlerFunc(h.get))))
	mux.Handle("POST /api/v1/cart/items", correlateTrace(RequireCaller(http.HandlerFunc(h.add))))
	mux.Handle("PATCH /api/v1/cart/items/{itemId}", correlateTrace(RequireCaller(http.HandlerFunc(h.updateQuantity))))
	mux.Handle("DELETE /api/v1/cart/items/{itemId}", correlateTrace(RequireCaller(http.HandlerFunc(h.remove))))
	mux.Handle("DELETE /api/v1/cart", correlateTrace(RequireCaller(http.HandlerFunc(h.clear))))
}

func (h *CartHandler) get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.cart.GetCart(r.Context(), callerID(r))
	if err != nil {
		h.writeCartError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newCartData(resp.GetCart()), "Cart retrieved.")
}

func (h *CartHandler) add(w http.ResponseWriter, r *http.Request) {
	var input cartMutationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid request body.", nil)
		return
	}
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.SkuID = strings.TrimSpace(input.SkuID)
	if input.Quantity <= 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_QUANTITY", "Quantity must be positive.", nil)
		return
	}
	if input.ProductID == "" || input.SkuID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Product and SKU are required.", nil)
		return
	}
	resp, err := h.cart.AddItem(r.Context(), callerID(r), input.ProductID, input.SkuID, int32(input.Quantity))
	if err != nil {
		h.writeCartError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newCartData(resp.GetCart()), "Product added to cart.")
}

func (h *CartHandler) updateQuantity(w http.ResponseWriter, r *http.Request) {
	var input cartQuantityRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid request body.", nil)
		return
	}
	if input.Quantity <= 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_QUANTITY", "Quantity must be positive.", nil)
		return
	}
	resp, err := h.cart.UpdateItemQuantity(r.Context(), callerID(r), r.PathValue("itemId"), int32(input.Quantity))
	if err != nil {
		h.writeCartError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newCartData(resp.GetCart()), "Cart quantity updated.")
}

func (h *CartHandler) remove(w http.ResponseWriter, r *http.Request) {
	resp, err := h.cart.RemoveItem(r.Context(), callerID(r), r.PathValue("itemId"))
	if err != nil {
		h.writeCartError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newCartData(resp.GetCart()), "Product removed from cart.")
}

func (h *CartHandler) clear(w http.ResponseWriter, r *http.Request) {
	resp, err := h.cart.ClearCart(r.Context(), callerID(r))
	if err != nil {
		h.writeCartError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newCartData(resp.GetCart()), "Cart cleared.")
}

func (h *CartHandler) writeCartError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, cartclient.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access or modify the cart.", nil)
	case errors.Is(err, cartclient.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	case errors.Is(err, cartclient.ErrCartItemNotFound):
		writeError(w, r, http.StatusNotFound, "CART_ITEM_NOT_FOUND", "Cart item not found.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid cart request.", nil)
	default:
		writeError(w, r, http.StatusBadGateway, "CART_UNAVAILABLE", "Cart service is unavailable.", nil)
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

func newCartData(cart *cartv1.Cart) cartData {
	if cart == nil {
		return cartData{Items: []cartItem{}}
	}
	items := make([]cartItem, 0, len(cart.GetItems()))
	for _, item := range cart.GetItems() {
		items = append(items, newCartItem(item))
	}
	return cartData{Items: items, Degraded: cart.GetDegraded()}
}

func newCartItem(item *cartv1.CartItem) cartItem {
	return cartItem{
		ItemID: item.GetItemId(), ProductID: item.GetProductId(), SkuID: item.GetSkuId(), VariantLabel: item.GetVariantLabel(), Quantity: item.GetQuantity(),
		Name: item.GetName(), Brand: item.GetBrand(), CategoryName: item.GetCategoryName(),
		UnitPrice: amountMinor(item.GetUnitPrice()), OriginalPrice: amountMinor(item.GetOriginalPrice()),
		ImageEmoji: item.GetImageEmoji(), ImageHue: uintsOrEmpty(item.GetImageHue()), ImageURL: item.GetImageUrl(), VariantColorHex: item.GetVariantColorHex(),
		Status: publicCartStatus(item.GetStatus()), AvailableQuantity: item.GetAvailableQuantity(), ProductVersion: item.GetProductVersion(),
	}
}

func publicCartStatus(value cartv1.CartItemStatus) string {
	switch value {
	case cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE:
		return "AVAILABLE"
	case cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED:
		return "LIMITED"
	case cartv1.CartItemStatus_CART_ITEM_STATUS_OUT_OF_STOCK:
		return "OUT_OF_STOCK"
	case cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE:
		return "UNAVAILABLE"
	default:
		return "UNAVAILABLE"
	}
}
