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

// Get loads stock for a SKU. A missing row is an intentional zero-stock
// result because availability is zero until Inventory has a local stock row.
func (s *Store) Get(ctx context.Context, skuID string) (domain.Stock, error) {
	var stock domain.Stock
	err := s.pool.QueryRow(ctx,
		"SELECT sku_id, product_id, quantity, reserved, updated_at FROM stock WHERE sku_id = $1",
		skuID,
	).Scan(&stock.SkuID, &stock.ProductID, &stock.Quantity, &stock.Reserved, &stock.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{SkuID: skuID}, nil
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("get stock %q: %w", skuID, err)
	}
	return stock, nil
}

// BatchGet returns stock entries matching the requested skuIDs in the same order,
// filling zero-stock entries for SKUs without a persisted stock row.
func (s *Store) BatchGet(ctx context.Context, skuIDs []string) ([]domain.Stock, error) {
	if len(skuIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		"SELECT sku_id, product_id, quantity, reserved, updated_at FROM stock WHERE sku_id = ANY($1)",
		skuIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("batch get stock: %w", err)
	}
	defer rows.Close()

	bySku := make(map[string]domain.Stock, len(skuIDs))
	for rows.Next() {
		var st domain.Stock
		if err := rows.Scan(&st.SkuID, &st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan batch stock row: %w", err)
		}
		bySku[st.SkuID] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch stock rows: %w", err)
	}

	out := make([]domain.Stock, len(skuIDs))
	for i, skuID := range skuIDs {
		if st, ok := bySku[skuID]; ok {
			out[i] = st
		} else {
			out[i] = domain.Stock{SkuID: skuID}
		}
	}
	return out, nil
}

// List returns all persisted stock rows ordered by updated_at DESC, sku_id ASC.
func (s *Store) List(ctx context.Context) ([]domain.Stock, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT sku_id, product_id, quantity, reserved, updated_at FROM stock ORDER BY updated_at DESC, sku_id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list stocks: %w", err)
	}
	defer rows.Close()

	var out []domain.Stock
	for rows.Next() {
		var st domain.Stock
		if err := rows.Scan(&st.SkuID, &st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan stock row: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// SetQuantity atomically upserts physical quantity for a SKU provided quantity >= reserved.
func (s *Store) SetQuantity(ctx context.Context, skuID, productID string, quantity int32) (domain.Stock, error) {
	if quantity < 0 {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	var st domain.Stock
	err := s.pool.QueryRow(ctx, `
		INSERT INTO stock (sku_id, product_id, quantity, reserved, updated_at)
		VALUES ($1, $2, $3, 0, now())
		ON CONFLICT (sku_id) DO UPDATE
		SET product_id = EXCLUDED.product_id,
		    quantity = EXCLUDED.quantity,
		    updated_at = now()
		WHERE stock.reserved <= EXCLUDED.quantity
		RETURNING sku_id, product_id, quantity, reserved, updated_at
	`, skuID, productID, quantity).Scan(&st.SkuID, &st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("set stock quantity %q: %w", skuID, err)
	}
	return st, nil
}

// AdjustQuantity atomically increments or decrements physical stock for a SKU without dropping below reserved.
func (s *Store) AdjustQuantity(ctx context.Context, skuID, productID string, delta int32) (domain.Stock, error) {
	var st domain.Stock
	if delta > 0 {
		err := s.pool.QueryRow(ctx, `
			INSERT INTO stock (sku_id, product_id, quantity, reserved, updated_at)
			VALUES ($1, $2, $3, 0, now())
			ON CONFLICT (sku_id) DO UPDATE
			SET product_id = EXCLUDED.product_id,
			    quantity = stock.quantity + $3,
			    updated_at = now()
			RETURNING sku_id, product_id, quantity, reserved, updated_at
		`, skuID, productID, delta).Scan(&st.SkuID, &st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
		if err != nil {
			return domain.Stock{}, fmt.Errorf("increment stock %q: %w", skuID, err)
		}
		return st, nil
	}

	err := s.pool.QueryRow(ctx, `
		UPDATE stock
		SET product_id = $2,
		    quantity = quantity + $3,
		    updated_at = now()
		WHERE sku_id = $1 AND quantity + $3 >= reserved
		RETURNING sku_id, product_id, quantity, reserved, updated_at
	`, skuID, productID, delta).Scan(&st.SkuID, &st.ProductID, &st.Quantity, &st.Reserved, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	if err != nil {
		return domain.Stock{}, fmt.Errorf("decrement stock %q: %w", skuID, err)
	}
	return st, nil
}

// DeleteUnreserved locks matching SKU rows in ascending sku_id order, rejects if
// any row currently holds reserved > 0, and deletes the rows when dryRun is false.
func (s *Store) DeleteUnreserved(ctx context.Context, skuIDs []string, dryRun bool) error {
	if len(skuIDs) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete stocks tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT sku_id, reserved
		FROM stock
		WHERE sku_id = ANY($1)
		ORDER BY sku_id ASC
		FOR UPDATE
	`, skuIDs)
	if err != nil {
		return fmt.Errorf("lock stocks for delete: %w", err)
	}
	for rows.Next() {
		var skuID string
		var reserved int32
		if err := rows.Scan(&skuID, &reserved); err != nil {
			rows.Close()
			return fmt.Errorf("scan locked stock: %w", err)
		}
		if reserved > 0 {
			rows.Close()
			return &domain.VariantHasReservedStockError{SkuID: skuID}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate locked stocks: %w", err)
	}
	rows.Close()

	if !dryRun {
		if _, err := tx.Exec(ctx, "DELETE FROM stock WHERE sku_id = ANY($1)", skuIDs); err != nil {
			return fmt.Errorf("delete stocks: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete stocks tx: %w", err)
	}
	return nil
}
