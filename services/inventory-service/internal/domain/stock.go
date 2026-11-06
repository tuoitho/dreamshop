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

// ErrReservationAlreadyReleased is returned when an order reservation has
// already transitioned to RELEASED (including tombstone records).
var ErrReservationAlreadyReleased = errors.New("stock reservation already released")

// VariantHasReservedStockError is returned when attempting to delete stock rows
// for one or more SKUs while at least one SKU currently holds reserved > 0.
type VariantHasReservedStockError struct {
	SkuID string
}

func (e *VariantHasReservedStockError) Error() string {
	return fmt.Sprintf("variant sku %q currently has reserved stock", e.SkuID)
}

// ReservationLine describes a SKU quantity requested for an order reservation.
type ReservationLine struct {
	SkuID     string
	ProductID string
	Quantity  int32
}

// InsufficientStockError reports the first SKU that lacked sufficient available units.
type InsufficientStockError struct {
	SkuID     string
	ProductID string
	Available int32
	Requested int32
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for sku %q: available=%d requested=%d", e.SkuID, e.Available, e.Requested)
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
	ReserveStock(ctx context.Context, orderID string, items []ReservationLine, expiresAt time.Time) (string, error)
	ConfirmReservation(ctx context.Context, orderID string) (string, error)
	ReleaseReservation(ctx context.Context, orderID, reason string) (string, error)
}
