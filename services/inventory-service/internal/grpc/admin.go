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

// GetStock returns physical, reserved, and available stock for a single SKU.
func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	skuID := strings.TrimSpace(req.GetSkuId())
	if skuID == "" {
		return nil, status.Error(codes.InvalidArgument, "sku_id is required")
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	st, err := s.repo.Get(ctx, skuID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock %q: %v", skuID, err)
	}
	return &inventoryv1.GetStockResponse{Stock: stockToProto(st)}, nil
}

// BatchGetStock returns stock records for requested SKU IDs in request order.
func (s *Server) BatchGetStock(ctx context.Context, req *inventoryv1.BatchGetStockRequest) (*inventoryv1.BatchGetStockResponse, error) {
	cleanIDs, err := sanitizeBatchSkuIDs(req.GetSkuIds())
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}
	if len(cleanIDs) == 0 {
		return &inventoryv1.BatchGetStockResponse{}, nil
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

// ListStocks returns all persisted SKU stock rows.
func (s *Server) ListStocks(ctx context.Context, req *inventoryv1.ListStocksRequest) (*inventoryv1.ListStocksResponse, error) {
	if err := s.authorize(ctx, req.GetUserId(), inventoryViewPermission); err != nil {
		return nil, err
	}

	stocks, err := s.repo.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list stocks: %v", err)
	}
	items := make([]*inventoryv1.StockItem, 0, len(stocks))
	for _, st := range stocks {
		items = append(items, stockToProto(st))
	}
	return &inventoryv1.ListStocksResponse{Items: items}, nil
}

// SetStock atomically sets physical quantity for a SKU.
func (s *Server) SetStock(ctx context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.SetStockResponse, error) {
	skuID := strings.TrimSpace(req.GetSkuId())
	if skuID == "" {
		return nil, status.Error(codes.InvalidArgument, "sku_id is required")
	}
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

	st, err := s.repo.SetQuantity(ctx, skuID, productID, req.GetQuantity())
	if err != nil {
		if errors.Is(err, domain.ErrStockBelowReserved) {
			return nil, statusWithReason(codes.FailedPrecondition, "quantity cannot be less than reserved stock", reasonStockBelowReserved)
		}
		return nil, status.Errorf(codes.Internal, "set stock %q: %v", skuID, err)
	}
	return &inventoryv1.SetStockResponse{Stock: stockToProto(st)}, nil
}

// AdjustStock atomically adds or subtracts delta from physical stock for a SKU.
func (s *Server) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	skuID := strings.TrimSpace(req.GetSkuId())
	if skuID == "" {
		return nil, status.Error(codes.InvalidArgument, "sku_id is required")
	}
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

	st, err := s.repo.AdjustQuantity(ctx, skuID, productID, req.GetDelta())
	if err != nil {
		if errors.Is(err, domain.ErrStockBelowReserved) {
			return nil, statusWithReason(codes.FailedPrecondition, "resulting quantity cannot be less than reserved stock", reasonStockBelowReserved)
		}
		return nil, status.Errorf(codes.Internal, "adjust stock %q: %v", skuID, err)
	}
	return &inventoryv1.AdjustStockResponse{Stock: stockToProto(st)}, nil
}

// DeleteStocks checks or deletes unreserved SKU stock rows after verifying product.update or inventory.update.
func (s *Server) DeleteStocks(ctx context.Context, req *inventoryv1.DeleteStocksRequest) (*inventoryv1.DeleteStocksResponse, error) {
	cleanIDs, err := sanitizeBatchSkuIDs(req.GetSkuIds())
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAny(ctx, req.GetUserId(), productUpdatePermission, inventoryUpdatePermission); err != nil {
		return nil, err
	}
	if len(cleanIDs) == 0 {
		return &inventoryv1.DeleteStocksResponse{}, nil
	}

	if err := s.repo.DeleteUnreserved(ctx, cleanIDs, req.GetDryRun()); err != nil {
		var resErr *domain.VariantHasReservedStockError
		if errors.As(err, &resErr) {
			return nil, statusWithReasonAndMetadata(
				codes.FailedPrecondition,
				"cannot delete variant with reserved stock",
				reasonVariantHasReservedStock,
				map[string]string{"sku_id": resErr.SkuID},
			)
		}
		return nil, status.Errorf(codes.Internal, "delete stocks: %v", err)
	}
	return &inventoryv1.DeleteStocksResponse{}, nil
}

func stockToProto(st domain.Stock) *inventoryv1.StockItem {
	var updatedAt string
	if !st.UpdatedAt.IsZero() {
		updatedAt = st.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return &inventoryv1.StockItem{
		SkuId:             st.SkuID,
		ProductId:         st.ProductID,
		Quantity:          st.Quantity,
		Reserved:          st.Reserved,
		AvailableQuantity: st.AvailableQuantity(),
		UpdatedAt:         updatedAt,
	}
}
