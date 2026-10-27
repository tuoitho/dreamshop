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
		"SELECT product_id, quantity, reserved, updated_at FROM stock WHERE product_id = $1",
		productID,
	).Scan(&stock.ProductID, &stock.Quantity, &stock.Reserved, &stock.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{ProductID: productID}, nil
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("get stock %q: %w", productID, err)
	}
	return stock, nil
}

// BatchGet returns stock entries matching the requested productIDs in the same order,
// filling zero-stock entries for products without a persisted stock row.
func (s *Store) BatchGet(ctx context.Context, productIDs []string) ([]domain.Stock, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		"SELECT product_id, quantity, reserved, updated_at FROM stock WHERE product_id = ANY($1)",
		productIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("batch get stock: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]domain.Stock, len(productIDs))
	for rows.Next() {
		var st domain.Stock
		if err := rows.Scan(&st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan batch stock row: %w", err)
		}
		byID[st.ProductID] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch stock rows: %w", err)
	}

	out := make([]domain.Stock, len(productIDs))
	for i, id := range productIDs {
		if st, ok := byID[id]; ok {
			out[i] = st
		} else {
			out[i] = domain.Stock{ProductID: id}
		}
	}
	return out, nil
}

// ListAll returns all persisted stock rows ordered by updated_at DESC, product_id ASC.
func (s *Store) ListAll(ctx context.Context) ([]domain.Stock, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT product_id, quantity, reserved, updated_at FROM stock ORDER BY updated_at DESC, product_id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list all stocks: %w", err)
	}
	defer rows.Close()

	var out []domain.Stock
	for rows.Next() {
		var st domain.Stock
		if err := rows.Scan(&st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan stock row: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// SetQuantity atomically upserts physical quantity provided quantity >= reserved.
func (s *Store) SetQuantity(ctx context.Context, productID string, quantity int32) (domain.Stock, error) {
	if quantity < 0 {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	var st domain.Stock
	err := s.pool.QueryRow(ctx, `
		INSERT INTO stock (product_id, quantity, reserved, updated_at)
		VALUES ($1, $2, 0, now())
		ON CONFLICT (product_id) DO UPDATE
		SET quantity = EXCLUDED.quantity, updated_at = now()
		WHERE stock.reserved <= EXCLUDED.quantity
		RETURNING product_id, quantity, reserved, updated_at
	`, productID, quantity).Scan(&st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("set stock quantity %q: %w", productID, err)
	}
	return st, nil
}

// AdjustQuantity atomically increments or decrements physical stock without dropping below reserved.
func (s *Store) AdjustQuantity(ctx context.Context, productID string, delta int32) (domain.Stock, error) {
	var st domain.Stock
	if delta > 0 {
		err := s.pool.QueryRow(ctx, `
			INSERT INTO stock (product_id, quantity, reserved, updated_at)
			VALUES ($1, $2, 0, now())
			ON CONFLICT (product_id) DO UPDATE
			SET quantity = stock.quantity + $2, updated_at = now()
			RETURNING product_id, quantity, reserved, updated_at
		`, productID, delta).Scan(&st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
		if err != nil {
			return domain.Stock{}, fmt.Errorf("increment stock %q: %w", productID, err)
		}
		return st, nil
	}

	err := s.pool.QueryRow(ctx, `
		UPDATE stock
		SET quantity = quantity + $2, updated_at = now()
		WHERE product_id = $1 AND quantity + $2 >= reserved
		RETURNING product_id, quantity, reserved, updated_at
	`, productID, delta).Scan(&st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("decrement stock %q: %w", productID, err)
	}
	return st, nil
}
