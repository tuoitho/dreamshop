package grpc

import (
	"context"
	"errors"
	"strings"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetStock returns physical, reserved, and available stock for a single product.
func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	st, err := s.repo.Get(ctx, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock %q: %v", productID, err)
	}
	return &inventoryv1.GetStockResponse{Stock: stockToProto(st)}, nil
}

// BatchGetStock returns stock records for up to 200 product IDs.
func (s *Server) BatchGetStock(ctx context.Context, req *inventoryv1.BatchGetStockRequest) (*inventoryv1.BatchGetStockResponse, error) {
	if len(req.GetProductIds()) > maxBatchProductIDs {
		return nil, status.Errorf(codes.InvalidArgument, "product_ids must not exceed %d entries", maxBatchProductIDs)
	}
	cleanIDs := make([]string, 0, len(req.GetProductIds()))
	for _, raw := range req.GetProductIds() {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, status.Error(codes.InvalidArgument, "product_ids entries must not be blank")
		}
		cleanIDs = append(cleanIDs, id)
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	stocks, err := s.repo.BatchGet(ctx, cleanIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch get stock: %v", err)
	}
	items := make([]*inventoryv1.StockItem, 0, len(stocks))
	for _, st := range stocks {
		items = append(items, stockToProto(st))
	}
	return &inventoryv1.BatchGetStockResponse{Items: items}, nil
}

// ListStocks returns all persisted stock rows.
func (s *Server) ListStocks(ctx context.Context, req *inventoryv1.ListStocksRequest) (*inventoryv1.ListStocksResponse, error) {
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	stocks, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list stocks: %v", err)
	}
	items := make([]*inventoryv1.StockItem, 0, len(stocks))
	for _, st := range stocks {
		items = append(items, stockToProto(st))
	}
	return &inventoryv1.ListStocksResponse{Items: items}, nil
}

// SetStock atomically sets physical quantity for a product.
func (s *Server) SetStock(ctx context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.SetStockResponse, error) {
	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	if req.GetQuantity() < 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be non-negative")
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryUpdatePermission); err != nil {
		return nil, err
	}

	st, err := s.repo.SetQuantity(ctx, productID, req.GetQuantity())
	if err != nil {
		if errors.Is(err, domain.ErrStockBelowReserved) {
			return nil, statusWithReason(codes.FailedPrecondition, "quantity cannot be less than reserved stock", reasonStockBelowReserved)
		}
		return nil, status.Errorf(codes.Internal, "set stock %q: %v", productID, err)
	}
	return &inventoryv1.SetStockResponse{Stock: stockToProto(st)}, nil
}

// AdjustStock atomically adds or subtracts delta from physical stock.
func (s *Server) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}
	if req.GetDelta() == 0 {
		return nil, status.Error(codes.InvalidArgument, "delta must be non-zero")
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryUpdatePermission); err != nil {
		return nil, err
	}

	st, err := s.repo.AdjustQuantity(ctx, productID, req.GetDelta())
	if err != nil {
		if errors.Is(err, domain.ErrStockBelowReserved) {
			return nil, statusWithReason(codes.FailedPrecondition, "resulting quantity cannot be less than reserved stock", reasonStockBelowReserved)
		}
		return nil, status.Errorf(codes.Internal, "adjust stock %q: %v", productID, err)
	}
	return &inventoryv1.AdjustStockResponse{Stock: stockToProto(st)}, nil
}

func stockToProto(st domain.Stock) *inventoryv1.StockItem {
	var updatedAt string
	if !st.UpdatedAt.IsZero() {
		updatedAt = st.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return &inventoryv1.StockItem{
		ProductId:         st.ProductID,
		Quantity:          st.Quantity,
		Reserved:          st.Reserved,
		AvailableQuantity: st.AvailableQuantity(),
		UpdatedAt:         updatedAt,
	}
}
