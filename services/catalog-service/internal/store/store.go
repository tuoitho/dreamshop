package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ecommerce.local/catalog-service/internal/domain"
)

// Store implements domain.Repository against PostgreSQL through pgxpool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const productColumns = `
	id, name, short_description, description, brand, category_id, category_name,
	price_amount_minor, original_price_amount_minor, rating, review_count,
	sold_count, is_new, badge, featured, emoji, image_hue, deleted_at`

// GetByID loads a full product including images, variants, and details.
// Soft-deleted products are returned with Deleted set.
func (s *Store) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	row := s.pool.QueryRow(ctx, "SELECT "+productColumns+" FROM products WHERE id = $1", id)

	var p domain.Product
	var imageHue []int16
	var deletedAt *time.Time
	err := row.Scan(
		&p.ID, &p.Name, &p.ShortDescription, &p.Description, &p.Brand,
		&p.CategoryID, &p.CategoryName, &p.PriceMinor, &p.OriginalPriceMinor,
		&p.Rating, &p.ReviewCount, &p.SoldCount, &p.IsNew, &p.Badge,
		&p.Featured, &p.Emoji, &imageHue, &deletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product %q: %w", id, err)
	}
	p.ImageHue = toUint32Hue(imageHue)
	p.Deleted = deletedAt != nil

	images, err := s.loadChildStrings(ctx,
		"SELECT url FROM product_images WHERE product_id = $1 ORDER BY position", id)
	if err != nil {
		return nil, fmt.Errorf("load images for product %q: %w", id, err)
	}
	p.Images = images

	variants, err := s.loadVariants(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load variants for product %q: %w", id, err)
	}
	p.Variants = variants

	details, err := s.loadChildStrings(ctx,
		"SELECT detail FROM product_details WHERE product_id = $1 ORDER BY position", id)
	if err != nil {
		return nil, fmt.Errorf("load details for product %q: %w", id, err)
	}
	p.Details = details

	return &p, nil
}

func (s *Store) loadChildStrings(ctx context.Context, query, id string) ([]string, error) {
	rows, err := s.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) loadVariants(ctx context.Context, id string) ([]domain.ProductVariant, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT label, color_hex FROM product_variants WHERE product_id = $1 ORDER BY position", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ProductVariant
	for rows.Next() {
		var v domain.ProductVariant
		var colorHex *string
		if err := rows.Scan(&v.Label, &colorHex); err != nil {
			return nil, err
		}
		if colorHex != nil {
			v.ColorHex = *colorHex
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SoftDelete marks a product soft-deleted (deleted_at now()) or restores it
// (deleted_at NULL). GetByID still sees the product with Deleted set; Search
// excludes it via buildWhere.
func (s *Store) SoftDelete(ctx context.Context, id string, deleted bool) error {
	query := "UPDATE products SET deleted_at = CASE WHEN $2 THEN now() ELSE NULL END WHERE id = $1"
	commandTag, err := s.pool.Exec(ctx, query, id, deleted)
	if err != nil {
		return fmt.Errorf("set product deleted %q: %w", id, err)
	}
	if commandTag.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

// Search applies filters, sort, and pagination and returns product summaries.
// The in-memory domain.SearchProducts is the behavioral reference; this query
// must stay consistent with it.
func (s *Store) Search(ctx context.Context, filter domain.SearchFilter) (*domain.SearchResult, error) {
	filter = filter.Normalize()

	where, args := buildWhere(filter)
	countSQL := "SELECT COUNT(*) FROM products WHERE " + where
	totalItems := int64(0)
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count products: %w", err)
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	limit := fmt.Sprintf("$%d", len(args)-1)
	offset := fmt.Sprintf("$%d", len(args))

	query := `
		SELECT id, name, brand, category_id, category_name, price_amount_minor,
		       original_price_amount_minor, rating, review_count, sold_count,
		       is_new, badge, featured, emoji, image_hue
		FROM products
		WHERE ` + where + `
		ORDER BY ` + orderBy(filter.Sort) + `
		LIMIT ` + limit + ` OFFSET ` + offset

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	var items []*domain.Summary
	pageIDs := make([]string, 0, filter.PageSize)
	for rows.Next() {
		s := &domain.Summary{}
		var imageHue []int16
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Brand, &s.CategoryID, &s.CategoryName,
			&s.PriceMinor, &s.OriginalPriceMinor, &s.Rating, &s.ReviewCount,
			&s.SoldCount, &s.IsNew, &s.Badge, &s.Featured, &s.Emoji, &imageHue,
		); err != nil {
			return nil, fmt.Errorf("scan product row: %w", err)
		}
		s.ImageHue = toUint32Hue(imageHue)
		items = append(items, s)
		pageIDs = append(pageIDs, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product rows: %w", err)
	}

	images, err := s.loadImagesFor(ctx, pageIDs)
	if err != nil {
		return nil, fmt.Errorf("load page images: %w", err)
	}
	for _, s := range items {
		s.Images = images[s.ID]
	}

	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	return &domain.SearchResult{
		Items:      items,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}, nil
}

// loadImagesFor returns the ordered image URLs for the given product ids in a
// single round trip.
func (s *Store) loadImagesFor(ctx context.Context, ids []string) (map[string][]string, error) {
	if len(ids) == 0 {
		return map[string][]string{}, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT product_id, url FROM product_images
		 WHERE product_id = ANY($1) ORDER BY product_id, position`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]string, len(ids))
	for rows.Next() {
		var productID, url string
		if err := rows.Scan(&productID, &url); err != nil {
			return nil, err
		}
		out[productID] = append(out[productID], url)
	}
	return out, rows.Err()
}

// buildWhere produces the parameterized WHERE clause and argument slice for a
// normalized filter. Soft-deleted products are always excluded.
func buildWhere(filter domain.SearchFilter) (string, []any) {
	clauses := []string{"deleted_at IS NULL"}
	args := []any{}
	next := func(a any) string {
		args = append(args, a)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.Query != "" {
		pattern := next("%" + filter.Query + "%")
		clauses = append(clauses,
			fmt.Sprintf("(name ILIKE %s OR brand ILIKE %s OR category_name ILIKE %s)",
				pattern, pattern, pattern))
	}
	if filter.CategoryID != "" {
		clauses = append(clauses, "category_id = "+next(filter.CategoryID))
	}
	if filter.PriceMin > 0 {
		clauses = append(clauses, "price_amount_minor >= "+next(filter.PriceMin))
	}
	if filter.PriceMax > 0 {
		clauses = append(clauses, "price_amount_minor <= "+next(filter.PriceMax))
	}
	if filter.OnSale {
		clauses = append(clauses, "original_price_amount_minor > price_amount_minor")
	}
	if filter.RatingMin > 0 {
		clauses = append(clauses, "rating >= "+next(filter.RatingMin))
	}

	return strings.Join(clauses, " AND "), args
}

// orderBy maps a canonical sort key to SQL ORDER BY. The id tiebreaker keeps
// results deterministic and consistent with the pure in-memory search.
func orderBy(sortKey string) string {
	switch sortKey {
	case domain.SortPriceAsc:
		return "price_amount_minor ASC, id ASC"
	case domain.SortPriceDesc:
		return "price_amount_minor DESC, id ASC"
	case domain.SortNewest:
		return "is_new DESC, id ASC"
	case domain.SortRating:
		return "rating DESC, id ASC"
	case domain.SortPopularity:
		return "sold_count DESC, id ASC"
	default:
		return "sold_count DESC, id ASC"
	}
}

// toUint32Hue converts a scanned smallint[] hue to the domain []uint32.
func toUint32Hue(hue []int16) []uint32 {
	if len(hue) == 0 {
		return nil
	}
	out := make([]uint32, len(hue))
	for i, h := range hue {
		out[i] = uint32(h)
	}
	return out
}
