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
	ErrReviewNotFound           = errors.New("catalog: review not found")
	ErrReviewNotEligible        = errors.New("catalog: order not eligible for review")
	ErrReviewAlreadyExists      = errors.New("catalog: review already exists")
	ErrReviewForbidden          = errors.New("catalog: review forbidden")
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
	ListProductReviews(ctx context.Context, req *catalogv1.ListProductReviewsRequest) (*catalogv1.ListProductReviewsResponse, error)
	GetReviewEligibility(ctx context.Context, req *catalogv1.GetReviewEligibilityRequest) (*catalogv1.GetReviewEligibilityResponse, error)
	CreateProductReview(ctx context.Context, req *catalogv1.CreateProductReviewRequest) (*catalogv1.CreateProductReviewResponse, error)
	UpdateProductReview(ctx context.Context, req *catalogv1.UpdateProductReviewRequest) (*catalogv1.UpdateProductReviewResponse, error)
	DeleteProductReview(ctx context.Context, req *catalogv1.DeleteProductReviewRequest) (*catalogv1.DeleteProductReviewResponse, error)
	CreateReviewMediaUpload(ctx context.Context, req *catalogv1.CreateReviewMediaUploadRequest) (*catalogv1.CreateReviewMediaUploadResponse, error)
	ListAdminReviews(ctx context.Context, req *catalogv1.ListAdminReviewsRequest) (*catalogv1.ListAdminReviewsResponse, error)
	SetReviewVisibility(ctx context.Context, req *catalogv1.SetReviewVisibilityRequest) (*catalogv1.SetReviewVisibilityResponse, error)
	ReplyProductReview(ctx context.Context, req *catalogv1.ReplyProductReviewRequest) (*catalogv1.ReplyProductReviewResponse, error)
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

func (c *client) ListProductReviews(ctx context.Context, req *catalogv1.ListProductReviewsRequest) (*catalogv1.ListProductReviewsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ListProductReviews(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetProductId())
	}
	return resp, nil
}

func (c *client) GetReviewEligibility(ctx context.Context, req *catalogv1.GetReviewEligibilityRequest) (*catalogv1.GetReviewEligibilityResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.GetReviewEligibility(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetProductId())
	}
	return resp, nil
}

func (c *client) CreateProductReview(ctx context.Context, req *catalogv1.CreateProductReviewRequest) (*catalogv1.CreateProductReviewResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.CreateProductReview(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetProductId())
	}
	return resp, nil
}

func (c *client) UpdateProductReview(ctx context.Context, req *catalogv1.UpdateProductReviewRequest) (*catalogv1.UpdateProductReviewResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.UpdateProductReview(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetReviewId())
	}
	return resp, nil
}

func (c *client) DeleteProductReview(ctx context.Context, req *catalogv1.DeleteProductReviewRequest) (*catalogv1.DeleteProductReviewResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.DeleteProductReview(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetReviewId())
	}
	return resp, nil
}

func (c *client) CreateReviewMediaUpload(ctx context.Context, req *catalogv1.CreateReviewMediaUploadRequest) (*catalogv1.CreateReviewMediaUploadResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.CreateReviewMediaUpload(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, "")
	}
	return resp, nil
}

func (c *client) ListAdminReviews(ctx context.Context, req *catalogv1.ListAdminReviewsRequest) (*catalogv1.ListAdminReviewsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ListAdminReviews(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, "")
	}
	return resp, nil
}

func (c *client) SetReviewVisibility(ctx context.Context, req *catalogv1.SetReviewVisibilityRequest) (*catalogv1.SetReviewVisibilityResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.SetReviewVisibility(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetReviewId())
	}
	return resp, nil
}

func (c *client) ReplyProductReview(ctx context.Context, req *catalogv1.ReplyProductReviewRequest) (*catalogv1.ReplyProductReviewResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.svc.ReplyProductReview(ctx, req)
	if err != nil {
		return nil, classifyCatalogError(err, req.GetReviewId())
	}
	return resp, nil
}

func classifyCatalogError(err error, id string) error {
	switch catalogErrorReason(err) {
	case "PRODUCT_PERMISSION_DENIED", "REVIEW_PERMISSION_DENIED":
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
	case "REVIEW_NOT_FOUND":
		return fmt.Errorf("%w: %v", ErrReviewNotFound, err)
	case "REVIEW_NOT_ELIGIBLE":
		return fmt.Errorf("%w: %v", ErrReviewNotEligible, err)
	case "REVIEW_ALREADY_EXISTS":
		return fmt.Errorf("%w: %v", ErrReviewAlreadyExists, err)
	case "REVIEW_FORBIDDEN":
		return fmt.Errorf("%w: %v", ErrReviewForbidden, err)
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
