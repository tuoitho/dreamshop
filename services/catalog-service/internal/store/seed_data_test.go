package store

import (
	"strings"
	"testing"

	"ecommerce.local/catalog-service/internal/domain"
)

// TestSeedProductsMirrorCoreFields guards the demo seed against regressions in
// the seedItem -> domain.Product conversion: every product must carry a name,
// category, price, description, and images so the detail page renders fully.
func TestSeedProductsMirrorCoreFields(t *testing.T) {
	products := seedProducts()
	if len(products) != 16 {
		t.Fatalf("expected 16 seed products, got %d", len(products))
	}

	for _, p := range products {
		if strings.TrimSpace(p.Name) == "" {
			t.Errorf("%s: name is empty", p.ID)
		}
		if strings.TrimSpace(p.CategoryID) == "" {
			t.Errorf("%s: categoryId is empty", p.ID)
		}
		if p.PriceMinor <= 0 {
			t.Errorf("%s: price must be positive, got %d", p.ID, p.PriceMinor)
		}
		if strings.TrimSpace(p.Description) == "" {
			t.Errorf("%s: description is empty", p.ID)
		}
		if len(p.Images) == 0 {
			t.Errorf("%s: no images", p.ID)
		}
		if p.Emoji == "" {
			t.Errorf("%s: emoji is empty", p.ID)
		}
		if len(p.Variants) == 0 {
			t.Errorf("%s: no variants", p.ID)
		}
		if len(p.Details) == 0 {
			t.Errorf("%s: no details", p.ID)
		}
	}
}

// TestSeedProductsSummaryProjection ensures the search projection still carries
// the card-rendering fields for every seed product.
func TestSeedProductsSummaryProjection(t *testing.T) {
	for _, p := range seedProducts() {
		s := p.Summary()
		if s.ID != p.ID || s.Name != p.Name || s.PriceMinor != p.PriceMinor {
			t.Errorf("%s: summary projection mismatch", p.ID)
		}
		if len(s.Images) != len(p.Images) {
			t.Errorf("%s: summary images mismatch", p.ID)
		}
	}
}

// TestSeedVariantsAreDomainValues ensures variant color data round-trips.
func TestSeedVariantsAreDomainValues(t *testing.T) {
	for _, p := range seedProducts() {
		for _, v := range p.Variants {
			if strings.TrimSpace(v.Color) == "" || strings.TrimSpace(v.ColorHex) == "" {
				t.Errorf("%s: variant with empty color", p.ID)
			}
			var _ domain.ProductVariant = v
		}
	}
}
