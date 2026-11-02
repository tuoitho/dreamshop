package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrProductNotFound is returned when no product matches the requested id.
	ErrProductNotFound = errors.New("product not found")

	// ErrProductExists is returned when creating a product whose id already exists.
	ErrProductExists = errors.New("product already exists")

	// ErrSkuConflict is returned when a variant sku_id is already owned by another product.
	ErrSkuConflict = errors.New("variant sku already exists")

	// ErrMediaAlreadyAttached is returned when attempting to attach a media asset
	// that is already bound to a different product.
	ErrMediaAlreadyAttached = errors.New("media asset is already attached to another product")

	// ErrMediaExpired is returned when attempting to attach a media asset whose
	// pending/orphan upload window has already expired or is being swept.
	ErrMediaExpired = errors.New("media asset upload has expired")
)

// Repository is the catalog persistence boundary. Implementations live in
// infrastructure packages (for example internal/store); domain code depends on
// this interface, never on a concrete database driver.
type Repository interface {
	// GetByID returns the full product with images, videos, variants, and details.
	// It returns ErrProductNotFound when the id does not exist.
	GetByID(ctx context.Context, id string) (*Product, error)

	// Search applies the filter, sort, and pagination and returns summaries.
	// Soft-deleted products are excluded.
	Search(ctx context.Context, filter SearchFilter) (*SearchResult, error)

	// SoftDelete marks a product as deleted or restores it. Deleted products
	// are still returned by GetByID with Deleted set, but are excluded from
	// Search. It returns ErrProductNotFound when the id does not exist.
	SoftDelete(ctx context.Context, id string, deleted bool) error

	// ListAdmin lists products for administrative views including soft-deleted
	// items, global status counts, and category metrics.
	ListAdmin(ctx context.Context, filter AdminSearchFilter) (*AdminSearchResult, error)

	// ListInventoryItems returns lightweight catalog metadata for stock views.
	ListInventoryItems(ctx context.Context, filter InventoryCatalogFilter) ([]*InventoryCatalogItem, error)

	// Create persists a new product and reconciles any uploaded media assets.
	Create(ctx context.Context, product *Product, orphanGrace time.Duration) (*Product, error)

	// Update replaces a product's editable fields, child collections, and media bindings.
	Update(ctx context.Context, product *Product, orphanGrace time.Duration) (*Product, error)

	// CreateMediaAsset records a PENDING media upload prior to browser PUT.
	CreateMediaAsset(ctx context.Context, asset *MediaAsset) error
}
