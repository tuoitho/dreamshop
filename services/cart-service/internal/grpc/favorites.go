package grpc

import (
	"context"
	"log/slog"
	"sync"

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
	enrichments := s.enrichFavoriteProducts(ctx, records, preloaded)
	for _, record := range records {
		item, degraded := enrichFavoriteRecord(record, enrichments[record.ProductID])
		list.Items = append(list.Items, item)
		list.Degraded = list.Degraded || degraded
	}
	return list, nil
}

func (s *Server) enrichFavoriteProducts(
	ctx context.Context,
	records []domain.FavoriteRecord,
	preloaded map[string]*catalogv1.GetProductResponse,
) map[string]productEnrichment {
	productIDs := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, ok := seen[record.ProductID]; ok {
			continue
		}
		seen[record.ProductID] = struct{}{}
		productIDs = append(productIDs, record.ProductID)
	}
	if len(productIDs) == 0 {
		return map[string]productEnrichment{}
	}

	results := make([]productEnrichment, len(productIDs))
	jobs := make(chan int)
	workers := enrichmentConcurrency
	if len(productIDs) < workers {
		workers = len(productIDs)
	}
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer waitGroup.Done()
			for index := range jobs {
				productID := productIDs[index]
				results[index] = s.enrichFavoriteProduct(ctx, productID, preloaded[productID])
			}
		}()
	}
	for index := range productIDs {
		jobs <- index
	}
	close(jobs)
	waitGroup.Wait()

	enrichments := make(map[string]productEnrichment, len(productIDs))
	for index, productID := range productIDs {
		enrichments[productID] = results[index]
	}
	return enrichments
}

func (s *Server) enrichFavoriteProduct(ctx context.Context, productID string, preloaded *catalogv1.GetProductResponse) productEnrichment {
	product := preloaded
	if product == nil {
		if s.catalog == nil {
			return productEnrichment{catalogErr: status.Error(codes.Unavailable, "catalog service is unavailable")}
		}
		var err error
		product, err = s.catalog.GetProduct(ctx, productID)
		if err != nil {
			return productEnrichment{catalogErr: err}
		}
	}
	if product == nil || product.GetDeleted() || len(product.GetVariants()) == 0 {
		return productEnrichment{product: product}
	}
	if s.inventory == nil {
		return productEnrichment{
			product:      product,
			inventoryErr: status.Error(codes.Unavailable, "inventory service is unavailable"),
		}
	}
	availability, err := s.inventory.GetAvailability(ctx, productID)
	return productEnrichment{product: product, availability: availability, inventoryErr: err}
}

func enrichFavoriteRecord(record domain.FavoriteRecord, enrichment productEnrichment) (*cartv1.FavoriteItem, bool) {
	item := &cartv1.FavoriteItem{
		ItemId:    record.ID,
		ProductId: record.ProductID,
		Status:    cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE,
	}
	if enrichment.catalogErr != nil {
		if status.Code(enrichment.catalogErr) == codes.NotFound {
			return item, false
		}
		return item, true
	}
	product := enrichment.product
	if product == nil {
		return item, false
	}

	item.Name = product.GetName()
	item.Brand = product.GetBrand()
	item.CategoryId = product.GetCategoryId()
	item.CategoryName = product.GetCategoryName()
	item.Price = product.GetPrice()
	item.OriginalPrice = product.GetOriginalPrice()
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
			Label:    variant.GetLabel(),
			ColorHex: variant.GetColorHex(),
		})
	}
	item.Variants = variants

	if product.GetDeleted() || len(variants) == 0 {
		return item, false
	}

	inventoryDegraded := enrichment.inventoryErr != nil || enrichment.availability == nil
	var available int32
	if !inventoryDegraded {
		available = enrichment.availability.GetAvailableQuantity()
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
