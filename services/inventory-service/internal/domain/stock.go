package domain

import "context"

type Stock struct {
	ProductID string
	Quantity  int32
	Reserved  int32
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
}
