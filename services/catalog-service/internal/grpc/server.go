// Package grpc implements the catalogv1.CatalogService gRPC surface for
// catalog-service.
package grpc

import (
	"context"
	"errors"
	"strings"

	"ecommerce.local/catalog-service/internal/domain"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	moneyCurrency  = "VND"
	errorDomain    = "ecommerce.catalog.v1"
	reasonNotFound = "PRODUCT_NOT_FOUND"
)

// Server implements catalogv1.CatalogServiceServer over a domain.Repository.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	repo domain.Repository
}

// NewServer creates a CatalogService gRPC server backed by repo.
func NewServer(repo domain.Repository) *Server {
	return &Server{repo: repo}
}

// GetProduct returns the full product detail.
func (s *Server) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	product, err := s.repo.GetByID(ctx, req.GetId())
	if errors.Is(err, domain.ErrProductNotFound) {
		return nil, notFoundError(req.GetId())
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get product %q: %v", req.GetId(), err)
	}

	return productToProto(product), nil
}

// SearchProducts returns paginated product summaries matching the filter.
func (s *Server) SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
	if req.GetRatingMin() < 0 {
		return nil, status.Error(codes.InvalidArgument, "rating_min must not be negative")
	}
	if req.GetPriceMin() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_min must not be negative")
	}

	filter := domain.SearchFilter{
		Query:      req.GetQuery(),
		CategoryID: req.GetCategoryId(),
		Sort:       req.GetSort(),
		PriceMin:   req.GetPriceMin(),
		PriceMax:   req.GetPriceMax(),
		OnSale:     req.GetOnSale(),
		RatingMin:  req.GetRatingMin(),
		Page:       req.GetPage(),
		PageSize:   req.GetPageSize(),
	}

	result, err := s.repo.Search(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search products: %v", err)
	}

	return searchResultToProto(result), nil
}

// SoftDeleteProduct marks a product as deleted or restores it.
func (s *Server) SoftDeleteProduct(ctx context.Context, req *catalogv1.SoftDeleteProductRequest) (*catalogv1.SoftDeleteProductResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if err := s.repo.SoftDelete(ctx, id, req.GetDeleted()); err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			return nil, notFoundError(id)
		}
		return nil, status.Errorf(codes.Internal, "set product deleted %q: %v", id, err)
	}
	return &catalogv1.SoftDeleteProductResponse{Id: id, Deleted: req.GetDeleted()}, nil
}

func productToProto(p *domain.Product) *catalogv1.GetProductResponse {
	resp := &catalogv1.GetProductResponse{
		Id:               p.ID,
		Name:             p.Name,
		ShortDescription: p.ShortDescription,
		Description:      p.Description,
		Brand:            p.Brand,
		CategoryId:       p.CategoryID,
		CategoryName:     p.CategoryName,
		Price:            moneyToProto(p.PriceMinor),
		OriginalPrice:    moneyToProto(p.OriginalPriceMinor),
		Images:           p.Images,
		Rating:           p.Rating,
		ReviewCount:      p.ReviewCount,
		SoldCount:        p.SoldCount,
		IsNew:            p.IsNew,
		Badge:            p.Badge,
		Featured:         p.Featured,
		Emoji:            p.Emoji,
		ImageHue:         p.ImageHue,
		Details:          p.Details,
		Deleted:          p.Deleted,
	}
	for _, v := range p.Variants {
		resp.Variants = append(resp.Variants, productVariantToProto(v))
	}
	return resp
}

func productVariantToProto(v domain.ProductVariant) *catalogv1.ProductVariant {
	return &catalogv1.ProductVariant{Label: v.Label, ColorHex: v.ColorHex}
}

func searchResultToProto(r *domain.SearchResult) *catalogv1.SearchProductsResponse {
	resp := &catalogv1.SearchProductsResponse{
		Page:       r.Page,
		PageSize:   r.PageSize,
		TotalItems: r.TotalItems,
		TotalPages: r.TotalPages,
	}
	for _, s := range r.Items {
		resp.Items = append(resp.Items, summaryToProto(s))
	}
	return resp
}

func summaryToProto(s *domain.Summary) *catalogv1.ProductSummary {
	return &catalogv1.ProductSummary{
		Id:            s.ID,
		Name:          s.Name,
		Brand:         s.Brand,
		CategoryId:    s.CategoryID,
		CategoryName:  s.CategoryName,
		Price:         moneyToProto(s.PriceMinor),
		OriginalPrice: moneyToProto(s.OriginalPriceMinor),
		Images:        s.Images,
		Rating:        s.Rating,
		ReviewCount:   s.ReviewCount,
		SoldCount:     s.SoldCount,
		IsNew:         s.IsNew,
		Badge:         s.Badge,
		Featured:      s.Featured,
		Emoji:         s.Emoji,
		ImageHue:      s.ImageHue,
	}
}

func moneyToProto(amountMinor int64) *commonv1.Money {
	return &commonv1.Money{
		Currency:    moneyCurrency,
		AmountMinor: amountMinor,
	}
}

// notFoundError builds a codes.NotFound status carrying a stable
// google.rpc.ErrorInfo reason so clients can branch on PRODUCT_NOT_FOUND
// instead of human-readable text.
func notFoundError(id string) error {
	st := status.New(codes.NotFound, "product not found")
	st, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: reasonNotFound,
		Domain: errorDomain,
		Metadata: map[string]string{
			"product_id": id,
		},
	})
	if err != nil {
		// Without detail support there is nothing to annotate; keep the
		// original NotFound status rather than degrading the code.
		return status.New(codes.NotFound, "product not found").Err()
	}
	return st.Err()
}
