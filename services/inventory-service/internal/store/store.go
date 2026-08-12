package store

import (
	"context"
	"errors"
	"fmt"

	"ecommerce.local/inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements domain.Repository against PostgreSQL through pgxpool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Get loads stock for a product. A missing row is an intentional zero-stock
// result because availability is zero until Inventory has a local stock row.
func (s *Store) Get(ctx context.Context, productID string) (domain.Stock, error) {
	var stock domain.Stock
	err := s.pool.QueryRow(ctx,
		"SELECT product_id, quantity, reserved FROM stock WHERE product_id = $1",
		productID,
	).Scan(&stock.ProductID, &stock.Quantity, &stock.Reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{ProductID: productID}, nil
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("get stock %q: %w", productID, err)
	}
	return stock, nil
}
