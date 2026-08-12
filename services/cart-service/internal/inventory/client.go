// Package inventory provides the Cart adapter for Inventory's private gRPC API.
package inventory

import (
	"context"
	"fmt"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	GetAvailability(context.Context, string) (*inventoryv1.GetAvailabilityResponse, error)
}

type client struct {
	conn    *grpc.ClientConn
	service inventoryv1.InventoryServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create inventory connection: %w", err)
	}
	return &client{
		conn:    conn,
		service: inventoryv1.NewInventoryServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.service.GetAvailability(callCtx, &inventoryv1.GetAvailabilityRequest{ProductId: productID})
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}
