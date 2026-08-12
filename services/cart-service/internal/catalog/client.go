// Package catalog provides the Cart adapter for Catalog's private gRPC API.
package catalog

import (
	"context"
	"fmt"
	"time"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	GetProduct(context.Context, string) (*catalogv1.GetProductResponse, error)
}

type client struct {
	conn    *grpc.ClientConn
	service catalogv1.CatalogServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create catalog connection: %w", err)
	}
	return &client{
		conn:    conn,
		service: catalogv1.NewCatalogServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) GetProduct(ctx context.Context, productID string) (*catalogv1.GetProductResponse, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.service.GetProduct(callCtx, &catalogv1.GetProductRequest{Id: productID})
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}
