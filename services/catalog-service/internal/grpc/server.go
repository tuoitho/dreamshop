// Package grpc implements the catalogv1.CatalogService gRPC surface for
// catalog-service.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
	"ecommerce.local/catalog-service/internal/media"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	moneyCurrency                  = "VND"
	errorDomain                    = "ecommerce.catalog.v1"
	productDeletePermission        = "product.delete"
	productViewPermission          = "product.view"
	productCreatePermission        = "product.create"
	productUpdatePermission        = "product.update"
	inventoryViewPermission        = "inventory.view"
	reasonNotFound                 = "PRODUCT_NOT_FOUND"
	reasonProductExists            = "PRODUCT_ID_EXISTS"
	reasonMediaAlreadyAttached     = "MEDIA_ALREADY_ATTACHED"
	reasonMediaStorageUnavailable  = "MEDIA_STORAGE_UNAVAILABLE"
	reasonProductPermissionDenied  = "PRODUCT_PERMISSION_DENIED"
	reasonAuthorizationUnavailable = "AUTHORIZATION_UNAVAILABLE"
	traceMetadataKey               = "x-trace-id"
	maxTraceIDLength               = 64
	defaultOrphanGrace             = 24 * time.Hour
	defaultPresignTTL              = 5 * time.Minute
)

type authorizationClient interface {
	CheckPermission(context.Context, string, string) (bool, error)
}

type batchAuthorizationClient interface {
	CheckAnyPermission(context.Context, string, ...string) (bool, error)
}

// MediaSigner presigns browser PUT uploads into the product-media bucket.
type MediaSigner interface {
	Bucket() string
	EnsureBucket(ctx context.Context) error
	PresignPut(objectKey, contentType string, sizeBytes int64, ttl time.Duration) (*media.SignedUpload, error)
}

// ServerOption configures optional dependencies on Server.
type ServerOption func(*Server)

// WithMediaSigner enables presigned product media uploads and sets lifecycle TTLs.
func WithMediaSigner(signer MediaSigner, presignTTL, orphanGrace time.Duration) ServerOption {
	return func(s *Server) {
		s.mediaSigner = signer
		if presignTTL > 0 {
			s.presignTTL = presignTTL
		}
		if orphanGrace > 0 {
			s.orphanGrace = orphanGrace
		}
	}
}

// Server implements catalogv1.CatalogServiceServer over a domain.Repository.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	repo          domain.Repository
	authorization authorizationClient
	mediaSigner   MediaSigner
	presignTTL    time.Duration
	orphanGrace   time.Duration
}

// NewServer creates a CatalogService gRPC server backed by repo and authorization.
func NewServer(repo domain.Repository, authorization authorizationClient, opts ...ServerOption) *Server {
	srv := &Server{
		repo:          repo,
		authorization: authorization,
		presignTTL:    defaultPresignTTL,
		orphanGrace:   defaultOrphanGrace,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(srv)
		}
	}
	return srv
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

// SoftDeleteProduct marks a product as deleted or restores it after verifying product.delete permission.
func (s *Server) SoftDeleteProduct(ctx context.Context, req *catalogv1.SoftDeleteProductRequest) (*catalogv1.SoftDeleteProductResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if err := s.authorize(ctx, req.GetUserId(), productDeletePermission); err != nil {
		return nil, err
	}

	if err := s.repo.SoftDelete(ctx, id, req.GetDeleted()); err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			return nil, notFoundError(id)
		}
		return nil, status.Errorf(codes.Internal, "set product deleted %q: %v", id, err)
	}
	return &catalogv1.SoftDeleteProductResponse{Id: id, Deleted: req.GetDeleted()}, nil
}

func withAuthorizationTrace(ctx context.Context) (context.Context, string) {
	incoming, _ := metadata.FromIncomingContext(ctx)
	values := incoming.Get(traceMetadataKey)
	if len(values) != 1 || !validTraceID(values[0]) {
		return ctx, ""
	}
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	outgoing = outgoing.Copy()
	outgoing.Set(traceMetadataKey, values[0])
	return metadata.NewOutgoingContext(ctx, outgoing), values[0]
}

func validTraceID(candidate string) bool {
	if len(candidate) == 0 || len(candidate) > maxTraceIDLength {
		return false
	}
	for i := range len(candidate) {
		value := candidate[i]
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '-' || value == '_' || value == '.' {
			continue
		}
		return false
	}
	return true
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
		Videos:           p.Videos,
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

func statusWithReason(code codes.Code, message, reason string) error {
	base := status.New(code, message)
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: errorDomain})
	if err != nil {
		return base.Err()
	}
	return withDetails.Err()
}

func (s *Server) authorize(ctx context.Context, rawUserID, permission string) error {
	userID := strings.TrimSpace(rawUserID)
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.authorization == nil {
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	authorizationCtx, traceID := withAuthorizationTrace(ctx)
	allowed, err := s.authorization.CheckPermission(authorizationCtx, userID, permission)
	if err != nil {
		if ctx.Err() != nil {
			return status.FromContextError(ctx.Err()).Err()
		}
		slog.WarnContext(ctx, "catalog authorization unavailable", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	if !allowed {
		slog.DebugContext(ctx, "catalog authorization denied", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.PermissionDenied, "product permission denied", reasonProductPermissionDenied)
	}
	return nil
}

func (s *Server) authorizeAny(ctx context.Context, rawUserID string, permissions ...string) error {
	userID := strings.TrimSpace(rawUserID)
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.authorization == nil {
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	authorizationCtx, traceID := withAuthorizationTrace(ctx)

	if batchAuth, ok := s.authorization.(batchAuthorizationClient); ok {
		allowed, err := batchAuth.CheckAnyPermission(authorizationCtx, userID, permissions...)
		if err != nil {
			if ctx.Err() != nil {
				return status.FromContextError(ctx.Err()).Err()
			}
			slog.WarnContext(ctx, "catalog batch authorization unavailable", "user_id", userID, "trace_id", traceID)
			return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
		}
		if !allowed {
			return statusWithReason(codes.PermissionDenied, "product permission denied", reasonProductPermissionDenied)
		}
		return nil
	}

	for _, perm := range permissions {
		allowed, err := s.authorization.CheckPermission(authorizationCtx, userID, perm)
		if err != nil {
			if ctx.Err() != nil {
				return status.FromContextError(ctx.Err()).Err()
			}
			slog.WarnContext(ctx, "catalog authorization unavailable", "user_id", userID, "permission", perm, "trace_id", traceID)
			return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
		}
		if allowed {
			return nil
		}
	}
	return statusWithReason(codes.PermissionDenied, "product permission denied", reasonProductPermissionDenied)
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
		return status.New(codes.NotFound, "product not found").Err()
	}
	return st.Err()
}
