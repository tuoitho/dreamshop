package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ecommerce.local/catalog-service/internal/domain"
)

// Seed populates the products catalog from the embedded demo data when the
// products table is empty. It is idempotent: an existing catalog is left
// untouched so operators can replace the demo data without surprises.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var count int64
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&count); err != nil {
		return fmt.Errorf("check products table: %w", err)
	}
	if count > 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, p := range seedProducts() {
		if err := insertProduct(ctx, tx, p); err != nil {
			return fmt.Errorf("seed product %q: %w", p.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

// insertProduct inserts a product and its child rows with stable positions.
func insertProduct(ctx context.Context, tx pgx.Tx, p *domain.Product) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO products (
			id, name, short_description, description, brand, category_id,
			category_name, price_amount_minor, original_price_amount_minor,
			rating, review_count, sold_count, is_new, badge, featured, emoji, image_hue
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		p.ID, p.Name, p.ShortDescription, p.Description, p.Brand, p.CategoryID,
		p.CategoryName, p.PriceMinor, p.OriginalPriceMinor, p.Rating, p.ReviewCount,
		p.SoldCount, p.IsNew, p.Badge, p.Featured, p.Emoji, toInt16Hue(p.ImageHue),
	); err != nil {
		return fmt.Errorf("insert product row: %w", err)
	}

	for i, url := range p.Images {
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_images (product_id, url, position) VALUES ($1, $2, $3)",
			p.ID, url, i,
		); err != nil {
			return fmt.Errorf("insert image %d: %w", i, err)
		}
	}

	for i, v := range p.Variants {
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_variants (product_id, label, color_hex, position) VALUES ($1, $2, $3, $4)",
			p.ID, v.Label, v.ColorHex, i,
		); err != nil {
			return fmt.Errorf("insert variant %d: %w", i, err)
		}
	}

	for i, detail := range p.Details {
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_details (product_id, detail, position) VALUES ($1, $2, $3)",
			p.ID, detail, i,
		); err != nil {
			return fmt.Errorf("insert detail %d: %w", i, err)
		}
	}

	return nil
}

// toInt16Hue converts the domain hue to a smallint-compatible slice for insert.
func toInt16Hue(hue []uint32) []int16 {
	if len(hue) == 0 {
		return nil
	}
	out := make([]int16, len(hue))
	for i, h := range hue {
		out[i] = int16(h)
	}
	return out
}
