package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ecommerce.local/catalog-service/internal/domain"
)

const pgUniqueViolation = "23505"

// ListAdmin returns paginated administrative product summaries alongside global
// active/deleted counts and active category distribution.
func (s *Store) ListAdmin(ctx context.Context, filter domain.AdminSearchFilter) (*domain.AdminSearchResult, error) {
	filter = filter.Normalize()

	var activeCount, deletedCount int64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE deleted_at IS NULL),
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL)
		FROM products
	`).Scan(&activeCount, &deletedCount); err != nil {
		return nil, fmt.Errorf("count admin product statuses: %w", err)
	}

	catRows, err := s.pool.Query(ctx, `
		SELECT category_id, MAX(category_name) AS category_name, COUNT(*) AS active_count
		FROM products
		WHERE deleted_at IS NULL
		GROUP BY category_id
		ORDER BY category_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query category metrics: %w", err)
	}
	var categories []*domain.CategoryMetric
	for catRows.Next() {
		cm := &domain.CategoryMetric{}
		if err := catRows.Scan(&cm.CategoryID, &cm.CategoryName, &cm.ActiveProductCount); err != nil {
			catRows.Close()
			return nil, fmt.Errorf("scan category metric: %w", err)
		}
		categories = append(categories, cm)
	}
	if err := catRows.Err(); err != nil {
		catRows.Close()
		return nil, fmt.Errorf("iterate category metrics: %w", err)
	}
	catRows.Close()

	where, args := buildAdminWhere(filter)
	var totalItems int64
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM products p WHERE "+where, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count admin products: %w", err)
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))

	query := `
		SELECT p.id, p.name, p.brand, p.category_id, p.category_name,
		       p.price_amount_minor, p.original_price_amount_minor,
		       p.rating, p.review_count, p.sold_count, p.is_new, p.badge,
		       p.featured, p.emoji, p.image_hue, p.deleted_at,
		       p.created_at, p.updated_at,
		       (SELECT COUNT(*)::int FROM product_variants pv WHERE pv.product_id = p.id) AS variant_count,
		       (SELECT COUNT(*)::int FROM product_videos pvid WHERE pvid.product_id = p.id) AS video_count
		FROM products p
		WHERE ` + where + `
		ORDER BY ` + adminOrderBy(filter.Sort) + `
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list admin products: %w", err)
	}
	defer rows.Close()

	var items []*domain.AdminSummary
	pageIDs := make([]string, 0, filter.PageSize)
	for rows.Next() {
		item := &domain.AdminSummary{}
		var imageHue []int16
		var deletedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Brand, &item.CategoryID, &item.CategoryName,
			&item.PriceMinor, &item.OriginalPriceMinor, &item.Rating, &item.ReviewCount,
			&item.SoldCount, &item.IsNew, &item.Badge, &item.Featured, &item.Emoji,
			&imageHue, &deletedAt, &item.CreatedAt, &item.UpdatedAt,
			&item.VariantCount, &item.VideoCount,
		); err != nil {
			return nil, fmt.Errorf("scan admin product row: %w", err)
		}
		item.ImageHue = toUint32Hue(imageHue)
		item.Deleted = deletedAt != nil
		items = append(items, item)
		pageIDs = append(pageIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin product rows: %w", err)
	}

	imagesByProduct, err := s.loadImagesFor(ctx, pageIDs)
	if err != nil {
		return nil, fmt.Errorf("load admin page images: %w", err)
	}
	for _, item := range items {
		item.Images = imagesByProduct[item.ID]
	}

	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	return &domain.AdminSearchResult{
		Items:        items,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
		TotalItems:   totalItems,
		TotalPages:   totalPages,
		ActiveCount:  activeCount,
		DeletedCount: deletedCount,
		Categories:   categories,
	}, nil
}

// ListInventoryItems returns lightweight catalog metadata capped at 1000 items.
func (s *Store) ListInventoryItems(ctx context.Context, filter domain.InventoryCatalogFilter) ([]*domain.InventoryCatalogItem, error) {
	clauses := []string{"1=1"}
	var args []any
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if !filter.IncludeDeleted {
		clauses = append(clauses, "p.deleted_at IS NULL")
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		pat := next("%" + q + "%")
		clauses = append(clauses, fmt.Sprintf("(p.id ILIKE %s OR p.name ILIKE %s OR p.brand ILIKE %s OR p.category_name ILIKE %s)", pat, pat, pat, pat))
	}
	if cat := strings.TrimSpace(filter.CategoryID); cat != "" {
		clauses = append(clauses, "p.category_id = "+next(cat))
	}

	query := `
		SELECT p.id, p.name, p.brand, p.category_id, p.category_name,
		       COALESCE((SELECT pi.url FROM product_images pi WHERE pi.product_id = p.id ORDER BY pi.position ASC LIMIT 1), '') AS image_url,
		       p.emoji, p.image_hue, p.deleted_at
		FROM products p
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY (p.deleted_at IS NOT NULL) ASC, p.updated_at DESC, p.id ASC
		LIMIT 1000`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inventory catalog items: %w", err)
	}
	defer rows.Close()

	var out []*domain.InventoryCatalogItem
	for rows.Next() {
		item := &domain.InventoryCatalogItem{}
		var imageHue []int16
		var deletedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Brand, &item.CategoryID, &item.CategoryName,
			&item.ImageURL, &item.Emoji, &imageHue, &deletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan inventory catalog item: %w", err)
		}
		item.ImageHue = toUint32Hue(imageHue)
		item.Deleted = deletedAt != nil
		out = append(out, item)
	}
	return out, rows.Err()
}

// Create inserts a new product, child collections, and reconciles media bindings atomically.
func (s *Store) Create(ctx context.Context, p *domain.Product, orphanGrace time.Duration) (*domain.Product, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create product tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hue := toInt16Hue(p.ImageHue)
	if hue == nil {
		hue = []int16{}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO products (
			id, name, short_description, description, brand, category_id,
			category_name, price_amount_minor, original_price_amount_minor,
			rating, review_count, sold_count, is_new, badge, featured, emoji, image_hue,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0, $10, $11, $12, $13, $14, now(), now())
	`,
		p.ID, p.Name, p.ShortDescription, p.Description, p.Brand, p.CategoryID,
		p.CategoryName, p.PriceMinor, p.OriginalPriceMinor,
		p.IsNew, p.Badge, p.Featured, p.Emoji, hue,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, domain.ErrProductExists
		}
		return nil, fmt.Errorf("insert product %q: %w", p.ID, err)
	}

	if err := replaceProductChildren(ctx, tx, p); err != nil {
		return nil, err
	}
	if err := reconcileMediaAssets(ctx, tx, p.ID, p.Images, p.Videos, orphanGrace, false); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create product %q: %w", p.ID, err)
	}
	return s.GetByID(ctx, p.ID)
}

// Update modifies an existing product, replaces child collections, and reconciles
// attached vs. orphaned media assets atomically.
func (s *Store) Update(ctx context.Context, p *domain.Product, orphanGrace time.Duration) (*domain.Product, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update product tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hue := toInt16Hue(p.ImageHue)
	if hue == nil {
		hue = []int16{}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE products
		SET name = $2,
		    short_description = $3,
		    description = $4,
		    brand = $5,
		    category_id = $6,
		    category_name = $7,
		    price_amount_minor = $8,
		    original_price_amount_minor = $9,
		    is_new = $10,
		    badge = $11,
		    featured = $12,
		    emoji = $13,
		    image_hue = $14,
		    updated_at = now()
		WHERE id = $1
	`,
		p.ID, p.Name, p.ShortDescription, p.Description, p.Brand, p.CategoryID,
		p.CategoryName, p.PriceMinor, p.OriginalPriceMinor,
		p.IsNew, p.Badge, p.Featured, p.Emoji, hue,
	)
	if err != nil {
		return nil, fmt.Errorf("update product %q: %w", p.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrProductNotFound
	}

	for _, table := range []string{"product_images", "product_videos", "product_variants", "product_details"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE product_id = $1", p.ID); err != nil {
			return nil, fmt.Errorf("clear %s for %q: %w", table, p.ID, err)
		}
	}

	if err := replaceProductChildren(ctx, tx, p); err != nil {
		return nil, err
	}
	if err := reconcileMediaAssets(ctx, tx, p.ID, p.Images, p.Videos, orphanGrace, true); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update product %q: %w", p.ID, err)
	}
	return s.GetByID(ctx, p.ID)
}

// CreateMediaAsset inserts a PENDING media asset row prior to presigned browser PUT.
func (s *Store) CreateMediaAsset(ctx context.Context, asset *domain.MediaAsset) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO product_media_assets (
			id, bucket, object_key, public_url, media_type,
			original_filename, content_type, size_bytes, status,
			uploaded_by, product_id, created_at, attached_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), $12, $13)
	`,
		asset.ID, asset.Bucket, asset.ObjectKey, asset.PublicURL, asset.MediaType,
		asset.OriginalFilename, asset.ContentType, asset.SizeBytes, asset.Status,
		asset.UploadedBy, asset.ProductID, asset.AttachedAt, asset.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert product_media_assets %q: %w", asset.ID, err)
	}
	return nil
}

// SweepExpiredMedia claims expired PENDING/ORPHANED rows using FOR UPDATE SKIP LOCKED
// with a short lease, releases the DB transaction before invoking S3 RemoveObject,
// and deletes rows whose storage object removal succeeded.
func (s *Store) SweepExpiredMedia(ctx context.Context, batchSize int, removeObject func(context.Context, string) error) (int, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	type claimedAsset struct {
		id        string
		objectKey string
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin media sweep claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM product_media_assets
			WHERE status IN ('PENDING', 'ORPHANED')
			  AND expires_at <= now()
			  AND (sweep_locked_until IS NULL OR sweep_locked_until <= now())
			ORDER BY expires_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE product_media_assets m
		SET sweep_locked_until = now() + interval '2 minutes'
		FROM candidates c
		WHERE m.id = c.id
		RETURNING m.id::text, m.object_key
	`, batchSize)
	if err != nil {
		return 0, fmt.Errorf("claim expired media assets: %w", err)
	}

	var claimed []claimedAsset
	for rows.Next() {
		var item claimedAsset
		if err := rows.Scan(&item.id, &item.objectKey); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan claimed media asset: %w", err)
		}
		claimed = append(claimed, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate claimed media assets: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit media sweep claim tx: %w", err)
	}
	if len(claimed) == 0 {
		return 0, nil
	}

	deleted := 0
	for _, item := range claimed {
		if err := removeObject(ctx, item.objectKey); err != nil {
			_, _ = s.pool.Exec(ctx, `
				UPDATE product_media_assets
				SET sweep_locked_until = NULL
				WHERE id = $1 AND status IN ('PENDING', 'ORPHANED')
			`, item.id)
			continue
		}
		tag, err := s.pool.Exec(ctx, `
			DELETE FROM product_media_assets
			WHERE id = $1 AND status IN ('PENDING', 'ORPHANED')
		`, item.id)
		if err == nil && tag.RowsAffected() > 0 {
			deleted++
		}
	}
	return deleted, nil
}

func replaceProductChildren(ctx context.Context, tx pgx.Tx, p *domain.Product) error {
	for i, url := range p.Images {
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_images (product_id, url, position) VALUES ($1, $2, $3)",
			p.ID, url, i,
		); err != nil {
			return fmt.Errorf("insert image %d: %w", i, err)
		}
	}
	for i, url := range p.Videos {
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_videos (product_id, url, position) VALUES ($1, $2, $3)",
			p.ID, url, i,
		); err != nil {
			return fmt.Errorf("insert video %d: %w", i, err)
		}
	}
	for i, v := range p.Variants {
		var colorHex *string
		if v.ColorHex != "" {
			ch := v.ColorHex
			colorHex = &ch
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO product_variants (product_id, label, color_hex, position) VALUES ($1, $2, $3, $4)",
			p.ID, v.Label, colorHex, i,
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

func reconcileMediaAssets(ctx context.Context, tx pgx.Tx, productID string, images, videos []string, orphanGrace time.Duration, isUpdate bool) error {
	if orphanGrace <= 0 {
		orphanGrace = 24 * time.Hour
	}
	seen := make(map[string]struct{}, len(images)+len(videos))
	urls := make([]string, 0, len(images)+len(videos))
	for _, u := range append(append([]string(nil), images...), videos...) {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}

	if len(urls) > 0 {
		rows, err := tx.Query(ctx, `
			SELECT public_url,
			       status,
			       COALESCE(product_id, ''),
			       ((status IN ('PENDING', 'ORPHANED')) AND (
			           (expires_at IS NOT NULL AND expires_at <= now())
			           OR (sweep_locked_until IS NOT NULL AND sweep_locked_until > now())
			       )) AS is_expired
			FROM product_media_assets
			WHERE public_url = ANY($1)
			ORDER BY id
			FOR UPDATE
		`, urls)
		if err != nil {
			return fmt.Errorf("lock media assets for reconciliation: %w", err)
		}
		for rows.Next() {
			var publicURL, rowStatus, rowProductID string
			var isExpired bool
			if err := rows.Scan(&publicURL, &rowStatus, &rowProductID, &isExpired); err != nil {
				rows.Close()
				return fmt.Errorf("scan locked media asset: %w", err)
			}
			if rowStatus == domain.MediaStatusAttached && rowProductID != "" && rowProductID != productID {
				rows.Close()
				return domain.ErrMediaAlreadyAttached
			}
			if isExpired {
				rows.Close()
				return domain.ErrMediaExpired
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate locked media assets: %w", err)
		}
		rows.Close()
	}

	if isUpdate {
		orphanExpiresAt := time.Now().UTC().Add(orphanGrace)
		if _, err := tx.Exec(ctx, `
			UPDATE product_media_assets
			SET status = 'ORPHANED',
			    product_id = NULL,
			    attached_at = NULL,
			    expires_at = $2,
			    sweep_locked_until = NULL
			WHERE product_id = $1
			  AND status = 'ATTACHED'
			  AND NOT (public_url = ANY($3))
		`, productID, orphanExpiresAt, urls); err != nil {
			return fmt.Errorf("mark removed media assets orphaned: %w", err)
		}
	}

	if len(urls) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE product_media_assets
			SET status = 'ATTACHED',
			    product_id = $1,
			    attached_at = COALESCE(attached_at, now()),
			    expires_at = NULL,
			    sweep_locked_until = NULL
			WHERE public_url = ANY($2)
			  AND (
			      (status = 'ATTACHED' AND product_id = $1)
			      OR (status IN ('PENDING', 'ORPHANED')
			          AND (expires_at IS NULL OR expires_at > now())
			          AND (sweep_locked_until IS NULL OR sweep_locked_until <= now()))
			  )
		`, productID, urls); err != nil {
			return fmt.Errorf("attach media assets: %w", err)
		}
	}
	return nil
}

func buildAdminWhere(filter domain.AdminSearchFilter) (string, []any) {
	clauses := []string{"1=1"}
	var args []any
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	switch filter.Status {
	case domain.AdminStatusActive:
		clauses = append(clauses, "p.deleted_at IS NULL")
	case domain.AdminStatusDeleted:
		clauses = append(clauses, "p.deleted_at IS NOT NULL")
	}
	if filter.Query != "" {
		pat := next("%" + filter.Query + "%")
		clauses = append(clauses, fmt.Sprintf("(p.id ILIKE %s OR p.name ILIKE %s OR p.brand ILIKE %s OR p.category_name ILIKE %s)", pat, pat, pat, pat))
	}
	if filter.CategoryID != "" {
		clauses = append(clauses, "p.category_id = "+next(filter.CategoryID))
	}
	return strings.Join(clauses, " AND "), args
}

func adminOrderBy(sortKey string) string {
	switch sortKey {
	case domain.SortPriceAsc:
		return "p.price_amount_minor ASC, p.id ASC"
	case domain.SortPriceDesc:
		return "p.price_amount_minor DESC, p.id ASC"
	case domain.SortNewest:
		return "p.is_new DESC, p.created_at DESC, p.id ASC"
	case domain.SortRating:
		return "p.rating DESC, p.id ASC"
	case domain.SortPopularity:
		return "p.sold_count DESC, p.id ASC"
	default:
		return "p.updated_at DESC, p.id ASC"
	}
}
