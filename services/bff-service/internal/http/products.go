package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
)

const (
	lowStockThreshold = 5

	productStatusAvailable   = "AVAILABLE"
	productStatusLimited     = "LIMITED"
	productStatusOutOfStock  = "OUT_OF_STOCK"
	productStatusUnavailable = "UNAVAILABLE"
	productStatusUnknown     = "UNKNOWN"
)

// productSummary is the REST shape for a search result / product card.
type productSummary struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Brand             string   `json:"brand"`
	CategoryID        string   `json:"categoryId"`
	CategoryName      string   `json:"categoryName"`
	Price             int64    `json:"price"`
	OriginalPrice     int64    `json:"originalPrice"`
	MaxPrice          int64    `json:"maxPrice"`
	Rating            float64  `json:"rating"`
	ReviewCount       int64    `json:"reviewCount"`
	SoldCount         int64    `json:"soldCount"`
	Images            []string `json:"images"`
	Emoji             string   `json:"emoji"`
	ImageHue          []uint32 `json:"imageHue"`
	IsNew             bool     `json:"isNew"`
	Badge             string   `json:"badge"`
	Featured          bool     `json:"featured"`
	Status            string   `json:"status"`
	AvailableQuantity *int32   `json:"availableQuantity,omitempty"`
	InventoryDegraded bool     `json:"inventoryDegraded,omitempty"`
}

// productDetail is the REST shape for the product detail page: the summary
// fields plus detail-specific fields and per-SKU variant pricing/availability.
type productDetail struct {
	productSummary
	ShortDescription string           `json:"shortDescription"`
	Description      string           `json:"description"`
	Details          []string         `json:"details"`
	Videos           []string         `json:"videos,omitempty"`
	Variants         []productVariant `json:"variants"`
	Deleted          bool             `json:"deleted"`
}

type productVariant struct {
	SkuID             string `json:"skuId"`
	Label             string `json:"label"`
	ColorHex          string `json:"colorHex,omitempty"`
	Price             int64  `json:"price"`
	OriginalPrice     int64  `json:"originalPrice"`
	Status            string `json:"status"`
	AvailableQuantity int32  `json:"availableQuantity"`
}

// paginatedData is the REST pagination envelope.
type paginatedData struct {
	Items      []productSummary `json:"items"`
	Page       int32            `json:"page"`
	PageSize   int32            `json:"pageSize"`
	TotalItems int64            `json:"totalItems"`
	TotalPages int32            `json:"totalPages"`
}

// ProductsHandler serves the storefront product search and detail routes.
type ProductsHandler struct {
	catalog   catalog.Client
	inventory inventory.Client
}

func NewProductsHandler(catalogClient catalog.Client, inventoryClient inventory.Client) *ProductsHandler {
	return &ProductsHandler{
		catalog:   catalogClient,
		inventory: inventoryClient,
	}
}

func (h *ProductsHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/storefront/products", correlateTrace(http.HandlerFunc(h.search)))
	mux.Handle("GET /api/v1/storefront/products/{id}", correlateTrace(http.HandlerFunc(h.detail)))
}

// search handles GET /api/v1/storefront/products.
func (h *ProductsHandler) search(w http.ResponseWriter, r *http.Request) {
	p, err := parseSearchParams(r.URL.Query())
	if err != nil {
		var pe *paramError
		if errors.As(err, &pe) {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", pe.message, pe.fields)
		} else {
			writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid query parameters.", nil)
		}
		return
	}

	resp, err := h.catalog.SearchProducts(r.Context(), &catalogv1.SearchProductsRequest{
		Query:      p.query,
		CategoryId: p.categoryID,
		Sort:       p.sort,
		PriceMin:   p.priceMin,
		PriceMax:   p.priceMax,
		OnSale:     p.onSale,
		RatingMin:  p.ratingMin,
		Page:       p.page,
		PageSize:   p.pageSize,
	})
	if err != nil {
		h.writeCatalogError(w, r, err)
		return
	}

	rawItems := resp.GetItems()
	availBySku, degraded := h.fetchBatchSkuAvailability(r.Context(), collectSummarySkuIDs(rawItems))

	items := make([]productSummary, 0, len(rawItems))
	for _, item := range rawItems {
		summary := newProductSummary(item)
		skuIDs := item.GetSkuIds()
		switch {
		case len(skuIDs) == 0:
			summary.Status = productStatusUnavailable
			summary.AvailableQuantity = int32Ptr(0)
		case degraded:
			avail := degradedAvailability()
			summary.Status = avail.status
			summary.AvailableQuantity = avail.quantity
			summary.InventoryDegraded = avail.degraded
		default:
			var total int32
			for _, skuID := range skuIDs {
				total += availBySku[skuID]
			}
			avail := classifyStock(total)
			summary.Status = avail.status
			summary.AvailableQuantity = avail.quantity
		}
		items = append(items, summary)
	}
	writeEnvelope(w, http.StatusOK, paginatedData{
		Items:      items,
		Page:       resp.GetPage(),
		PageSize:   resp.GetPageSize(),
		TotalItems: resp.GetTotalItems(),
		TotalPages: resp.GetTotalPages(),
	}, "Products retrieved.")
}

// detail handles GET /api/v1/storefront/products/{id}.
func (h *ProductsHandler) detail(w http.ResponseWriter, r *http.Request) {
	resp, err := h.catalog.GetProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeCatalogError(w, r, err)
		return
	}
	detail := newProductDetail(resp)
	if resp.GetDeleted() || len(resp.GetVariants()) == 0 {
		detail.Status = productStatusUnavailable
		detail.AvailableQuantity = int32Ptr(0)
		for i := range detail.Variants {
			detail.Variants[i].Status = productStatusUnavailable
			detail.Variants[i].AvailableQuantity = 0
		}
	} else {
		skuIDs := make([]string, 0, len(resp.GetVariants()))
		seen := make(map[string]struct{}, len(resp.GetVariants()))
		for _, v := range resp.GetVariants() {
			skuID := strings.TrimSpace(v.GetSkuId())
			if skuID == "" {
				continue
			}
			if _, ok := seen[skuID]; !ok {
				seen[skuID] = struct{}{}
				skuIDs = append(skuIDs, skuID)
			}
		}

		availBySku, degraded := h.fetchBatchSkuAvailability(r.Context(), skuIDs)
		if degraded {
			avail := degradedAvailability()
			detail.Status = avail.status
			detail.AvailableQuantity = avail.quantity
			detail.InventoryDegraded = avail.degraded
			for i := range detail.Variants {
				detail.Variants[i].Status = productStatusUnknown
				detail.Variants[i].AvailableQuantity = 0
			}
		} else {
			var total int32
			for i := range detail.Variants {
				vQty := availBySku[detail.Variants[i].SkuID]
				vAvail := classifyStock(vQty)
				detail.Variants[i].Status = vAvail.status
				detail.Variants[i].AvailableQuantity = vQty
				total += vQty
			}
			prodAvail := classifyStock(total)
			detail.Status = prodAvail.status
			detail.AvailableQuantity = prodAvail.quantity
		}
	}
	writeEnvelope(w, http.StatusOK, detail, "Product retrieved.")
}

type availabilityInfo struct {
	status   string
	quantity *int32
	degraded bool
}

func degradedAvailability() availabilityInfo {
	return availabilityInfo{
		status:   productStatusUnknown,
		degraded: true,
	}
}

func classifyStock(available int32) availabilityInfo {
	if available <= 0 {
		return availabilityInfo{
			status:   productStatusOutOfStock,
			quantity: int32Ptr(0),
		}
	}
	if available <= lowStockThreshold {
		return availabilityInfo{
			status:   productStatusLimited,
			quantity: int32Ptr(available),
		}
	}
	return availabilityInfo{
		status:   productStatusAvailable,
		quantity: int32Ptr(available),
	}
}

func collectSummarySkuIDs(items []*catalogv1.ProductSummary) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, item := range items {
		for _, rawSku := range item.GetSkuIds() {
			skuID := strings.TrimSpace(rawSku)
			if skuID == "" {
				continue
			}
			if _, exists := seen[skuID]; !exists {
				seen[skuID] = struct{}{}
				out = append(out, skuID)
			}
		}
	}
	return out
}

func (h *ProductsHandler) fetchBatchSkuAvailability(ctx context.Context, skuIDs []string) (map[string]int32, bool) {
	if len(skuIDs) == 0 {
		return map[string]int32{}, false
	}
	if h.inventory == nil {
		return nil, true
	}
	items, err := h.inventory.BatchGetAvailability(ctx, skuIDs)
	if err != nil {
		traceID, _ := ctx.Value(traceIDKey{}).(string)
		log.Printf("bff: inventory batch availability lookup failed (skus=%d, traceId=%s): %v", len(skuIDs), traceID, err)
		return nil, true
	}
	availBySku := make(map[string]int32, len(items))
	for _, item := range items {
		qty := item.GetAvailableQuantity()
		if qty < 0 {
			qty = 0
		}
		availBySku[item.GetSkuId()] = qty
	}
	return availBySku, false
}

func int32Ptr(v int32) *int32 {
	return &v
}

// writeCatalogError maps catalog outcomes to the public REST error contract.
func (h *ProductsHandler) writeCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, catalog.ErrProductNotFound):
		writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid request parameters.", nil)
	default:
		writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
	}
}

// searchParams is the validated subset of the /products query string.
type searchParams struct {
	query      string
	categoryID string
	priceMin   int64
	priceMax   int64
	onSale     bool
	ratingMin  int32
	sort       string
	page       int32
	pageSize   int32
}

// paramError carries field-level validation details for a 400 response.
type paramError struct {
	message string
	fields  map[string]string
}

func (e *paramError) Error() string {
	return e.message
}

func parseSearchParams(q url.Values) (*searchParams, error) {
	p := &searchParams{}
	fields := make(map[string]string)

	p.query = strings.TrimSpace(q.Get("q"))
	p.categoryID = q.Get("danh-muc")

	if v := q.Get("gia"); v != "" {
		min, max, ok := parsePriceRange(v)
		if !ok {
			fields["gia"] = "must be a \"min-max\" range of non-negative integers; a max of 0 means no upper bound"
		} else {
			p.priceMin = min
			p.priceMax = max
		}
	}

	if v := q.Get("giam-gia"); v != "" {
		switch v {
		case "1":
			p.onSale = true
		case "0":
			p.onSale = false
		default:
			fields["giam-gia"] = "must be 1 to filter on-sale products"
		}
	}

	if v := q.Get("danh-gia"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 5 {
			fields["danh-gia"] = "must be an integer between 1 and 5"
		} else {
			p.ratingMin = int32(n)
		}
	}

	if v := q.Get("sap-xep"); v != "" {
		switch v {
		case "pho-bien", "gia-thap", "gia-cao", "moi-nhat", "danh-gia":
			p.sort = v
		default:
			fields["sap-xep"] = "must be one of: pho-bien, gia-thap, gia-cao, moi-nhat, danh-gia"
		}
	}

	page, err := parseBoundedInt(q.Get("page"), 1, 1, 1<<31-1)
	if err != nil {
		fields["page"] = err.Error()
	} else {
		p.page = int32(page)
	}

	pageSize, err := parseBoundedInt(q.Get("pageSize"), 20, 1, 50)
	if err != nil {
		fields["pageSize"] = err.Error()
	} else {
		p.pageSize = int32(pageSize)
	}

	if len(fields) > 0 {
		return nil, &paramError{message: "Invalid query parameters.", fields: fields}
	}
	return p, nil
}

// parseBoundedInt parses v as an integer within [min, max]; an empty v yields
// def. A present out-of-range or non-numeric value is an error.
func parseBoundedInt(v string, def, min, max int) (int, error) {
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	if n < min || n > max {
		return 0, fmt.Errorf("must be between %d and %d", min, max)
	}
	return n, nil
}

// parsePriceRange parses "min-max". max of 0 means unbounded and is passed
// through as an unset upper bound in the gRPC request.
func parsePriceRange(v string) (min, max int64, ok bool) {
	parts := strings.Split(v, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	min, errMin := strconv.ParseInt(parts[0], 10, 64)
	max, errMax := strconv.ParseInt(parts[1], 10, 64)
	if errMin != nil || errMax != nil || min < 0 || max < 0 {
		return 0, 0, false
	}
	return min, max, true
}

func newProductSummary(p *catalogv1.ProductSummary) productSummary {
	return productSummary{
		ID:            p.GetId(),
		Name:          p.GetName(),
		Brand:         p.GetBrand(),
		CategoryID:    p.GetCategoryId(),
		CategoryName:  p.GetCategoryName(),
		Price:         amountMinor(p.GetPrice()),
		OriginalPrice: amountMinor(p.GetOriginalPrice()),
		MaxPrice:      amountMinor(p.GetMaxPrice()),
		Rating:        p.GetRating(),
		ReviewCount:   p.GetReviewCount(),
		SoldCount:     p.GetSoldCount(),
		Images:        stringsOrEmpty(p.GetImages()),
		Emoji:         p.GetEmoji(),
		ImageHue:      uintsOrEmpty(p.GetImageHue()),
		IsNew:         p.GetIsNew(),
		Badge:         p.GetBadge(),
		Featured:      p.GetFeatured(),
	}
}

func newProductDetail(p *catalogv1.GetProductResponse) productDetail {
	variants := make([]productVariant, 0, len(p.GetVariants()))
	for _, v := range p.GetVariants() {
		variants = append(variants, productVariant{
			SkuID:         v.GetSkuId(),
			Label:         v.GetLabel(),
			ColorHex:      v.GetColorHex(),
			Price:         amountMinor(v.GetPrice()),
			OriginalPrice: amountMinor(v.GetOriginalPrice()),
		})
	}
	return productDetail{
		productSummary: productSummary{
			ID:            p.GetId(),
			Name:          p.GetName(),
			Brand:         p.GetBrand(),
			CategoryID:    p.GetCategoryId(),
			CategoryName:  p.GetCategoryName(),
			Price:         amountMinor(p.GetPrice()),
			OriginalPrice: amountMinor(p.GetOriginalPrice()),
			MaxPrice:      amountMinor(p.GetMaxPrice()),
			Rating:        p.GetRating(),
			ReviewCount:   p.GetReviewCount(),
			SoldCount:     p.GetSoldCount(),
			Images:        stringsOrEmpty(p.GetImages()),
			Emoji:         p.GetEmoji(),
			ImageHue:      uintsOrEmpty(p.GetImageHue()),
			IsNew:         p.GetIsNew(),
			Badge:         p.GetBadge(),
			Featured:      p.GetFeatured(),
		},
		ShortDescription: p.GetShortDescription(),
		Description:      p.GetDescription(),
		Details:          stringsOrEmpty(p.GetDetails()),
		Videos:           p.GetVideos(),
		Variants:         variants,
		Deleted:          p.GetDeleted(),
	}
}

// amountMinor converts the protobuf Money amount to the public integer JSON
// value (VND has no minor subunits).
func amountMinor(m *commonv1.Money) int64 {
	if m == nil {
		return 0
	}
	return m.GetAmountMinor()
}

func stringsOrEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func uintsOrEmpty(in []uint32) []uint32 {
	if in == nil {
		return []uint32{}
	}
	return in
}
