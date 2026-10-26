// Package inventory provides the BFF client adapter for the inventory service
// gRPC API.
package inventory

import (
	"context"
	"fmt"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the contract the BFF HTTP layer uses to query stock availability.
type Client interface {
	GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error)
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

func (c *client) GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	return c.svc.GetAvailability(ctx, &inventoryv1.GetAvailabilityRequest{ProductId: productID})
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}
