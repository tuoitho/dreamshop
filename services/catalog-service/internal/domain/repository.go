package domain

import (
	"context"
	"errors"
)

// ErrProductNotFound is returned by Repository.GetByID when no product matches
// the requested id.
var ErrProductNotFound = errors.New("product not found")

// Repository is the catalog persistence boundary. Implementations live in
// infrastructure packages (for example internal/store); domain code depends on
// this interface, never on a concrete database driver.
type Repository interface {
	// GetByID returns the full product with images, variants, and details.
	// It returns ErrProductNotFound when the id does not exist.
	GetByID(ctx context.Context, id string) (*Product, error)

	// Search applies the filter, sort, and pagination and returns summaries.
	// Soft-deleted products are excluded.
	Search(ctx context.Context, filter SearchFilter) (*SearchResult, error)

	// SoftDelete marks a product as deleted or restores it. Deleted products
	// are still returned by GetByID with Deleted set, but are excluded from
	// Search. It returns ErrProductNotFound when the id does not exist.
	SoftDelete(ctx context.Context, id string, deleted bool) error
}
