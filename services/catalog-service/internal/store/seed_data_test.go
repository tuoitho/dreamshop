package store

import (
	"strings"
	"testing"

	"ecommerce.local/catalog-service/internal/domain"
)

func TestSeedProductsMirrorCoreFields(t *testing.T) {
	products := seedProducts()
	if len(products) != 16 {
		t.Fatalf("expected 16 seed products, got %d", len(products))
	}

	seenSkus := make(map[string]string)
	for _, p := range products {
		if strings.TrimSpace(p.Name) == "" {
			t.Errorf("%s: name is empty", p.ID)
		}
		if strings.TrimSpace(p.CategoryID) == "" {
			t.Errorf("%s: categoryId is empty", p.ID)
		}
		if p.PriceMinor <= 0 || p.MaxPriceMinor < p.PriceMinor {
			t.Errorf("%s: invalid price bounds price=%d max=%d", p.ID, p.PriceMinor, p.MaxPriceMinor)
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
		for _, v := range p.Variants {
			if prev, exists := seenSkus[v.SkuID]; exists {
				t.Errorf("%s: duplicate seed SkuID %q already used by %s", p.ID, v.SkuID, prev)
			}
			seenSkus[v.SkuID] = p.ID
		}
	}
}

func TestSeedProductsSummaryProjection(t *testing.T) {
	for _, p := range seedProducts() {
		s := p.Summary()
		if s.ID != p.ID || s.Name != p.Name || s.PriceMinor != p.PriceMinor || s.MaxPriceMinor != p.MaxPriceMinor {
			t.Errorf("%s: summary projection mismatch", p.ID)
		}
		if len(s.Images) != len(p.Images) || len(s.SkuIDs) != len(p.Variants) {
			t.Errorf("%s: summary child slices mismatch", p.ID)
		}
	}
}

func TestSeedVariantsAreDomainValues(t *testing.T) {
	for _, p := range seedProducts() {
		for _, v := range p.Variants {
			if strings.TrimSpace(v.SkuID) == "" {
				t.Errorf("%s: variant with empty SkuID", p.ID)
			}
			if strings.TrimSpace(v.Label) == "" {
				t.Errorf("%s: variant with empty label", p.ID)
			}
			if v.PriceMinor <= 0 || v.OriginalPriceMinor < v.PriceMinor {
				t.Errorf("%s (%s): invalid variant prices %d/%d", p.ID, v.SkuID, v.PriceMinor, v.OriginalPriceMinor)
			}
			if v.ColorHex != "" && strings.TrimSpace(v.ColorHex) == "" {
				t.Errorf("%s: variant with blank color hex", p.ID)
			}
			var _ domain.ProductVariant = v
		}
	}
}
