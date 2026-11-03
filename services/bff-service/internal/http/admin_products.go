package http

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"ecommerce.local/bff-service/internal/catalog"
	"ecommerce.local/bff-service/internal/inventory"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
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

type mediaPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type mediaPresignData struct {
	UploadID         string `json:"uploadId"`
	MediaType        string `json:"mediaType"`
	UploadURL        string `json:"uploadUrl"`
	PublicURL        string `json:"publicUrl"`
	ObjectKey        string `json:"objectKey"`
	ExpiresInSeconds int32  `json:"expiresInSeconds"`
}

type adminProductVariantInput struct {
	SkuID         string `json:"skuId,omitempty"`
	Label         string `json:"label"`
	ColorHex      string `json:"colorHex,omitempty"`
	Price         int64  `json:"price"`
	OriginalPrice int64  `json:"originalPrice,omitempty"`
}

type adminProductMutationRequest struct {
	ID               string                     `json:"id,omitempty"`
	Name             string                     `json:"name"`
	ShortDescription string                     `json:"shortDescription"`
	Description      string                     `json:"description"`
	Brand            string                     `json:"brand"`
	CategoryID       string                     `json:"categoryId"`
	CategoryName     string                     `json:"categoryName"`
	Images           []string                   `json:"images"`
	Videos           []string                   `json:"videos"`
	IsNew            bool                       `json:"isNew"`
	Badge            string                     `json:"badge"`
	Featured         bool                       `json:"featured"`
	Emoji            string                     `json:"emoji"`
	ImageHue         []uint32                   `json:"imageHue"`
	Variants         []adminProductVariantInput `json:"variants"`
	Details          []string                   `json:"details"`
}

type adminStockSnapshot struct {
	Quantity          int32  `json:"quantity"`
	Reserved          int32  `json:"reserved"`
	AvailableQuantity int32  `json:"availableQuantity"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

type adminCategoryMetric struct {
	CategoryID         string `json:"categoryId"`
	CategoryName       string `json:"categoryName"`
	ActiveProductCount int64  `json:"activeProductCount"`
}

type adminProductSummary struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Brand         string              `json:"brand"`
	CategoryID    string              `json:"categoryId"`
	CategoryName  string              `json:"categoryName"`
	Price         int64               `json:"price"`
	OriginalPrice int64               `json:"originalPrice"`
	MaxPrice      int64               `json:"maxPrice"`
	Images        []string            `json:"images"`
	Rating        float64             `json:"rating"`
	ReviewCount   int64               `json:"reviewCount"`
	SoldCount     int64               `json:"soldCount"`
	IsNew         bool                `json:"isNew"`
	Badge         string              `json:"badge"`
	Featured      bool                `json:"featured"`
	Emoji         string              `json:"emoji"`
	ImageHue      []uint32            `json:"imageHue"`
	Deleted       bool                `json:"deleted"`
	VariantCount  int32               `json:"variantCount"`
	VideoCount    int32               `json:"videoCount"`
	CreatedAt     string              `json:"createdAt"`
	UpdatedAt     string              `json:"updatedAt"`
	Stock         *adminStockSnapshot `json:"stock"`
}

type adminProductsPageData struct {
	Items         []adminProductSummary `json:"items"`
	Page          int32                 `json:"page"`
	PageSize      int32                 `json:"pageSize"`
	TotalItems    int64                 `json:"totalItems"`
	TotalPages    int32                 `json:"totalPages"`
	ActiveCount   int64                 `json:"activeCount"`
	DeletedCount  int64                 `json:"deletedCount"`
	Categories    []adminCategoryMetric `json:"categories"`
	StockDegraded bool                  `json:"stockDegraded"`
}

type adminProductVariant struct {
	SkuID         string              `json:"skuId"`
	Label         string              `json:"label"`
	ColorHex      string              `json:"colorHex,omitempty"`
	Price         int64               `json:"price"`
	OriginalPrice int64               `json:"originalPrice"`
	Stock         *adminStockSnapshot `json:"stock"`
}

type adminProductDetail struct {
	productSummary
	ShortDescription string                `json:"shortDescription"`
	Description      string                `json:"description"`
	Details          []string              `json:"details"`
	Videos           []string              `json:"videos"`
	Variants         []adminProductVariant `json:"variants"`
	Deleted          bool                  `json:"deleted"`
	Stock            *adminStockSnapshot   `json:"stock"`
}

func newAdminProductDetail(resp *catalogv1.GetProductResponse, stockBySku map[string]*inventoryv1.StockItem) adminProductDetail {
	variants := make([]adminProductVariant, 0, len(resp.GetVariants()))
	var productStock *adminStockSnapshot
	if stockBySku != nil {
		productStock = &adminStockSnapshot{}
	}

	for _, v := range resp.GetVariants() {
		var varStock *adminStockSnapshot
		if stockBySku != nil {
			if st, ok := stockBySku[v.GetSkuId()]; ok && st != nil {
				varStock = toAdminStockSnapshot(st)
				productStock.Quantity += st.GetQuantity()
				productStock.Reserved += st.GetReserved()
				productStock.AvailableQuantity += st.GetAvailableQuantity()
				if st.GetUpdatedAt() > productStock.UpdatedAt {
					productStock.UpdatedAt = st.GetUpdatedAt()
				}
			} else {
				varStock = &adminStockSnapshot{}
			}
		}
		variants = append(variants, adminProductVariant{
			SkuID:         v.GetSkuId(),
			Label:         v.GetLabel(),
			ColorHex:      v.GetColorHex(),
			Price:         amountMinor(v.GetPrice()),
			OriginalPrice: amountMinor(v.GetOriginalPrice()),
			Stock:         varStock,
		})
	}

	base := newProductDetail(resp)
	return adminProductDetail{
		productSummary:   base.productSummary,
		ShortDescription: base.ShortDescription,
		Description:      base.Description,
		Details:          base.Details,
		Videos:           stringsOrEmpty(resp.GetVideos()),
		Variants:         variants,
		Deleted:          base.Deleted,
		Stock:            productStock,
	}
}

type AdminProductsHandler struct {
	catalog   catalog.Client
	inventory inventory.Client
}

func NewAdminProductsHandler(client catalog.Client, inv ...inventory.Client) *AdminProductsHandler {
	var invClient inventory.Client
	if len(inv) > 0 {
		invClient = inv[0]
	}
	return &AdminProductsHandler{catalog: client, inventory: invClient}
}

func (h *AdminProductsHandler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/admin/products/media/presign", correlateTrace(RequireCaller(http.HandlerFunc(h.presignMedia))))
	mux.Handle("GET /api/v1/admin/products", correlateTrace(RequireCaller(http.HandlerFunc(h.list))))
	mux.Handle("GET /api/v1/admin/products/{id}", correlateTrace(RequireCaller(http.HandlerFunc(h.detail))))
	mux.Handle("POST /api/v1/admin/products", correlateTrace(RequireCaller(http.HandlerFunc(h.create))))
	mux.Handle("PUT /api/v1/admin/products/{id}", correlateTrace(RequireCaller(http.HandlerFunc(h.update))))
	mux.Handle("POST /api/v1/admin/products/{id}/delete", correlateTrace(RequireCaller(http.HandlerFunc(h.softDelete))))
}

func (h *AdminProductsHandler) presignMedia(w http.ResponseWriter, r *http.Request) {
	var input mediaPresignRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.", nil)
		return
	}
	if strings.TrimSpace(input.ContentType) == "" || input.SizeBytes <= 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "contentType and positive sizeBytes are required.", nil)
		return
	}

	resp, err := h.catalog.CreateProductMediaUpload(r.Context(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      callerID(r),
		Filename:    input.Filename,
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
	})
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to upload product media.")
		return
	}

	writeEnvelope(w, http.StatusOK, mediaPresignData{
		UploadID:         resp.GetUploadId(),
		MediaType:        resp.GetMediaType(),
		UploadURL:        resp.GetUploadUrl(),
		PublicURL:        resp.GetPublicUrl(),
		ObjectKey:        resp.GetObjectKey(),
		ExpiresInSeconds: resp.GetExpiresInSeconds(),
	}, "Upload URL generated.")
}

func (h *AdminProductsHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, errPage := parseBoundedInt(q.Get("page"), 1, 1, 1<<31-1)
	pageSize, errSize := parseBoundedInt(q.Get("pageSize"), 20, 1, 200)
	if errPage != nil || errSize != nil {
		fields := map[string]string{}
		if errPage != nil {
			fields["page"] = errPage.Error()
		}
		if errSize != nil {
			fields["pageSize"] = errSize.Error()
		}
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid pagination parameters.", fields)
		return
	}

	userID := callerID(r)
	resp, err := h.catalog.ListAdminProducts(r.Context(), &catalogv1.ListAdminProductsRequest{
		UserId:     userID,
		Query:      strings.TrimSpace(q.Get("q")),
		CategoryId: strings.TrimSpace(q.Get("categoryId")),
		Status:     strings.TrimSpace(q.Get("status")),
		Sort:       strings.TrimSpace(q.Get("sort")),
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to view products.")
		return
	}

	rawItems := resp.GetItems()
	var stockBySku map[string]*inventoryv1.StockItem
	stockDegraded := false
	if len(rawItems) > 0 && h.inventory != nil {
		allSkuIDs := collectAdminSummarySkuIDs(rawItems)
		if len(allSkuIDs) > 0 {
			stocks, err := h.inventory.BatchGetStock(r.Context(), userID, allSkuIDs)
			if err != nil {
				stockDegraded = true
			} else {
				stockBySku = make(map[string]*inventoryv1.StockItem, len(stocks))
				for _, st := range stocks {
					stockBySku[st.GetSkuId()] = st
				}
			}
		} else {
			stockBySku = map[string]*inventoryv1.StockItem{}
		}
	}

	items := make([]adminProductSummary, 0, len(rawItems))
	for _, p := range rawItems {
		var stock *adminStockSnapshot
		if stockBySku != nil {
			stock = rollupProductStock(p.GetSkuIds(), stockBySku)
		}
		items = append(items, adminProductSummary{
			ID:            p.GetId(),
			Name:          p.GetName(),
			Brand:         p.GetBrand(),
			CategoryID:    p.GetCategoryId(),
			CategoryName:  p.GetCategoryName(),
			Price:         amountMinor(p.GetPrice()),
			OriginalPrice: amountMinor(p.GetOriginalPrice()),
			MaxPrice:      amountMinor(p.GetMaxPrice()),
			Images:        stringsOrEmpty(p.GetImages()),
			Rating:        p.GetRating(),
			ReviewCount:   p.GetReviewCount(),
			SoldCount:     p.GetSoldCount(),
			IsNew:         p.GetIsNew(),
			Badge:         p.GetBadge(),
			Featured:      p.GetFeatured(),
			Emoji:         p.GetEmoji(),
			ImageHue:      uintsOrEmpty(p.GetImageHue()),
			Deleted:       p.GetDeleted(),
			VariantCount:  p.GetVariantCount(),
			VideoCount:    p.GetVideoCount(),
			CreatedAt:     p.GetCreatedAt(),
			UpdatedAt:     p.GetUpdatedAt(),
			Stock:         stock,
		})
	}

	categories := make([]adminCategoryMetric, 0, len(resp.GetCategories()))
	for _, cm := range resp.GetCategories() {
		categories = append(categories, adminCategoryMetric{
			CategoryID:         cm.GetCategoryId(),
			CategoryName:       cm.GetCategoryName(),
			ActiveProductCount: cm.GetActiveProductCount(),
		})
	}

	writeEnvelope(w, http.StatusOK, adminProductsPageData{
		Items:         items,
		Page:          resp.GetPage(),
		PageSize:      resp.GetPageSize(),
		TotalItems:    resp.GetTotalItems(),
		TotalPages:    resp.GetTotalPages(),
		ActiveCount:   resp.GetActiveCount(),
		DeletedCount:  resp.GetDeletedCount(),
		Categories:    categories,
		StockDegraded: stockDegraded,
	}, "Admin products retrieved.")
}

func (h *AdminProductsHandler) detail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	userID := callerID(r)

	resp, err := h.catalog.GetAdminProduct(r.Context(), userID, id)
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to view products.")
		return
	}

	var stockBySku map[string]*inventoryv1.StockItem
	if h.inventory != nil && len(resp.GetVariants()) > 0 {
		skuIDs := make([]string, 0, len(resp.GetVariants()))
		for _, v := range resp.GetVariants() {
			if skuID := strings.TrimSpace(v.GetSkuId()); skuID != "" {
				skuIDs = append(skuIDs, skuID)
			}
		}
		if len(skuIDs) > 0 {
			if stocks, err := h.inventory.BatchGetStock(r.Context(), userID, skuIDs); err == nil {
				stockBySku = make(map[string]*inventoryv1.StockItem, len(stocks))
				for _, st := range stocks {
					stockBySku[st.GetSkuId()] = st
				}
			}
		}
	}
	writeEnvelope(w, http.StatusOK, newAdminProductDetail(resp, stockBySku), "Product retrieved.")
}

func (h *AdminProductsHandler) create(w http.ResponseWriter, r *http.Request) {
	var input adminProductMutationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.", nil)
		return
	}
	if msg := validateAdminVariants(input.Variants); msg != "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", msg, nil)
		return
	}

	resp, err := h.catalog.CreateProduct(r.Context(), &catalogv1.CreateProductRequest{
		UserId:           callerID(r),
		Id:               input.ID,
		Name:             input.Name,
		ShortDescription: input.ShortDescription,
		Description:      input.Description,
		Brand:            input.Brand,
		CategoryId:       input.CategoryID,
		CategoryName:     input.CategoryName,
		Images:           input.Images,
		Videos:           input.Videos,
		IsNew:            input.IsNew,
		Badge:            input.Badge,
		Featured:         input.Featured,
		Emoji:            input.Emoji,
		ImageHue:         input.ImageHue,
		Variants:         toProtoVariants(input.Variants),
		Details:          input.Details,
	})
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to create products.")
		return
	}

	writeEnvelope(w, http.StatusCreated, newAdminProductDetail(resp, nil), "Product created.")
}

func (h *AdminProductsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	userID := callerID(r)

	var input adminProductMutationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.", nil)
		return
	}
	if msg := validateAdminVariants(input.Variants); msg != "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", msg, nil)
		return
	}

	existing, err := h.catalog.GetAdminProduct(r.Context(), userID, id)
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to update products.")
		return
	}

	removedSkuIDs := findRemovedSkuIDs(existing.GetVariants(), input.Variants)
	if len(removedSkuIDs) > 0 && h.inventory != nil {
		if err := h.inventory.DeleteStocks(r.Context(), userID, removedSkuIDs, true); err != nil {
			h.writeVariantRemovalStockError(w, r, err)
			return
		}
	}

	resp, err := h.catalog.UpdateProduct(r.Context(), &catalogv1.UpdateProductRequest{
		UserId:           userID,
		Id:               id,
		Name:             input.Name,
		ShortDescription: input.ShortDescription,
		Description:      input.Description,
		Brand:            input.Brand,
		CategoryId:       input.CategoryID,
		CategoryName:     input.CategoryName,
		Images:           input.Images,
		Videos:           input.Videos,
		IsNew:            input.IsNew,
		Badge:            input.Badge,
		Featured:         input.Featured,
		Emoji:            input.Emoji,
		ImageHue:         input.ImageHue,
		Variants:         toProtoVariants(input.Variants),
		Details:          input.Details,
	})
	if err != nil {
		h.writeAdminCatalogError(w, r, err, "You do not have permission to update products.")
		return
	}

	if len(removedSkuIDs) > 0 && h.inventory != nil {
		if err := h.inventory.DeleteStocks(r.Context(), userID, removedSkuIDs, false); err != nil {
			log.Printf("bff: post-update stock cleanup skipped for product %s (skus=%v, traceId=%s): %v", id, removedSkuIDs, requestTraceID(r), err)
		}
	}

	writeEnvelope(w, http.StatusOK, newAdminProductDetail(resp, nil), "Product updated.")
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
		h.writeAdminCatalogError(w, r, err, "You do not have permission to delete or restore products.")
		return
	}

	writeEnvelope(w, http.StatusOK, softDeleteData{ID: id, Deleted: deleted}, "Product availability updated.")
}

func (h *AdminProductsHandler) writeAdminCatalogError(w http.ResponseWriter, r *http.Request, err error, forbiddenMsg string) {
	switch {
	case errors.Is(err, catalog.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", forbiddenMsg, nil)
	case errors.Is(err, catalog.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	case errors.Is(err, catalog.ErrMediaStorageUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "MEDIA_STORAGE_UNAVAILABLE", "Media storage service is unavailable.", nil)
	case errors.Is(err, catalog.ErrProductNotFound):
		writeError(w, r, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
	case errors.Is(err, catalog.ErrProductExists):
		writeError(w, r, http.StatusConflict, "PRODUCT_ID_EXISTS", "A product with this ID already exists.", nil)
	case errors.Is(err, catalog.ErrSkuConflict):
		writeError(w, r, http.StatusConflict, "SKU_ALREADY_EXISTS", "A variant with this SKU ID already exists on another product.", nil)
	case errors.Is(err, catalog.ErrMediaAlreadyAttached):
		writeError(w, r, http.StatusConflict, "MEDIA_ALREADY_ATTACHED", "One or more media files are already attached to another product.", nil)
	case status.Code(err) == codes.InvalidArgument:
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", normalizeUpstreamInvalidArgument(status.Convert(err).Message()), nil)
	default:
		writeError(w, r, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Catalog service is unavailable.", nil)
	}
}

var upstreamFieldReplacer = strings.NewReplacer(
	"short_description", "shortDescription",
	"original_price", "originalPrice",
	"category_name", "categoryName",
	"category_id", "categoryId",
	"content_type", "contentType",
	"image_hue", "imageHue",
	"size_bytes", "sizeBytes",
	"color_hex", "colorHex",
	"product_id", "productId",
	"user_id", "userId",
	"sku_ids", "skuIds",
	"sku_id", "skuId",
	"is_new", "isNew",
)

func normalizeUpstreamInvalidArgument(msg string) string {
	return upstreamFieldReplacer.Replace(msg)
}

func (h *AdminProductsHandler) writeVariantRemovalStockError(w http.ResponseWriter, r *http.Request, err error) {
	var reservedErr *inventory.VariantHasReservedStockError
	switch {
	case errors.As(err, &reservedErr):
		var fields map[string]string
		if reservedErr.SkuID != "" {
			fields = map[string]string{"skuId": reservedErr.SkuID}
		}
		writeError(w, r, http.StatusConflict, "VARIANT_HAS_RESERVED_STOCK", "Cannot delete a variant while orders still hold reserved stock for it.", fields)
	case errors.Is(err, inventory.ErrVariantHasReservedStock):
		writeError(w, r, http.StatusConflict, "VARIANT_HAS_RESERVED_STOCK", "Cannot delete a variant while orders still hold reserved stock for it.", nil)
	case errors.Is(err, inventory.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You do not have permission to update products.", nil)
	case errors.Is(err, inventory.ErrAuthorizationUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_SERVICE_UNAVAILABLE", "Authorization service is unavailable.", nil)
	default:
		writeError(w, r, http.StatusBadGateway, "INVENTORY_UNAVAILABLE", "Inventory service is unavailable.", nil)
	}
}

func validateAdminVariants(variants []adminProductVariantInput) string {
	if len(variants) == 0 {
		return "At least one variant is required."
	}
	for _, v := range variants {
		if strings.TrimSpace(v.Label) == "" {
			return "Every variant must have a non-empty label."
		}
		if v.Price <= 0 {
			return "Every variant must have a positive price."
		}
		if v.OriginalPrice != 0 && v.OriginalPrice < v.Price {
			return "originalPrice must be zero or at least price."
		}
	}
	return ""
}

func findRemovedSkuIDs(existing []*catalogv1.ProductVariant, next []adminProductVariantInput) []string {
	retained := make(map[string]struct{}, len(next))
	for _, v := range next {
		if skuID := strings.TrimSpace(v.SkuID); skuID != "" {
			retained[skuID] = struct{}{}
		}
	}
	var removed []string
	seen := make(map[string]struct{})
	for _, old := range existing {
		skuID := strings.TrimSpace(old.GetSkuId())
		if skuID == "" {
			continue
		}
		if _, keep := retained[skuID]; keep {
			continue
		}
		if _, dup := seen[skuID]; !dup {
			seen[skuID] = struct{}{}
			removed = append(removed, skuID)
		}
	}
	return removed
}

func collectAdminSummarySkuIDs(items []*catalogv1.AdminProductSummary) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, item := range items {
		for _, raw := range item.GetSkuIds() {
			skuID := strings.TrimSpace(raw)
			if skuID == "" {
				continue
			}
			if _, ok := seen[skuID]; !ok {
				seen[skuID] = struct{}{}
				out = append(out, skuID)
			}
		}
	}
	return out
}

func rollupProductStock(skuIDs []string, stockBySku map[string]*inventoryv1.StockItem) *adminStockSnapshot {
	snap := &adminStockSnapshot{}
	for _, skuID := range skuIDs {
		if st, ok := stockBySku[skuID]; ok && st != nil {
			snap.Quantity += st.GetQuantity()
			snap.Reserved += st.GetReserved()
			snap.AvailableQuantity += st.GetAvailableQuantity()
			if st.GetUpdatedAt() > snap.UpdatedAt {
				snap.UpdatedAt = st.GetUpdatedAt()
			}
		}
	}
	return snap
}

func toProtoVariants(variants []adminProductVariantInput) []*catalogv1.ProductVariant {
	out := make([]*catalogv1.ProductVariant, 0, len(variants))
	for _, v := range variants {
		out = append(out, &catalogv1.ProductVariant{
			SkuId:         strings.TrimSpace(v.SkuID),
			Label:         v.Label,
			ColorHex:      v.ColorHex,
			Price:         &commonv1.Money{Currency: "VND", AmountMinor: v.Price},
			OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: v.OriginalPrice},
		})
	}
	return out
}

func toAdminStockSnapshot(st *inventoryv1.StockItem) *adminStockSnapshot {
	if st == nil {
		return nil
	}
	return &adminStockSnapshot{
		Quantity:          st.GetQuantity(),
		Reserved:          st.GetReserved(),
		AvailableQuantity: st.GetAvailableQuantity(),
		UpdatedAt:         st.GetUpdatedAt(),
	}
}
