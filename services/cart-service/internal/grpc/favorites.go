package grpc

import (
	"context"
	"log/slog"
	"strings"

	"ecommerce.local/cart-service/internal/domain"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	favoriteManagePermission         = "favorite.manage"
	reasonFavoritePermissionDenied   = "FAVORITE_PERMISSION_DENIED"
	reasonFavoriteProductUnavailable = "FAVORITE_PRODUCT_UNAVAILABLE"
	maxProductIDLength               = 128
)

func (s *Server) GetFavorites(ctx context.Context, req *cartv1.GetFavoritesRequest) (*cartv1.GetFavoritesResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	if err := s.authorizeFavoriteManage(ctx, userID); err != nil {
		return nil, err
	}
	favorites, err := s.getFavorites(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	return &cartv1.GetFavoritesResponse{Favorites: favorites}, nil
}

func (s *Server) AddFavorite(ctx context.Context, req *cartv1.AddFavoriteRequest) (*cartv1.AddFavoriteResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	productID, err := requiredProductID(req.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := s.authorizeFavoriteManage(ctx, userID); err != nil {
		return nil, err
	}
	if s.catalog == nil {
		return nil, status.Error(codes.Unavailable, "catalog service is unavailable")
	}
	product, err := s.catalog.GetProduct(ctx, productID)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		if status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, status.Error(codes.Unavailable, "catalog service is unavailable")
	}
	if product == nil {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	if product.GetDeleted() || len(product.GetVariants()) == 0 {
		return nil, statusWithReason(codes.FailedPrecondition, "product is unavailable", reasonFavoriteProductUnavailable)
	}
	if err := s.store.AddFavorite(ctx, userID, productID); err != nil {
		return nil, internalError("add favorite", err)
	}
	favorites, err := s.getFavorites(ctx, userID, map[string]*catalogv1.GetProductResponse{productID: product})
	if err != nil {
		return nil, err
	}
	return &cartv1.AddFavoriteResponse{Favorites: favorites}, nil
}

func (s *Server) RemoveFavorite(ctx context.Context, req *cartv1.RemoveFavoriteRequest) (*cartv1.RemoveFavoriteResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	productID, err := requiredProductID(req.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := s.authorizeFavoriteManage(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.store.RemoveFavorite(ctx, userID, productID); err != nil {
		return nil, internalError("remove favorite", err)
	}
	favorites, err := s.getFavorites(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	return &cartv1.RemoveFavoriteResponse{Favorites: favorites}, nil
}

func (s *Server) ClearFavorites(ctx context.Context, req *cartv1.ClearFavoritesRequest) (*cartv1.ClearFavoritesResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	if err := s.authorizeFavoriteManage(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.store.ClearFavorites(ctx, userID); err != nil {
		return nil, internalError("clear favorites", err)
	}
	favorites, err := s.getFavorites(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	return &cartv1.ClearFavoritesResponse{Favorites: favorites}, nil
}

func (s *Server) authorizeFavoriteManage(ctx context.Context, userID string) error {
	if s.authorization == nil {
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	authorizationCtx, traceID := withAuthorizationTrace(ctx)
	allowed, err := s.authorization.CheckPermission(authorizationCtx, userID, favoriteManagePermission)
	if err != nil {
		if ctx.Err() != nil {
			return status.FromContextError(ctx.Err()).Err()
		}
		slog.WarnContext(ctx, "favorite authorization unavailable", "user_id", userID,
			"permission", favoriteManagePermission, "trace_id", traceID)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	if !allowed {
		slog.DebugContext(ctx, "favorite authorization denied", "user_id", userID,
			"permission", favoriteManagePermission, "trace_id", traceID)
		return statusWithReason(codes.PermissionDenied, "favorite permission denied", reasonFavoritePermissionDenied)
	}
	return nil
}

func requiredProductID(value string) (string, error) {
	trimmed, err := required(value, "product_id")
	if err != nil {
		return "", err
	}
	if len(trimmed) > maxProductIDLength {
		return "", status.Error(codes.InvalidArgument, "product_id must not exceed 128 characters")
	}
	return trimmed, nil
}

func (s *Server) getFavorites(ctx context.Context, userID string, preloaded map[string]*catalogv1.GetProductResponse) (*cartv1.FavoriteList, error) {
	records, err := s.store.ListFavorites(ctx, userID)
	if err != nil {
		return nil, internalError("list favorites", err)
	}
	list := &cartv1.FavoriteList{Items: make([]*cartv1.FavoriteItem, 0, len(records))}
	if len(records) == 0 {
		return list, nil
	}

	productIDs := make([]string, 0, len(records))
	seenProducts := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, ok := seenProducts[record.ProductID]; ok {
			continue
		}
		seenProducts[record.ProductID] = struct{}{}
		productIDs = append(productIDs, record.ProductID)
	}
	catalogResults := s.fetchCatalogProducts(ctx, productIDs, preloaded)

	allSkuIDs := make([]string, 0, len(productIDs)*2)
	seenSkus := make(map[string]struct{}, len(productIDs)*2)
	for _, productID := range productIDs {
		lookup := catalogResults[productID]
		if lookup.err != nil || lookup.product == nil || lookup.product.GetDeleted() || len(lookup.product.GetVariants()) == 0 {
			continue
		}
		for _, variant := range lookup.product.GetVariants() {
			skuID := strings.TrimSpace(variant.GetSkuId())
			if skuID == "" {
				continue
			}
			if _, ok := seenSkus[skuID]; ok {
				continue
			}
			seenSkus[skuID] = struct{}{}
			allSkuIDs = append(allSkuIDs, skuID)
		}
	}

	availabilityBySku, inventoryErr := s.fetchSkuAvailability(ctx, allSkuIDs)
	for _, record := range records {
		item, degraded := enrichFavoriteRecord(record, catalogResults[record.ProductID], availabilityBySku, inventoryErr)
		list.Items = append(list.Items, item)
		list.Degraded = list.Degraded || degraded
	}
	return list, nil
}

func enrichFavoriteRecord(
	record domain.FavoriteRecord,
	lookup catalogLookupResult,
	availabilityBySku map[string]int32,
	inventoryErr error,
) (*cartv1.FavoriteItem, bool) {
	item := &cartv1.FavoriteItem{
		ItemId:    record.ID,
		ProductId: record.ProductID,
		Status:    cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE,
	}
	if lookup.err != nil {
		if status.Code(lookup.err) == codes.NotFound {
			return item, false
		}
		return item, true
	}
	product := lookup.product
	if product == nil {
		return item, false
	}

	item.Name = product.GetName()
	item.Brand = product.GetBrand()
	item.CategoryId = product.GetCategoryId()
	item.CategoryName = product.GetCategoryName()
	item.Price = product.GetPrice()
	item.OriginalPrice = product.GetOriginalPrice()
	item.MaxPrice = product.GetMaxPrice()
	item.Rating = product.GetRating()
	item.ReviewCount = product.GetReviewCount()
	item.SoldCount = product.GetSoldCount()
	item.ImageUrl = firstString(product.GetImages())
	item.ImageEmoji = product.GetEmoji()
	item.ImageHue = product.GetImageHue()
	item.Badge = product.GetBadge()

	variants := make([]*cartv1.FavoriteVariant, 0, len(product.GetVariants()))
	for _, variant := range product.GetVariants() {
		variants = append(variants, &cartv1.FavoriteVariant{
			SkuId:         variant.GetSkuId(),
			Label:         variant.GetLabel(),
			ColorHex:      variant.GetColorHex(),
			Price:         variant.GetPrice(),
			OriginalPrice: variant.GetOriginalPrice(),
		})
	}
	item.Variants = variants

	if product.GetDeleted() || len(variants) == 0 {
		return item, false
	}

	inventoryDegraded := inventoryErr != nil
	var available int32
	if !inventoryDegraded {
		for _, variant := range product.GetVariants() {
			available += availabilityBySku[variant.GetSkuId()]
		}
	}
	resolved := domain.ResolveFavoriteStatus(true, false, len(variants), inventoryDegraded, available)
	item.Status = favoriteStatusToProto(resolved)
	if !inventoryDegraded && available > 0 {
		item.AvailableQuantity = available
	}
	return item, inventoryDegraded
}

func favoriteStatusToProto(statusValue domain.FavoriteStatus) cartv1.FavoriteItemStatus {
	switch statusValue {
	case domain.FavoriteStatusAvailable:
		return cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_AVAILABLE
	case domain.FavoriteStatusLimited:
		return cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_LIMITED
	case domain.FavoriteStatusOutOfStock:
		return cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_OUT_OF_STOCK
	case domain.FavoriteStatusUnknown:
		return cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNKNOWN
	default:
		return cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE
	}
}
