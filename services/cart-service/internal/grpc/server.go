// Package grpc implements the CartService gRPC surface.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"ecommerce.local/cart-service/internal/domain"
	"ecommerce.local/cart-service/internal/store"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	cartViewPermission             = "cart.view"
	cartItemAddPermission          = "cart.item.add"
	cartItemUpdatePermission       = "cart.item.update"
	cartItemRemovePermission       = "cart.item.remove"
	cartClearPermission            = "cart.clear"
	grpcErrorDomain                = "ecommerce.cart.v1"
	reasonCartPermissionDenied     = "CART_PERMISSION_DENIED"
	reasonAuthorizationUnavailable = "AUTHORIZATION_UNAVAILABLE"
	traceMetadataKey               = "x-trace-id"
	maxTraceIDLength               = 64
)

type cartStore interface {
	EnsureCart(context.Context, string) (string, error)
	FindCart(context.Context, string) (string, bool, error)
	ListItems(context.Context, string) ([]domain.Line, error)
	AddItem(context.Context, string, string, string, int32) error
	UpdateQuantity(context.Context, string, string, int32) error
	RemoveItem(context.Context, string, string) error
	Clear(context.Context, string) error
	ListFavorites(context.Context, string) ([]domain.FavoriteRecord, error)
	AddFavorite(context.Context, string, string) error
	RemoveFavorite(context.Context, string, string) error
	ClearFavorites(context.Context, string) error
}

type catalogClient interface {
	GetProduct(context.Context, string) (*catalogv1.GetProductResponse, error)
}

type inventoryClient interface {
	BatchGetAvailability(context.Context, []string) ([]*inventoryv1.AvailabilityItem, error)
}

type authorizationClient interface {
	CheckPermission(context.Context, string, string) (bool, error)
}

type Server struct {
	cartv1.UnimplementedCartServiceServer
	store         cartStore
	catalog       catalogClient
	inventory     inventoryClient
	authorization authorizationClient
}

func NewServer(cartStore cartStore, catalog catalogClient, inventory inventoryClient, authorization authorizationClient) *Server {
	return &Server{store: cartStore, catalog: catalog, inventory: inventory, authorization: authorization}
}

func (s *Server) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	if err := s.authorizeCart(ctx, userID, cartViewPermission); err != nil {
		return nil, err
	}
	cartID, found, err := s.store.FindCart(ctx, userID)
	if err != nil {
		return nil, internalError("find cart", err)
	}
	if !found {
		return &cartv1.GetCartResponse{Cart: &cartv1.Cart{}}, nil
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.GetCartResponse{Cart: cart}, nil
}

func (s *Server) AddItem(ctx context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	productID, err := required(req.GetProductId(), "product_id")
	if err != nil {
		return nil, err
	}
	skuID, err := required(req.GetSkuId(), "sku_id")
	if err != nil {
		return nil, err
	}
	if err := positiveQuantity(req.GetQuantity()); err != nil {
		return nil, err
	}
	if err := s.authorizeCart(ctx, userID, cartItemAddPermission); err != nil {
		return nil, err
	}
	cartID, err := s.store.EnsureCart(ctx, userID)
	if err != nil {
		return nil, internalError("ensure cart", err)
	}
	if err := s.store.AddItem(ctx, cartID, productID, skuID, req.GetQuantity()); err != nil {
		return nil, mutationError(err)
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.AddItemResponse{Cart: cart}, nil
}

func (s *Server) authorizeCart(ctx context.Context, userID, permission string) error {
	if s.authorization == nil {
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	authorizationCtx, traceID := withAuthorizationTrace(ctx)
	allowed, err := s.authorization.CheckPermission(authorizationCtx, userID, permission)
	if err != nil {
		if ctx.Err() != nil {
			return status.FromContextError(ctx.Err()).Err()
		}
		slog.WarnContext(ctx, "cart authorization unavailable", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	if !allowed {
		slog.DebugContext(ctx, "cart authorization denied", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.PermissionDenied, "cart permission denied", reasonCartPermissionDenied)
	}
	return nil
}

func withAuthorizationTrace(ctx context.Context) (context.Context, string) {
	incoming, _ := metadata.FromIncomingContext(ctx)
	values := incoming.Get(traceMetadataKey)
	if len(values) != 1 || !validTraceID(values[0]) {
		return ctx, ""
	}
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	outgoing = outgoing.Copy()
	outgoing.Set(traceMetadataKey, values[0])
	return metadata.NewOutgoingContext(ctx, outgoing), values[0]
}

func validTraceID(candidate string) bool {
	if len(candidate) == 0 || len(candidate) > maxTraceIDLength {
		return false
	}
	for i := range len(candidate) {
		value := candidate[i]
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '-' || value == '_' || value == '.' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) UpdateItemQuantity(ctx context.Context, req *cartv1.UpdateItemQuantityRequest) (*cartv1.UpdateItemQuantityResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	itemID, err := requiredUUID(req.GetItemId(), "item_id")
	if err != nil {
		return nil, err
	}
	if err := positiveQuantity(req.GetQuantity()); err != nil {
		return nil, err
	}
	if err := s.authorizeCart(ctx, userID, cartItemUpdatePermission); err != nil {
		return nil, err
	}
	cartID, err := s.store.EnsureCart(ctx, userID)
	if err != nil {
		return nil, internalError("ensure cart", err)
	}
	if err := s.store.UpdateQuantity(ctx, cartID, itemID, req.GetQuantity()); err != nil {
		return nil, mutationError(err)
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.UpdateItemQuantityResponse{Cart: cart}, nil
}

func (s *Server) RemoveItem(ctx context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	itemID, err := requiredUUID(req.GetItemId(), "item_id")
	if err != nil {
		return nil, err
	}
	if err := s.authorizeCart(ctx, userID, cartItemRemovePermission); err != nil {
		return nil, err
	}
	cartID, err := s.store.EnsureCart(ctx, userID)
	if err != nil {
		return nil, internalError("ensure cart", err)
	}
	if err := s.store.RemoveItem(ctx, cartID, itemID); err != nil {
		return nil, mutationError(err)
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.RemoveItemResponse{Cart: cart}, nil
}

func (s *Server) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	userID, err := requiredUUID(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	if err := s.authorizeCart(ctx, userID, cartClearPermission); err != nil {
		return nil, err
	}
	cartID, err := s.store.EnsureCart(ctx, userID)
	if err != nil {
		return nil, internalError("ensure cart", err)
	}
	if err := s.store.Clear(ctx, cartID); err != nil {
		return nil, internalError("clear cart", err)
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.ClearCartResponse{Cart: cart}, nil
}

func (s *Server) getCart(ctx context.Context, cartID string) (*cartv1.Cart, error) {
	lines, err := s.store.ListItems(ctx, cartID)
	if err != nil {
		return nil, internalError("list cart items", err)
	}
	cart := &cartv1.Cart{Items: make([]*cartv1.CartItem, 0, len(lines))}
	if len(lines) == 0 {
		return cart, nil
	}

	productIDs := make([]string, 0, len(lines))
	seenProducts := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		if _, ok := seenProducts[line.ProductID]; ok {
			continue
		}
		seenProducts[line.ProductID] = struct{}{}
		productIDs = append(productIDs, line.ProductID)
	}
	catalogResults := s.fetchCatalogProducts(ctx, productIDs, nil)

	skuIDs := make([]string, 0, len(lines))
	seenSkus := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		lookup := catalogResults[line.ProductID]
		if lookup.err != nil || lookup.product == nil || lookup.product.GetDeleted() {
			continue
		}
		if findVariantBySku(lookup.product, line.SkuID) == nil {
			continue
		}
		if _, ok := seenSkus[line.SkuID]; ok {
			continue
		}
		seenSkus[line.SkuID] = struct{}{}
		skuIDs = append(skuIDs, line.SkuID)
	}

	availabilityBySku, inventoryErr := s.fetchSkuAvailability(ctx, skuIDs)
	for _, line := range lines {
		item, degraded := enrichLine(line, catalogResults[line.ProductID], availabilityBySku, inventoryErr)
		cart.Items = append(cart.Items, item)
		cart.Degraded = cart.Degraded || degraded
	}
	return cart, nil
}

const enrichmentConcurrency = 8

type catalogLookupResult struct {
	product *catalogv1.GetProductResponse
	err     error
}

func (s *Server) fetchCatalogProducts(
	ctx context.Context,
	productIDs []string,
	preloaded map[string]*catalogv1.GetProductResponse,
) map[string]catalogLookupResult {
	if len(productIDs) == 0 {
		return map[string]catalogLookupResult{}
	}

	results := make([]catalogLookupResult, len(productIDs))
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
				if preloaded != nil {
					if cached := preloaded[productID]; cached != nil {
						results[index] = catalogLookupResult{product: cached}
						continue
					}
				}
				if s.catalog == nil {
					results[index] = catalogLookupResult{err: status.Error(codes.Unavailable, "catalog service is unavailable")}
					continue
				}
				product, err := s.catalog.GetProduct(ctx, productID)
				results[index] = catalogLookupResult{product: product, err: err}
			}
		}()
	}
	for index := range productIDs {
		jobs <- index
	}
	close(jobs)
	waitGroup.Wait()

	lookups := make(map[string]catalogLookupResult, len(productIDs))
	for index, productID := range productIDs {
		lookups[productID] = results[index]
	}
	return lookups
}

func (s *Server) fetchSkuAvailability(ctx context.Context, skuIDs []string) (map[string]int32, error) {
	if len(skuIDs) == 0 {
		return nil, nil
	}
	if s.inventory == nil {
		return nil, status.Error(codes.Unavailable, "inventory service is unavailable")
	}
	items, err := s.inventory.BatchGetAvailability(ctx, skuIDs)
	if err != nil {
		return nil, err
	}
	bySku := make(map[string]int32, len(items))
	for _, item := range items {
		bySku[item.GetSkuId()] = item.GetAvailableQuantity()
	}
	return bySku, nil
}

func findVariantBySku(product *catalogv1.GetProductResponse, skuID string) *catalogv1.ProductVariant {
	for _, variant := range product.GetVariants() {
		if variant.GetSkuId() == skuID {
			return variant
		}
	}
	return nil
}

func enrichLine(
	line domain.Line,
	lookup catalogLookupResult,
	availabilityBySku map[string]int32,
	inventoryErr error,
) (*cartv1.CartItem, bool) {
	item := &cartv1.CartItem{
		ItemId:    line.ID,
		ProductId: line.ProductID,
		SkuId:     line.SkuID,
		Quantity:  line.Quantity,
		Status:    cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE,
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
	item.CategoryName = product.GetCategoryName()
	item.UnitPrice = product.GetPrice()
	item.OriginalPrice = product.GetOriginalPrice()
	item.ImageEmoji = product.GetEmoji()
	item.ImageHue = product.GetImageHue()
	item.ImageUrl = firstString(product.GetImages())

	matchedVariant := findVariantBySku(product, line.SkuID)
	if matchedVariant != nil {
		item.VariantLabel = matchedVariant.GetLabel()
		item.VariantColorHex = matchedVariant.GetColorHex()
		item.UnitPrice = matchedVariant.GetPrice()
		item.OriginalPrice = matchedVariant.GetOriginalPrice()
	}
	if product.GetDeleted() || matchedVariant == nil {
		return item, false
	}
	if inventoryErr != nil {
		return item, true
	}
	available := availabilityBySku[line.SkuID]
	resolved := domain.ResolveStatus(true, false, true, available, line.Quantity)
	item.Status = statusToProto(resolved)
	item.AvailableQuantity = available
	return item, false
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func statusToProto(statusValue domain.ItemStatus) cartv1.CartItemStatus {
	switch statusValue {
	case domain.StatusAvailable:
		return cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE
	case domain.StatusLimited:
		return cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED
	case domain.StatusOutOfStock:
		return cartv1.CartItemStatus_CART_ITEM_STATUS_OUT_OF_STOCK
	default:
		return cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE
	}
}

func required(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	return value, nil
}

func requiredUUID(value, field string) (string, error) {
	value, err := required(value, field)
	if err != nil {
		return "", err
	}
	if !isValidUUID(value) {
		return "", status.Errorf(codes.InvalidArgument, "%s must be a valid UUID", field)
	}
	return value, nil
}

func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := range len(value) {
		c := value[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}
	return true
}

func positiveQuantity(quantity int32) error {
	if quantity <= 0 {
		return status.Error(codes.InvalidArgument, "quantity must be positive")
	}
	return nil
}

func mutationError(err error) error {
	if errors.Is(err, store.ErrItemNotFound) {
		return status.Error(codes.NotFound, "cart item not found")
	}
	return internalError("cart mutation", err)
}

func internalError(operation string, err error) error {
	return status.Errorf(codes.Internal, "%s failed", operation)
}

func statusWithReason(code codes.Code, message, reason string) error {
	base := status.New(code, message)
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: grpcErrorDomain})
	if err != nil {
		return base.Err()
	}
	return withDetails.Err()
}
