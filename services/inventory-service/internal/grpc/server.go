// Package grpc implements the inventoryv1.InventoryService gRPC surface for
// inventory-service.
package grpc

import (
	"context"
	"log/slog"
	"strings"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/domain"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	errorDomain                      = "ecommerce.inventory.v1"
	inventoryViewPermission          = "inventory.view"
	inventoryUpdatePermission        = "inventory.update"
	reasonInventoryPermissionDenied  = "INVENTORY_PERMISSION_DENIED"
	reasonAuthorizationUnavailable   = "AUTHORIZATION_UNAVAILABLE"
	reasonStockBelowReserved         = "STOCK_BELOW_RESERVED"
	traceMetadataKey                 = "x-trace-id"
	maxTraceIDLength                 = 64
	maxBatchProductIDs               = 200
)

type authorizationClient interface {
	CheckPermission(context.Context, string, string) (bool, error)
}

// Server implements inventoryv1.InventoryServiceServer over a domain.Repository.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	repo          domain.Repository
	authorization authorizationClient
}

// NewServer creates an InventoryService gRPC server backed by repo (public read RPCs only).
func NewServer(repo domain.Repository) *Server {
	return &Server{repo: repo}
}

// NewServerWithAuth creates an InventoryService gRPC server with Identity authorization wired.
func NewServerWithAuth(repo domain.Repository, authorization authorizationClient) *Server {
	return &Server{repo: repo, authorization: authorization}
}

// GetAvailability returns the currently available quantity for a product.
func (s *Server) GetAvailability(ctx context.Context, req *inventoryv1.GetAvailabilityRequest) (*inventoryv1.GetAvailabilityResponse, error) {
	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}

	stock, err := s.repo.Get(ctx, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get availability for %q: %v", productID, err)
	}

	return &inventoryv1.GetAvailabilityResponse{
		ProductId:         productID,
		AvailableQuantity: stock.AvailableQuantity(),
	}, nil
}

func (s *Server) authorize(ctx context.Context, rawUserID, permission string) error {
	userID := strings.TrimSpace(rawUserID)
	if userID == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	if s.authorization == nil {
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}

	authCtx, traceID := withAuthorizationTrace(ctx)
	allowed, err := s.authorization.CheckPermission(authCtx, userID, permission)
	if err != nil {
		if ctx.Err() != nil {
			return status.FromContextError(ctx.Err()).Err()
		}
		slog.WarnContext(ctx, "inventory authorization unavailable", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.Unavailable, "authorization service is unavailable", reasonAuthorizationUnavailable)
	}
	if !allowed {
		slog.DebugContext(ctx, "inventory authorization denied", "user_id", userID,
			"permission", permission, "trace_id", traceID)
		return statusWithReason(codes.PermissionDenied, "inventory permission denied", reasonInventoryPermissionDenied)
	}
	return nil
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
		ch := candidate[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func statusWithReason(code codes.Code, message, reason string) error {
	base := status.New(code, message)
	withDetails, err := base.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: errorDomain})
	if err != nil {
		return base.Err()
	}
	return withDetails.Err()
}
