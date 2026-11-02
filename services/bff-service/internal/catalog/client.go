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
	ErrProductExists            = errors.New("catalog: product already exists")
	ErrSkuConflict              = errors.New("catalog: sku already exists")
	ErrMediaAlreadyAttached     = errors.New("catalog: media already attached")
	ErrMediaStorageUnavailable  = errors.New("catalog: media storage unavailable")
	ErrPermissionDenied         = errors.New("catalog: permission denied")
	ErrAuthorizationUnavailable = errors.New("catalog: authorization unavailable")
)

// Client is the contract the BFF HTTP layer uses to talk to the catalog
// service. Handlers depend on this interface so they can be tested with fakes.
type Client interface {
	GetProduct(ctx context.Context, id string) (*catalogv1.GetProductResponse, error)
	SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error)
	SoftDeleteProduct(ctx context.Context, userID, id string, deleted bool) error
	ListAdminProducts(ctx context.Context, req *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error)
	ListInventoryCatalogItems(ctx context.Context, req *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error)
	GetAdminProduct(ctx context.Context, userID, id string) (*catalogv1.GetProductResponse, error)
	CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.GetProductResponse, error)
	UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.GetProductResponse, error)
	CreateProductMediaUpload(ctx context.Context, req *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error)
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
		return nil, classifyCatalogError(err, id)
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
		return classifyCatalogError(err, id)
	}
	return nil
}

func (c *client) ListAdminProducts(ctx context.Context, req *catalogv1.ListAdminProductsRequest) (*catalogv1.ListAdminProductsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ListAdminProducts(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, "")
	}
	return resp, nil
}

func (c *client) ListInventoryCatalogItems(ctx context.Context, req *catalogv1.ListInventoryCatalogItemsRequest) (*catalogv1.ListInventoryCatalogItemsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ListInventoryCatalogItems(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, "")
	}
	return resp, nil
}

func (c *client) GetAdminProduct(ctx context.Context, userID, id string) (*catalogv1.GetProductResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.GetAdminProduct(ctx, &catalogv1.GetAdminProductRequest{
		Id:     id,
		UserId: userID,
	})
	if err != nil {
		return nil, classifyCatalogError(err, id)
	}
	return resp.GetProduct(), nil
}

func (c *client) CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.GetProductResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.CreateProduct(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetId())
	}
	return resp.GetProduct(), nil
}

func (c *client) UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.GetProductResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.UpdateProduct(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetId())
	}
	return resp.GetProduct(), nil
}

func (c *client) CreateProductMediaUpload(ctx context.Context, req *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.CreateProductMediaUpload(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, "")
	}
	return resp, nil
}

func classifyCatalogError(err error, id string) error {
	switch catalogErrorReason(err) {
	case "PRODUCT_PERMISSION_DENIED":
		return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
	case "AUTHORIZATION_UNAVAILABLE":
		return fmt.Errorf("%w: %v", ErrAuthorizationUnavailable, err)
	case "PRODUCT_ID_EXISTS":
		return fmt.Errorf("%w: %v", ErrProductExists, err)
	case "SKU_ALREADY_EXISTS":
		return fmt.Errorf("%w: %v", ErrSkuConflict, err)
	case "MEDIA_ALREADY_ATTACHED":
		return fmt.Errorf("%w: %v", ErrMediaAlreadyAttached, err)
	case "MEDIA_STORAGE_UNAVAILABLE":
		return fmt.Errorf("%w: %v", ErrMediaStorageUnavailable, err)
	}
	if status.Code(err) == codes.NotFound {
		if id != "" {
			return fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		return fmt.Errorf("%w: %v", ErrProductNotFound, err)
	}
	return err
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
