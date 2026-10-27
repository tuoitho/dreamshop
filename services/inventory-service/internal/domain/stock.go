package domain

import (
	"context"
	"errors"
	"time"
)

// ErrStockBelowReserved is returned when a stock mutation would reduce physical
// quantity below currently reserved stock (or below zero).
var ErrStockBelowReserved = errors.New("stock quantity cannot be less than reserved units")

type Stock struct {
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
	Get(ctx context.Context, productID string) (Stock, error)
	BatchGet(ctx context.Context, productIDs []string) ([]Stock, error)
	ListAll(ctx context.Context) ([]Stock, error)
	SetQuantity(ctx context.Context, productID string, quantity int32) (Stock, error)
	AdjustQuantity(ctx context.Context, productID string, delta int32) (Stock, error)
}
