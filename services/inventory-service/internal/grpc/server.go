// Package grpc implements the inventoryv1.InventoryService gRPC surface for
// inventory-service.
package grpc

import (
	"context"
	"strings"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements inventoryv1.InventoryServiceServer over a domain.Repository.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	repo domain.Repository
}

// NewServer creates an InventoryService gRPC server backed by repo.
func NewServer(repo domain.Repository) *Server {
	return &Server{repo: repo}
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
