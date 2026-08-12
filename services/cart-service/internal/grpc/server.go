// Package grpc implements the CartService gRPC surface.
package grpc

import (
	"context"
	"errors"
	"strings"
	"sync"

	"ecommerce.local/cart-service/internal/domain"
	"ecommerce.local/cart-service/internal/store"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cartStore interface {
	EnsureCart(context.Context, string) (string, error)
	FindCart(context.Context, string) (string, bool, error)
	ListItems(context.Context, string) ([]domain.Line, error)
	AddItem(context.Context, string, string, string, int32) error
	UpdateQuantity(context.Context, string, string, int32) error
	RemoveItem(context.Context, string, string) error
	Clear(context.Context, string) error
}

type catalogClient interface {
	GetProduct(context.Context, string) (*catalogv1.GetProductResponse, error)
}

type inventoryClient interface {
	GetAvailability(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error)
}

type Server struct {
	cartv1.UnimplementedCartServiceServer
	store     cartStore
	catalog   catalogClient
	inventory inventoryClient
}

func NewServer(cartStore cartStore, catalog catalogClient, inventory inventoryClient) *Server {
	return &Server{store: cartStore, catalog: catalog, inventory: inventory}
}

func (s *Server) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	userID, err := required(req.GetUserId(), "user_id")
	if err != nil {
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
	userID, err := required(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	productID, err := required(req.GetProductId(), "product_id")
	if err != nil {
		return nil, err
	}
	variantLabel, err := required(req.GetVariantLabel(), "variant_label")
	if err != nil {
		return nil, err
	}
	if err := positiveQuantity(req.GetQuantity()); err != nil {
		return nil, err
	}
	cartID, err := s.store.EnsureCart(ctx, userID)
	if err != nil {
		return nil, internalError("ensure cart", err)
	}
	if err := s.store.AddItem(ctx, cartID, productID, variantLabel, req.GetQuantity()); err != nil {
		return nil, mutationError(err)
	}
	cart, err := s.getCart(ctx, cartID)
	if err != nil {
		return nil, err
	}
	return &cartv1.AddItemResponse{Cart: cart}, nil
}

func (s *Server) UpdateItemQuantity(ctx context.Context, req *cartv1.UpdateItemQuantityRequest) (*cartv1.UpdateItemQuantityResponse, error) {
	userID, err := required(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	itemID, err := required(req.GetItemId(), "item_id")
	if err != nil {
		return nil, err
	}
	if err := positiveQuantity(req.GetQuantity()); err != nil {
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
	userID, err := required(req.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	itemID, err := required(req.GetItemId(), "item_id")
	if err != nil {
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
	userID, err := required(req.GetUserId(), "user_id")
	if err != nil {
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
	enrichments := s.enrichProducts(ctx, lines)
	cart := &cartv1.Cart{Items: make([]*cartv1.CartItem, 0, len(lines))}
	for _, line := range lines {
		item, degraded := enrichLine(line, enrichments[line.ProductID])
		cart.Items = append(cart.Items, item)
		cart.Degraded = cart.Degraded || degraded
	}
	return cart, nil
}

const enrichmentConcurrency = 8

type productEnrichment struct {
	product      *catalogv1.GetProductResponse
	availability *inventoryv1.GetAvailabilityResponse
	catalogErr   error
	inventoryErr error
}

func (s *Server) enrichProducts(ctx context.Context, lines []domain.Line) map[string]productEnrichment {
	productIDs := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	variantLabels := make(map[string][]string, len(lines))
	for _, line := range lines {
		variantLabels[line.ProductID] = append(variantLabels[line.ProductID], line.VariantLabel)
		if _, ok := seen[line.ProductID]; ok {
			continue
		}
		seen[line.ProductID] = struct{}{}
		productIDs = append(productIDs, line.ProductID)
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
				results[index] = s.enrichProduct(ctx, productID, variantLabels[productID])
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

func (s *Server) enrichProduct(ctx context.Context, productID string, requestedLabels []string) productEnrichment {
	product, err := s.catalog.GetProduct(ctx, productID)
	if err != nil {
		return productEnrichment{catalogErr: err}
	}
	if product == nil || product.GetDeleted() {
		return productEnrichment{product: product}
	}
	if !hasRequestedVariant(product, requestedLabels) {
		return productEnrichment{product: product}
	}
	availability, err := s.inventory.GetAvailability(ctx, productID)
	return productEnrichment{product: product, availability: availability, inventoryErr: err}
}

func hasRequestedVariant(product *catalogv1.GetProductResponse, requestedLabels []string) bool {
	for _, variant := range product.GetVariants() {
		for _, requestedLabel := range requestedLabels {
			if variant.GetLabel() == requestedLabel {
				return true
			}
		}
	}
	return false
}

func enrichLine(line domain.Line, enrichment productEnrichment) (*cartv1.CartItem, bool) {
	item := &cartv1.CartItem{
		ItemId:       line.ID,
		ProductId:    line.ProductID,
		VariantLabel: line.VariantLabel,
		Quantity:     line.Quantity,
		Status:       cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE,
	}
	if enrichment.catalogErr != nil {
		if status.Code(enrichment.catalogErr) == codes.NotFound {
			return item, false
		}
		return item, true
	}
	product := enrichment.product
	if product == nil || product.GetDeleted() {
		return item, false
	}
	var matchedVariant *catalogv1.ProductVariant
	for _, variant := range product.GetVariants() {
		if variant.GetLabel() == line.VariantLabel {
			matchedVariant = variant
			break
		}
	}
	if matchedVariant == nil {
		return item, false
	}
	item.Name = product.GetName()
	item.Brand = product.GetBrand()
	item.CategoryName = product.GetCategoryName()
	item.UnitPrice = product.GetPrice()
	item.ImageEmoji = product.GetEmoji()
	item.ImageHue = product.GetImageHue()
	item.ImageUrl = firstString(product.GetImages())
	item.VariantColorHex = matchedVariant.GetColorHex()
	if enrichment.inventoryErr != nil || enrichment.availability == nil {
		return item, true
	}
	available := enrichment.availability.GetAvailableQuantity()
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
