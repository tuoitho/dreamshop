package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	inventoryStatusInStock    = "IN_STOCK"
	inventoryStatusLowStock   = "LOW_STOCK"
	inventoryStatusOutOfStock = "OUT_OF_STOCK"
)

type setStockRequest struct {
	ProductID string `json:"productId"`
	Quantity  *int32 `json:"quantity"`
}

type adjustStockRequest struct {
	ProductID string `json:"productId"`
	Delta     *int32 `json:"delta"`
}

type adminInventoryItem struct {
	SkuID             string   `json:"skuId"`
	ProductID         string   `json:"productId"`
	Name              string   `json:"name"`
	VariantLabel      string   `json:"variantLabel"`
	VariantColorHex   string   `json:"variantColorHex,omitempty"`
	Brand             string   `json:"brand"`
	CategoryID        string   `json:"categoryId"`
	CategoryName      string   `json:"categoryName"`
	Price             int64    `json:"price"`
	OriginalPrice     int64    `json:"originalPrice"`
	ImageURL          string   `json:"imageUrl"`
	Emoji             string   `json:"emoji"`
	ImageHue          []uint32 `json:"imageHue"`
	Deleted           bool     `json:"deleted"`
	Quantity          int32    `json:"quantity"`
	Reserved          int32    `json:"reserved"`
	AvailableQuantity int32    `json:"availableQuantity"`
	StockStatus       string   `json:"stockStatus"`
	UpdatedAt         string   `json:"updatedAt,omitempty"`
}

type adminInventorySummary struct {
	TotalTrackedProducts int64 `json:"totalTrackedProducts"`
	TotalTrackedSkus     int64 `json:"totalTrackedSkus"`
	InStockCount         int64 `json:"inStockCount"`
	LowStockCount        int64 `json:"lowStockCount"`
	OutOfStockCount      int64 `json:"outOfStockCount"`
	TotalQuantity        int64 `json:"totalQuantity"`
	TotalReserved        int64 `json:"totalReserved"`
	TotalAvailable       int64 `json:"totalAvailable"`
}

type adminInventoryPageData struct {
	Items      []adminInventoryItem  `json:"items"`
	Page       int32                 `json:"page"`
	PageSize   int32                 `json:"pageSize"`
	TotalItems int64                 `json:"totalItems"`
	TotalPages int32                 `json:"totalPages"`
	Summary    adminInventorySummary `json:"summary"`
}

type stockMutationData struct {
	SkuID             string `json:"skuId"`
	ProductID         string `json:"productId"`
	Quantity          int32  `json:"quantity"`
	Reserved          int32  `json:"reserved"`
	AvailableQuantity int32  `json:"availableQuantity"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

type AdminInventoryHandler struct {
	catalog   catalog.Client
	inventory inventory.Client
}

func NewAdminInventoryHandler(catalogClient catalog.Client, inventoryClient inventory.Client) *AdminInventoryHandler {
	return &AdminInventoryHandler{
		catalog:   catalogClient,
		inventory: inventoryClient,
	}
}

func (h *AdminInventoryHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/inventory", correlateTrace(RequireCaller(http.HandlerFunc(h.list))))
	mux.Handle("PUT /api/v1/admin/inventory/skus/{skuId}", correlateTrace(RequireCaller(http.HandlerFunc(h.setStock))))
	mux.Handle("POST /api/v1/admin/inventory/skus/{skuId}/adjust", correlateTrace(RequireCaller(http.HandlerFunc(h.adjustStock))))
}

func (h *AdminInventoryHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, errPage := parseBoundedInt(q.Get("page"), 1, 1, 1<<31-1)
	pageSize, errSize := parseBoundedInt(q.Get("pageSize"), 20, 1, 100)
	if errPage != nil || errSize != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid pagination parameters.", nil)
		return
	}

	includeDeleted := true
	if rawInc := strings.TrimSpace(q.Get("includeDeleted")); rawInc != "" {
		parsed, err := strconv.ParseBool(rawInc)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "includeDeleted must be a boolean.", nil)
			return
		}
		includeDeleted = parsed
	}

	filterStatus := strings.ToLower(strings.TrimSpace(q.Get("stockStatus")))
	switch filterStatus {
	case "", "all", "in_stock", "low_stock", "out_of_stock":
	default:
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "stockStatus must be one of: all, in_stock, low_stock, out_of_stock.", nil)
		return
	}

	userID := callerID(r)
	stocks, err := h.inventory.ListStocks(r.Context(), userID)
	if err != nil {
		h.writeInventoryError(w, r, err, "You do not have permission to view inventory.")
		return
	}

	queryText := strings.TrimSpace(q.Get("q"))
	catResp, err := h.catalog.ListInventoryCatalogItems(r.Context(), &catalogv1.ListInventoryCatalogItemsRequest{
		UserId:         userID,
		Query:          queryText,
		CategoryId:     strings.TrimSpace(q.Get("categoryId")),
		IncludeDeleted: includeDeleted,
	})
	if err != nil {
		switch {
		case errors.Is(err, catalog.ErrPermissionDenied):
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to view inventory.", nil)
		case errors.Is(err, catalog.ErrAuthorizationUnavailable):
			writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
		}
		return
	}

	stockBySku := make(map[string]*inventoryv1.StockItem, len(stocks))
	for _, st := range stocks {
		stockBySku[st.GetSkuId()] = st
	}

	var summary adminInventorySummary
	trackedProducts := make(map[string]struct{}, len(catResp.GetItems()))
	filtered := make([]adminInventoryItem, 0, len(catResp.GetItems()))

	for _, item := range catResp.GetItems() {
		productMatched := queryText == "" ||
			containsFold(item.GetId(), queryText) ||
			containsFold(item.GetName(), queryText) ||
			containsFold(item.GetBrand(), queryText) ||
			containsFold(item.GetCategoryName(), queryText)

		for _, variant := range item.GetVariants() {
			skuID := strings.TrimSpace(variant.GetSkuId())
			if skuID == "" {
				continue
			}
			if !productMatched && !containsFold(skuID, queryText) && !containsFold(variant.GetLabel(), queryText) {
				continue
			}

			trackedProducts[item.GetId()] = struct{}{}

			var qty, reserved, avail int32
			var updatedAt string
			if st, ok := stockBySku[skuID]; ok {
				qty = st.GetQuantity()
				reserved = st.GetReserved()
				avail = st.GetAvailableQuantity()
				updatedAt = st.GetUpdatedAt()
			}

			statusLabel := classifyInventoryBucket(avail)
			summary.TotalTrackedSkus++
			summary.TotalQuantity += int64(qty)
			summary.TotalReserved += int64(reserved)
			summary.TotalAvailable += int64(avail)
			switch statusLabel {
			case inventoryStatusInStock:
				summary.InStockCount++
			case inventoryStatusLowStock:
				summary.LowStockCount++
			default:
				summary.OutOfStockCount++
			}

			if filterStatus != "" && filterStatus != "all" {
				if (filterStatus == "in_stock" && statusLabel != inventoryStatusInStock) ||
					(filterStatus == "low_stock" && statusLabel != inventoryStatusLowStock) ||
					(filterStatus == "out_of_stock" && statusLabel != inventoryStatusOutOfStock) {
					continue
				}
			}

			filtered = append(filtered, adminInventoryItem{
				SkuID:             skuID,
				ProductID:         item.GetId(),
				Name:              item.GetName(),
				VariantLabel:      variant.GetLabel(),
				VariantColorHex:   variant.GetColorHex(),
				Brand:             item.GetBrand(),
				CategoryID:        item.GetCategoryId(),
				CategoryName:      item.GetCategoryName(),
				Price:             amountMinor(variant.GetPrice()),
				OriginalPrice:     amountMinor(variant.GetOriginalPrice()),
				ImageURL:          item.GetImageUrl(),
				Emoji:             item.GetEmoji(),
				ImageHue:          uintsOrEmpty(item.GetImageHue()),
				Deleted:           item.GetDeleted(),
				Quantity:          qty,
				Reserved:          reserved,
				AvailableQuantity: avail,
				StockStatus:       statusLabel,
				UpdatedAt:         updatedAt,
			})
		}
	}
	summary.TotalTrackedProducts = int64(len(trackedProducts))

	totalItems := int64(len(filtered))
	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(pageSize) - 1) / int64(pageSize))
	}
	offset := (page - 1) * pageSize
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	writeEnvelope(w, http.StatusOK, adminInventoryPageData{
		Items:      filtered[offset:end],
		Page:       int32(page),
		PageSize:   int32(pageSize),
		TotalItems: totalItems,
		TotalPages: totalPages,
		Summary:    summary,
	}, "Inventory retrieved.")
}

func (h *AdminInventoryHandler) setStock(w http.ResponseWriter, r *http.Request) {
	skuID := strings.TrimSpace(r.PathValue("skuId"))
	var input setStockRequest
	if err := decodeJSON(r, &input); err != nil || input.Quantity == nil || *input.Quantity < 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "productId and non-negative quantity are required.", nil)
		return
	}
	productID := strings.TrimSpace(input.ProductID)
	if skuID == "" || productID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "skuId and productId are required.", nil)
		return
	}

	if !h.verifyProductOwnsSku(w, r, productID, skuID) {
		return
	}

	st, err := h.inventory.SetStock(r.Context(), callerID(r), skuID, productID, *input.Quantity)
	if err != nil {
		h.writeInventoryError(w, r, err, "You do not have permission to update inventory.")
		return
	}

	writeEnvelope(w, http.StatusOK, toStockMutationData(st), "Stock quantity updated.")
}

func (h *AdminInventoryHandler) adjustStock(w http.ResponseWriter, r *http.Request) {
	skuID := strings.TrimSpace(r.PathValue("skuId"))
	var input adjustStockRequest
	if err := decodeJSON(r, &input); err != nil || input.Delta == nil || *input.Delta == 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "productId and non-zero delta are required.", nil)
		return
	}
	productID := strings.TrimSpace(input.ProductID)
	if skuID == "" || productID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "skuId and productId are required.", nil)
		return
	}

	if !h.verifyProductOwnsSku(w, r, productID, skuID) {
		return
	}

	st, err := h.inventory.AdjustStock(r.Context(), callerID(r), skuID, productID, *input.Delta)
	if err != nil {
		h.writeInventoryError(w, r, err, "You do not have permission to update inventory.")
		return
	}

	writeEnvelope(w, http.StatusOK, toStockMutationData(st), "Stock adjusted.")
}

func (h *AdminInventoryHandler) verifyProductOwnsSku(w http.ResponseWriter, r *http.Request, productID, skuID string) bool {
	product, err := h.catalog.GetAdminProduct(r.Context(), callerID(r), productID)
	if err != nil {
		switch {
		case errors.Is(err, catalog.ErrPermissionDenied):
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to update inventory.", nil)
		case errors.Is(err, catalog.ErrAuthorizationUnavailable):
			writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
		case errors.Is(err, catalog.ErrProductNotFound):
			writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
		}
		return false
	}
	for _, v := range product.GetVariants() {
		if v.GetSkuId() == skuID {
			return true
		}
	}
	writeError(w, r, http.StatusNotFound, "SKU_NOT_FOUND", "Variant SKU not found on product.", nil)
	return false
}

func (h *AdminInventoryHandler) writeInventoryError(w http.ResponseWriter, r *http.Request, err error, forbiddenMsg string) {
	switch {
	case errors.Is(err, inventory.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", forbiddenMsg, nil)
	case errors.Is(err, inventory.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	case errors.Is(err, inventory.ErrStockBelowReserved):
		writeError(w, r, http.StatusConflict, "STOCK_BELOW_RESERVED", "Physical stock quantity cannot be less than currently reserved units.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", normalizeUpstreamInvalidArgument(status.Convert(err).Message()), nil)
	default:
		writeError(w, r, http.StatusBadGateway, "INVENTORY_UNAVAILABLE", "Inventory service is unavailable.", nil)
	}
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func classifyInventoryBucket(avail int32) string {
	switch {
	case avail <= 0:
		return inventoryStatusOutOfStock
	case avail <= lowStockThreshold:
		return inventoryStatusLowStock
	default:
		return inventoryStatusInStock
	}
}

func toStockMutationData(st *inventoryv1.StockItem) stockMutationData {
	return stockMutationData{
		SkuID:             st.GetSkuId(),
		ProductID:         st.GetProductId(),
		Quantity:          st.GetQuantity(),
		Reserved:          st.GetReserved(),
		AvailableQuantity: st.GetAvailableQuantity(),
		UpdatedAt:         st.GetUpdatedAt(),
	}
}
