// Package catalog provides the BFF client adapter for the catalog service
// gRPC API. It owns the connection and maps catalog outcomes to errors the
// HTTP layer can classify.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
)

var (
	// ErrProductNotFound is returned when the catalog service reports that the
	// requested product does not exist.
	ErrProductNotFound          = errors.New("catalog: product not found")
	ErrPermissionDenied         = errors.New("catalog: permission denied")
	ErrAuthorizationUnavailable = errors.New("catalog: authorization unavailable")
)

// Client is the contract the BFF HTTP layer uses to talk to the catalog
// service. Handlers depend on this interface so they can be tested with fakes.
type Client interface {
	GetProduct(ctx context.Context, id string) (*catalogv1.GetProductResponse, error)
	SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error)
	SoftDeleteProduct(ctx context.Context, userID, id string, deleted bool) error
}

type client struct {
	conn    *grpc.ClientConn
	svc     catalogv1.CatalogServiceClient
	timeout time.Duration
}

// NewClient creates a lazy gRPC connection to the catalog service at target.
// timeout bounds each call; a non-positive value disables the per-call
// deadline.
func NewClient(target string, timeout time.Duration) (*client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to catalog service at %s: %w", target, err)
	}
	return &client{
		conn:    conn,
		svc:     catalogv1.NewCatalogServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Close releases the underlying gRPC connection.
func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) GetProduct(ctx context.Context, id string) (*catalogv1.GetProductResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.GetProduct(ctx, &catalogv1.GetProductRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		return nil, err
	}
	return resp, nil
}

func (c *client) SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	return c.svc.SearchProducts(ctx, req)
}

func (c *client) SoftDeleteProduct(ctx context.Context, userID, id string, deleted bool) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	_, err := c.svc.SoftDeleteProduct(ctx, &catalogv1.SoftDeleteProductRequest{
		Id:      id,
		Deleted: deleted,
		UserId:  userID,
	})
	if err != nil {
		switch catalogErrorReason(err) {
		case "PRODUCT_PERMISSION_DENIED":
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		case "AUTHORIZATION_UNAVAILABLE":
			return fmt.Errorf("%w: %v", ErrAuthorizationUnavailable, err)
		}
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		return err
	}
	return nil
}

func catalogErrorReason(err error) string {
	for _, detail := range status.Convert(err).Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if ok && info.GetDomain() == "ecommerce.catalog.v1" {
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
