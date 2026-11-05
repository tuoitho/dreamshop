package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListAdminProducts returns paginated products including soft-deleted items and global KPIs.
func (s *Server) ListAdminProducts(ctx context.Context, req *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error) {
	if err := s.authorize(ctx, req.GetUserId(), productViewPermission); err != nil {
		return nil, err
	}

	res, err := s.repo.ListAdmin(ctx, domain.AdminSearchFilter{
		Query:      req.GetQuery(),
		CategoryID: req.GetCategoryId(),
		Status:     req.GetStatus(),
		Sort:       req.GetSort(),
		Page:       req.GetPage(),
		PageSize:   req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list admin products: %v", err)
	}

	out := &catalogv1.ListAdminProductsResponse{
		Page:         res.Page,
		PageSize:     res.PageSize,
		TotalItems:   res.TotalItems,
		TotalPages:   res.TotalPages,
		ActiveCount:  res.ActiveCount,
		DeletedCount: res.DeletedCount,
	}
	for _, item := range res.Items {
		out.Items = append(out.Items, adminSummaryToProto(item))
	}
	for _, cat := range res.Categories {
		out.Categories = append(out.Categories, &catalogv1.CategoryMetric{
			CategoryId:         cat.CategoryID,
			CategoryName:       cat.CategoryName,
			ActiveProductCount: cat.ActiveProductCount,
		})
	}
	return out, nil
}

// ListInventoryCatalogItems returns lightweight catalog metadata for inventory administration.
func (s *Server) ListInventoryCatalogItems(ctx context.Context, req *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	items, err := s.repo.ListInventoryItems(ctx, domain.InventoryCatalogFilter{
		Query:          req.GetQuery(),
		CategoryID:     req.GetCategoryId(),
		IncludeDeleted: req.GetIncludeDeleted(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list inventory catalog items: %v", err)
	}

	resp := &catalogv1.ListInventoryCatalogItemsResponse{
		Items: make([]*catalogv1.InventoryCatalogItem, 0, len(items)),
	}
	for _, item := range items {
		variants := make([]*catalogv1.ProductVariant, 0, len(item.Variants))
		for _, v := range item.Variants {
			variants = append(variants, productVariantToProto(v))
		}
		resp.Items = append(resp.Items, &catalogv1.InventoryCatalogItem{
			Id:           item.ID,
			Name:         item.Name,
			Brand:        item.Brand,
			CategoryId:   item.CategoryID,
			CategoryName: item.CategoryName,
			ImageUrl:     item.ImageURL,
			Emoji:        item.Emoji,
			ImageHue:     item.ImageHue,
			Deleted:      item.Deleted,
			Variants:     variants,
		})
	}
	return resp, nil
}

// GetAdminProduct returns full product detail (including soft-deleted items) after verifying product.view, product.update, or inventory.update.
func (s *Server) GetAdminProduct(ctx context.Context, req *catalogv1.GetAdminProductRequest) (*catalogv1.GetAdminProductResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if err := s.authorizeAny(ctx, req.GetUserId(), productViewPermission, productUpdatePermission, inventoryUpdatePermission); err != nil {
		return nil, err
	}

	product, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, domain.ErrProductNotFound) {
		return nil, notFoundError(id)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get admin product %q: %v", id, err)
	}
	return &catalogv1.GetAdminProductResponse{Product: productToProto(product)}, nil
}

// CreateProduct validates, authorizes, and persists a new product.
func (s *Server) CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.CreateProductResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	variants, err := variantsFromProto(req.GetVariants())
	if err != nil {
		return nil, err
	}

	prepared, err := domain.ValidateAndPrepareCreate(&domain.Product{
		ID:               req.GetId(),
		Name:             req.GetName(),
		ShortDescription: req.GetShortDescription(),
		Description:      req.GetDescription(),
		Brand:            req.GetBrand(),
		CategoryID:       req.GetCategoryId(),
		CategoryName:     req.GetCategoryName(),
		Images:           req.GetImages(),
		Videos:           req.GetVideos(),
		IsNew:            req.GetIsNew(),
		Badge:            req.GetBadge(),
		Featured:         req.GetFeatured(),
		Emoji:            req.GetEmoji(),
		ImageHue:         req.GetImageHue(),
		Variants:         variants,
		Details:          req.GetDetails(),
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.authorize(ctx, req.GetUserId(), productCreatePermission); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, prepared, s.orphanGrace)
	if err != nil {
		if errors.Is(err, domain.ErrProductExists) {
			return nil, statusWithReason(codes.AlreadyExists, "product id already exists", reasonProductExists)
		}
		if errors.Is(err, domain.ErrSkuConflict) {
			return nil, statusWithReason(codes.AlreadyExists, "variant sku_id already exists", reasonSkuExists)
		}
		if errors.Is(err, domain.ErrMediaAlreadyAttached) {
			return nil, statusWithReason(codes.FailedPrecondition, "media asset is already attached to another product", reasonMediaAlreadyAttached)
		}
		if errors.Is(err, domain.ErrMediaExpired) {
			return nil, status.Error(codes.InvalidArgument, "media asset upload has expired; please re-upload")
		}
		return nil, status.Errorf(codes.Internal, "create product: %v", err)
	}
	return &catalogv1.CreateProductResponse{Product: productToProto(created)}, nil
}

// UpdateProduct validates, authorizes, and updates an existing product.
func (s *Server) UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.UpdateProductResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	variants, err := variantsFromProto(req.GetVariants())
	if err != nil {
		return nil, err
	}

	prepared, err := domain.ValidateAndPrepareUpdate(&domain.Product{
		ID:               req.GetId(),
		Name:             req.GetName(),
		ShortDescription: req.GetShortDescription(),
		Description:      req.GetDescription(),
		Brand:            req.GetBrand(),
		CategoryID:       req.GetCategoryId(),
		CategoryName:     req.GetCategoryName(),
		Images:           req.GetImages(),
		Videos:           req.GetVideos(),
		IsNew:            req.GetIsNew(),
		Badge:            req.GetBadge(),
		Featured:         req.GetFeatured(),
		Emoji:            req.GetEmoji(),
		ImageHue:         req.GetImageHue(),
		Variants:         variants,
		Details:          req.GetDetails(),
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.authorize(ctx, req.GetUserId(), productUpdatePermission); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, prepared, s.orphanGrace)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			return nil, notFoundError(prepared.ID)
		}
		if errors.Is(err, domain.ErrSkuConflict) {
			return nil, statusWithReason(codes.AlreadyExists, "variant sku_id already exists", reasonSkuExists)
		}
		if errors.Is(err, domain.ErrMediaAlreadyAttached) {
			return nil, statusWithReason(codes.FailedPrecondition, "media asset is already attached to another product", reasonMediaAlreadyAttached)
		}
		if errors.Is(err, domain.ErrMediaExpired) {
			return nil, status.Error(codes.InvalidArgument, "media asset upload has expired; please re-upload")
		}
		return nil, status.Errorf(codes.Internal, "update product %q: %v", prepared.ID, err)
	}
	return &catalogv1.UpdateProductResponse{Product: productToProto(updated)}, nil
}

// CreateProductMediaUpload authorizes product.create or product.update, records
// a PENDING asset row, and issues a presigned S3 PUT URL.
func (s *Server) CreateProductMediaUpload(ctx context.Context, req *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	spec, err := domain.ValidateMediaUpload(req.GetFilename(), req.GetContentType(), req.GetSizeBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.authorizeAny(ctx, userID, productCreatePermission, productUpdatePermission); err != nil {
		return nil, err
	}

	if s.mediaSigner == nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}
	if err := s.mediaSigner.EnsureBucket(ctx); err != nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}

	uploadID, err := domain.NewUUIDv4()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate upload id: %v", err)
	}
	now := time.Now().UTC()
	objectKey := fmt.Sprintf("products/%04d/%02d/%s%s", now.Year(), int(now.Month()), uploadID, spec.Extension)

	signed, err := s.mediaSigner.PresignPut(objectKey, spec.ContentType, spec.SizeBytes, s.presignTTL)
	if err != nil {
		return nil, statusWithReason(codes.Unavailable, "media storage is unavailable", reasonMediaStorageUnavailable)
	}

	expiresAt := now.Add(s.orphanGrace)
	asset := &domain.MediaAsset{
		ID:               uploadID,
		Bucket:           s.mediaSigner.Bucket(),
		ObjectKey:        objectKey,
		PublicURL:        signed.PublicURL,
		MediaType:        spec.MediaType,
		OriginalFilename: spec.OriginalFilename,
		ContentType:      spec.ContentType,
		SizeBytes:        spec.SizeBytes,
		Status:           domain.MediaStatusPending,
		UploadedBy:       userID,
		ExpiresAt:        &expiresAt,
	}
	if err := s.repo.CreateMediaAsset(ctx, asset); err != nil {
		return nil, status.Errorf(codes.Internal, "record pending media upload: %v", err)
	}

	return &catalogv1.CreateProductMediaUploadResponse{
		UploadId:         uploadID,
		MediaType:        spec.MediaType,
		UploadUrl:        signed.UploadURL,
		PublicUrl:        signed.PublicURL,
		ObjectKey:        objectKey,
		ExpiresInSeconds: signed.ExpiresInSeconds,
	}, nil
}

func extractVariantMoneyBounds(price, originalPrice *commonv1.Money) (int64, int64, error) {
	if price == nil {
		return 0, 0, status.Error(codes.InvalidArgument, "variants.price is required")
	}
	if c := strings.TrimSpace(price.GetCurrency()); c != "" && c != moneyCurrency {
		return 0, 0, status.Error(codes.InvalidArgument, "variants.price currency must be VND")
	}
	origMinor := int64(0)
	if originalPrice != nil {
		if c := strings.TrimSpace(originalPrice.GetCurrency()); c != "" && c != moneyCurrency {
			return 0, 0, status.Error(codes.InvalidArgument, "variants.original_price currency must be VND")
		}
		origMinor = originalPrice.GetAmountMinor()
	}
	return price.GetAmountMinor(), origMinor, nil
}

func variantsFromProto(in []*catalogv1.ProductVariant) ([]domain.ProductVariant, error) {
	out := make([]domain.ProductVariant, 0, len(in))
	for _, v := range in {
		if v == nil {
			continue
		}
		priceMinor, origMinor, err := extractVariantMoneyBounds(v.GetPrice(), v.GetOriginalPrice())
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ProductVariant{
			SkuID:              v.GetSkuId(),
			Label:              v.GetLabel(),
			ColorHex:           v.GetColorHex(),
			PriceMinor:         priceMinor,
			OriginalPriceMinor: origMinor,
		})
	}
	return out, nil
}

func adminSummaryToProto(s *domain.AdminSummary) *catalogv1.AdminProductSummary {
	var createdAt, updatedAt string
	if !s.CreatedAt.IsZero() {
		createdAt = s.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !s.UpdatedAt.IsZero() {
		updatedAt = s.UpdatedAt.UTC().Format(time.RFC3339)
	}
	version := s.Version
	if version <= 0 {
		version = 1
	}
	return &catalogv1.AdminProductSummary{
		Id:            s.ID,
		Name:          s.Name,
		Brand:         s.Brand,
		CategoryId:    s.CategoryID,
		CategoryName:  s.CategoryName,
		Price:         moneyToProto(s.PriceMinor),
		OriginalPrice: moneyToProto(s.OriginalPriceMinor),
		MaxPrice:      moneyToProto(maxPriceProto(s.PriceMinor, s.MaxPriceMinor)),
		Images:        s.Images,
		Rating:        s.Rating,
		ReviewCount:   s.ReviewCount,
		SoldCount:     s.SoldCount,
		IsNew:         s.IsNew,
		Badge:         s.Badge,
		Featured:      s.Featured,
		Emoji:         s.Emoji,
		ImageHue:      s.ImageHue,
		Deleted:       s.Deleted,
		VariantCount:  s.VariantCount,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		VideoCount:    s.VideoCount,
		SkuIds:        s.SkuIDs,
		Version:       version,
	}
}
