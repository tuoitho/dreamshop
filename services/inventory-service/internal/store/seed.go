package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type seedStock struct {
	productID string
	quantity  int32
}

var seedStocks = []seedStock{
	{productID: "dt-1", quantity: 3},
	{productID: "dt-2", quantity: 0},
	{productID: "lt-1", quantity: 1},
	{productID: "lt-2", quantity: 5},
	{productID: "tn-1", quantity: 8},
	{productID: "tn-2", quantity: 6},
	{productID: "dh-1", quantity: 4},
	{productID: "dh-2", quantity: 2},
	{productID: "tt-1", quantity: 12},
	{productID: "tt-2", quantity: 7},
	{productID: "gd-1", quantity: 9},
	{productID: "gd-2", quantity: 3},
	{productID: "ld-1", quantity: 10},
	{productID: "ld-2", quantity: 11},
	{productID: "sach-1", quantity: 15},
	{productID: "sach-2", quantity: 13},
}

// Seed adds deterministic demo stock without overwriting operator-managed
// rows, making startup reruns safe.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, stock := range seedStocks {
		if err := insertStock(ctx, tx, stock); err != nil {
			return fmt.Errorf("seed stock %q: %w", stock.productID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func insertStock(ctx context.Context, tx pgx.Tx, stock seedStock) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO stock (product_id, quantity)
		VALUES ($1, $2)
		ON CONFLICT (product_id) DO NOTHING`, stock.productID, stock.quantity)
	return err
}
