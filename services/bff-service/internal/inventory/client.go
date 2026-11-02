// Package inventory provides the BFF client adapter for the inventory service
// gRPC API.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	ErrPermissionDenied         = errors.New("inventory: permission denied")
	ErrAuthorizationUnavailable = errors.New("inventory: authorization unavailable")
	ErrStockBelowReserved       = errors.New("inventory: stock below reserved")
	ErrVariantHasReservedStock  = errors.New("inventory: variant has reserved stock")
)

// VariantHasReservedStockError carries the SKU ID whose active reservation
// blocked variant deletion.
type VariantHasReservedStockError struct {
	SkuID string
	Cause error
}

func (e *VariantHasReservedStockError) Error() string {
	if e.SkuID != "" {
		return fmt.Sprintf("%s (%s): %v", ErrVariantHasReservedStock.Error(), e.SkuID, e.Cause)
	}
	return fmt.Sprintf("%s: %v", ErrVariantHasReservedStock.Error(), e.Cause)
}

func (e *VariantHasReservedStockError) Unwrap() error {
	return ErrVariantHasReservedStock
}

// Client is the contract the BFF HTTP layer uses to query and mutate SKU stock.
type Client interface {
	GetAvailability(ctx context.Context, skuID string) (*inventoryv1.GetAvailabilityResponse, error)
	BatchGetAvailability(ctx context.Context, skuIDs []string) ([]*inventoryv1.AvailabilityItem, error)
	GetStock(ctx context.Context, userID, skuID string) (*inventoryv1.StockItem, error)
	BatchGetStock(ctx context.Context, userID string, skuIDs []string) ([]*inventoryv1.StockItem, error)
	ListStocks(ctx context.Context, userID string) ([]*inventoryv1.StockItem, error)
	SetStock(ctx context.Context, userID, skuID, productID string, quantity int32) (*inventoryv1.StockItem, error)
	AdjustStock(ctx context.Context, userID, skuID, productID string, delta int32) (*inventoryv1.StockItem, error)
	DeleteStocks(ctx context.Context, userID string, skuIDs []string, dryRun bool) error
}

type client struct {
	conn    *grpc.ClientConn
	svc     inventoryv1.InventoryServiceClient
	timeout time.Duration
}

// NewClient creates a lazy gRPC connection to the inventory service at target.
// timeout bounds each call; a non-positive value disables the per-call deadline.
func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to inventory service at %s: %w", target, err)
	}
	return &client{
		conn:    conn,
		svc:     inventoryv1.NewInventoryServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Close releases the underlying gRPC connection.
func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) GetAvailability(ctx context.Context, skuID string) (*inventoryv1.GetAvailabilityResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	return c.svc.GetAvailability(ctx, &inventoryv1.GetAvailabilityRequest{SkuId: skuID})
}

func (c *client) BatchGetAvailability(ctx context.Context, skuIDs []string) ([]*inventoryv1.AvailabilityItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.BatchGetAvailability(ctx, &inventoryv1.BatchGetAvailabilityRequest{SkuIds: skuIDs})
	if err != nil {
		return nil, err
	}
	return resp.GetItems(), nil
}

func (c *client) GetStock(ctx context.Context, userID, skuID string) (*inventoryv1.StockItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.GetStock(ctx, &inventoryv1.GetStockRequest{
		UserId: userID,
		SkuId:  skuID,
	})
	if err != nil {
		return nil, classifyInventoryError(err)
	}
	return resp.GetStock(), nil
}

func (c *client) BatchGetStock(ctx context.Context, userID string, skuIDs []string) ([]*inventoryv1.StockItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.BatchGetStock(ctx, &inventoryv1.BatchGetStockRequest{
		UserId: userID,
		SkuIds: skuIDs,
	})
	if err != nil {
		return nil, classifyInventoryError(err)
	}
	return resp.GetItems(), nil
}

func (c *client) ListStocks(ctx context.Context, userID string) ([]*inventoryv1.StockItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ListStocks(ctx, &inventoryv1.ListStocksRequest{UserId: userID})
	if err != nil {
		return nil, classifyInventoryError(err)
	}
	return resp.GetItems(), nil
}

func (c *client) SetStock(ctx context.Context, userID, skuID, productID string, quantity int32) (*inventoryv1.StockItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.SetStock(ctx, &inventoryv1.SetStockRequest{
		UserId:    userID,
		SkuId:     skuID,
		ProductId: productID,
		Quantity:  quantity,
	})
	if err != nil {
		return nil, classifyInventoryError(err)
	}
	return resp.GetStock(), nil
}

func (c *client) AdjustStock(ctx context.Context, userID, skuID, productID string, delta int32) (*inventoryv1.StockItem, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.AdjustStock(ctx, &inventoryv1.AdjustStockRequest{
		UserId:    userID,
		SkuId:     skuID,
		ProductId: productID,
		Delta:     delta,
	})
	if err != nil {
		return nil, classifyInventoryError(err)
	}
	return resp.GetStock(), nil
}

func (c *client) DeleteStocks(ctx context.Context, userID string, skuIDs []string, dryRun bool) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	_, err := c.svc.DeleteStocks(ctx, &inventoryv1.DeleteStocksRequest{
		UserId: userID,
		SkuIds: skuIDs,
		DryRun: dryRun,
	})
	if err != nil {
		return classifyInventoryError(err)
	}
	return nil
}

func classifyInventoryError(err error) error {
	for _, detail := range status.Convert(err).Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetDomain() != "ecommerce.inventory.v1" {
			continue
		}
		switch info.GetReason() {
		case "INVENTORY_PERMISSION_DENIED":
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		case "AUTHORIZATION_UNAVAILABLE":
			return fmt.Errorf("%w: %v", ErrAuthorizationUnavailable, err)
		case "STOCK_BELOW_RESERVED":
			return fmt.Errorf("%w: %v", ErrStockBelowReserved, err)
		case "VARIANT_HAS_RESERVED_STOCK":
			return &VariantHasReservedStockError{
				SkuID: info.GetMetadata()["sku_id"],
				Cause: err,
			}
		}
	}
	return err
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}
