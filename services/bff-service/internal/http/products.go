package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"

	"ecommerce.local/bff-service/internal/catalog"
)

// productSummary is the REST shape for a search result / product card.
type productSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Brand         string   `json:"brand"`
	CategoryID    string   `json:"categoryId"`
	CategoryName  string   `json:"categoryName"`
	Price         int64    `json:"price"`
	OriginalPrice int64    `json:"originalPrice"`
	Rating        float64  `json:"rating"`
	ReviewCount   int64    `json:"reviewCount"`
	SoldCount     int64    `json:"soldCount"`
	Images        []string `json:"images"`
	Emoji         string   `json:"emoji"`
	ImageHue      []uint32 `json:"imageHue"`
	IsNew         bool     `json:"isNew"`
	Badge         string   `json:"badge"`
	Featured      bool     `json:"featured"`
}

// productDetail is the REST shape for the product detail page: the summary
// fields plus detail-specific fields.
type productDetail struct {
	productSummary
	ShortDescription string           `json:"shortDescription"`
	Description      string           `json:"description"`
	Details          []string         `json:"details"`
	Variants         []productVariant `json:"variants"`
	Deleted          bool             `json:"deleted"`
}

type productVariant struct {
	Label    string `json:"label"`
	ColorHex string `json:"colorHex,omitempty"`
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
	catalog catalog.Client
}

func NewProductsHandler(client catalog.Client) *ProductsHandler {
	return &ProductsHandler{catalog: client}
}

func (h *ProductsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", h.search)
	mux.HandleFunc("GET /products/{id}", h.detail)
}

// search handles GET /products.
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

	items := make([]productSummary, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, newProductSummary(item))
	}
	writeEnvelope(w, http.StatusOK, paginatedData{
		Items:      items,
		Page:       resp.GetPage(),
		PageSize:   resp.GetPageSize(),
		TotalItems: resp.GetTotalItems(),
		TotalPages: resp.GetTotalPages(),
	}, "Products retrieved.")
}

// detail handles GET /products/{id}.
func (h *ProductsHandler) detail(w http.ResponseWriter, r *http.Request) {
	resp, err := h.catalog.GetProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeCatalogError(w, r, err)
		return
	}
	writeEnvelope(w, http.StatusOK, newProductDetail(resp), "Product retrieved.")
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
			Label:    v.GetLabel(),
			ColorHex: v.GetColorHex(),
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
