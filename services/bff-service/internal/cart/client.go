// Package cart provides the BFF client adapter for the cart service gRPC API.
package cart

import (
	"context"
	"errors"
	"fmt"
	"time"

	cartv1 "ecommerce.local/contracts/go/cart/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	ErrCartItemNotFound         = errors.New("cart: item not found")
	ErrProductNotFound          = errors.New("cart: product not found")
	ErrProductUnavailable       = errors.New("cart: product unavailable")
	ErrPermissionDenied         = errors.New("cart: permission denied")
	ErrAuthorizationUnavailable = errors.New("cart: authorization unavailable")
)

type Client interface {
	GetCart(ctx context.Context, userID string) (*cartv1.GetCartResponse, error)
	AddItem(ctx context.Context, userID, productID, variantLabel string, quantity int32) (*cartv1.AddItemResponse, error)
	UpdateItemQuantity(ctx context.Context, userID, itemID string, quantity int32) (*cartv1.UpdateItemQuantityResponse, error)
	RemoveItem(ctx context.Context, userID, itemID string) (*cartv1.RemoveItemResponse, error)
	ClearCart(ctx context.Context, userID string) (*cartv1.ClearCartResponse, error)
	GetFavorites(ctx context.Context, userID string) (*cartv1.GetFavoritesResponse, error)
	AddFavorite(ctx context.Context, userID, productID string) (*cartv1.AddFavoriteResponse, error)
	RemoveFavorite(ctx context.Context, userID, productID string) (*cartv1.RemoveFavoriteResponse, error)
	ClearFavorites(ctx context.Context, userID string) (*cartv1.ClearFavoritesResponse, error)
}

type client struct {
	conn    *grpc.ClientConn
	svc     cartv1.CartServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to cart service at %s: %w", target, err)
	}
	return &client{conn: conn, svc: cartv1.NewCartServiceClient(conn), timeout: timeout}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) GetCart(ctx context.Context, userID string) (*cartv1.GetCartResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.GetCart(ctx, &cartv1.GetCartRequest{UserId: userID})
	return resp, c.mapError(err)
}

func (c *client) AddItem(ctx context.Context, userID, productID, variantLabel string, quantity int32) (*cartv1.AddItemResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.AddItem(ctx, &cartv1.AddItemRequest{UserId: userID, ProductId: productID, VariantLabel: variantLabel, Quantity: quantity})
	return resp, c.mapError(err)
}

func (c *client) UpdateItemQuantity(ctx context.Context, userID, itemID string, quantity int32) (*cartv1.UpdateItemQuantityResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.UpdateItemQuantity(ctx, &cartv1.UpdateItemQuantityRequest{UserId: userID, ItemId: itemID, Quantity: quantity})
	return resp, c.mapError(err)
}

func (c *client) RemoveItem(ctx context.Context, userID, itemID string) (*cartv1.RemoveItemResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.RemoveItem(ctx, &cartv1.RemoveItemRequest{UserId: userID, ItemId: itemID})
	return resp, c.mapError(err)
}

func (c *client) ClearCart(ctx context.Context, userID string) (*cartv1.ClearCartResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.ClearCart(ctx, &cartv1.ClearCartRequest{UserId: userID})
	return resp, c.mapError(err)
}

func (c *client) GetFavorites(ctx context.Context, userID string) (*cartv1.GetFavoritesResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.GetFavorites(ctx, &cartv1.GetFavoritesRequest{UserId: userID})
	return resp, c.mapFavoriteError(err)
}

func (c *client) AddFavorite(ctx context.Context, userID, productID string) (*cartv1.AddFavoriteResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.AddFavorite(ctx, &cartv1.AddFavoriteRequest{UserId: userID, ProductId: productID})
	return resp, c.mapFavoriteError(err)
}

func (c *client) RemoveFavorite(ctx context.Context, userID, productID string) (*cartv1.RemoveFavoriteResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.RemoveFavorite(ctx, &cartv1.RemoveFavoriteRequest{UserId: userID, ProductId: productID})
	return resp, c.mapFavoriteError(err)
}

func (c *client) ClearFavorites(ctx context.Context, userID string) (*cartv1.ClearFavoritesResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	resp, err := c.svc.ClearFavorites(ctx, &cartv1.ClearFavoritesRequest{UserId: userID})
	return resp, c.mapFavoriteError(err)
}

func (c *client) mapError(err error) error {
	if err == nil {
		return nil
	}
	if reason := cartErrorReason(err); reason == "CART_PERMISSION_DENIED" {
		return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
	} else if reason == "AUTHORIZATION_UNAVAILABLE" {
		return fmt.Errorf("%w: %v", ErrAuthorizationUnavailable, err)
	}
	if status.Code(err) == codes.NotFound {
		return fmt.Errorf("%w: %v", ErrCartItemNotFound, err)
	}
	return err
}

func (c *client) mapFavoriteError(err error) error {
	if err == nil {
		return nil
	}
	switch cartErrorReason(err) {
	case "FAVORITE_PERMISSION_DENIED":
		return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
	case "AUTHORIZATION_UNAVAILABLE":
		return fmt.Errorf("%w: %v", ErrAuthorizationUnavailable, err)
	case "FAVORITE_PRODUCT_UNAVAILABLE":
		return fmt.Errorf("%w: %v", ErrProductUnavailable, err)
	}
	if status.Code(err) == codes.NotFound {
		return fmt.Errorf("%w: %v", ErrProductNotFound, err)
	}
	return err
}

func cartErrorReason(err error) string {
	for _, detail := range status.Convert(err).Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if ok && info.GetDomain() == "ecommerce.cart.v1" {
			return info.GetReason()
		}
	}
	return ""
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}
