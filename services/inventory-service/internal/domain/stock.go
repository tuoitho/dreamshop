package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrStockBelowReserved is returned when a stock mutation would reduce physical
// quantity below currently reserved stock (or below zero).
var ErrStockBelowReserved = errors.New("stock quantity cannot be less than reserved units")

// VariantHasReservedStockError is returned when attempting to delete stock rows
// for one or more SKUs while at least one SKU currently holds reserved > 0.
type VariantHasReservedStockError struct {
	SkuID string
}

func (e *VariantHasReservedStockError) Error() string {
	return fmt.Sprintf("variant sku %q currently has reserved stock", e.SkuID)
}

type Stock struct {
	SkuID     string
	ProductID string
	Quantity  int32
	Reserved  int32
	UpdatedAt time.Time
}

func (s Stock) AvailableQuantity() int32 {
	available := s.Quantity - s.Reserved
	if available < 0 {
		return 0
	}
	return available
}

type Repository interface {
	Get(ctx context.Context, skuID string) (Stock, error)
	BatchGet(ctx context.Context, skuIDs []string) ([]Stock, error)
	List(ctx context.Context) ([]Stock, error)
	SetQuantity(ctx context.Context, skuID, productID string, quantity int32) (Stock, error)
	AdjustQuantity(ctx context.Context, skuID, productID string, delta int32) (Stock, error)
	DeleteUnreserved(ctx context.Context, skuIDs []string, dryRun bool) error
}
