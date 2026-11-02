package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type seedStock struct {
	skuID     string
	productID string
	quantity  int32
}

var seedStocks = []seedStock{
	{skuID: "dt-1-v1", productID: "dt-1", quantity: 8},
	{skuID: "dt-1-v2", productID: "dt-1", quantity: 3},
	{skuID: "dt-1-v3", productID: "dt-1", quantity: 0},
	{skuID: "dt-2-v1", productID: "dt-2", quantity: 0},
	{skuID: "dt-2-v2", productID: "dt-2", quantity: 0},
	{skuID: "lt-1-v1", productID: "lt-1", quantity: 4},
	{skuID: "lt-1-v2", productID: "lt-1", quantity: 1},
	{skuID: "lt-2-v1", productID: "lt-2", quantity: 5},
	{skuID: "lt-2-v2", productID: "lt-2", quantity: 2},
	{skuID: "tn-1-v1", productID: "tn-1", quantity: 8},
	{skuID: "tn-1-v2", productID: "tn-1", quantity: 6},
	{skuID: "tn-1-v3", productID: "tn-1", quantity: 4},
	{skuID: "tn-2-v1", productID: "tn-2", quantity: 6},
	{skuID: "tn-2-v2", productID: "tn-2", quantity: 5},
	{skuID: "dh-1-v1", productID: "dh-1", quantity: 4},
	{skuID: "dh-1-v2", productID: "dh-1", quantity: 7},
	{skuID: "dh-2-v1", productID: "dh-2", quantity: 2},
	{skuID: "dh-2-v2", productID: "dh-2", quantity: 3},
	{skuID: "tt-1-v1", productID: "tt-1", quantity: 12},
	{skuID: "tt-1-v2", productID: "tt-1", quantity: 9},
	{skuID: "tt-1-v3", productID: "tt-1", quantity: 5},
	{skuID: "tt-2-v1", productID: "tt-2", quantity: 7},
	{skuID: "tt-2-v2", productID: "tt-2", quantity: 8},
	{skuID: "gd-1-v1", productID: "gd-1", quantity: 9},
	{skuID: "gd-1-v2", productID: "gd-1", quantity: 6},
	{skuID: "gd-2-v1", productID: "gd-2", quantity: 3},
	{skuID: "ld-1-v1", productID: "ld-1", quantity: 10},
	{skuID: "ld-2-v1", productID: "ld-2", quantity: 11},
	{skuID: "sach-1-v1", productID: "sach-1", quantity: 15},
	{skuID: "sach-1-v2", productID: "sach-1", quantity: 12},
	{skuID: "sach-2-v1", productID: "sach-2", quantity: 13},
}

// Seed adds deterministic demo SKU stock without overwriting operator-managed
// rows, making startup reruns safe.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, stock := range seedStocks {
		if err := insertStock(ctx, tx, stock); err != nil {
			return fmt.Errorf("seed stock %q: %w", stock.skuID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func insertStock(ctx context.Context, tx pgx.Tx, stock seedStock) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO stock (sku_id, product_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (sku_id) DO NOTHING`, stock.skuID, stock.productID, stock.quantity)
	return err
}
