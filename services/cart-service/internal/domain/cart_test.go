package domain

import "testing"

func TestResolveStatus(t *testing.T) {
	cases := []struct {
		name                         string
		found, deleted, variantFound bool
		available, quantity          int32
		want                         ItemStatus
	}{
		{"active", true, false, true, 5, 2, StatusAvailable},
		{"limited", true, false, true, 1, 2, StatusLimited},
		{"out of stock", true, false, true, 0, 2, StatusOutOfStock},
		{"deleted", true, true, true, 5, 2, StatusUnavailable},
		{"variant removed", true, false, false, 5, 2, StatusUnavailable},
		{"missing", false, false, false, 0, 2, StatusUnavailable},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveStatus(tt.found, tt.deleted, tt.variantFound, tt.available, tt.quantity); got != tt.want {
				t.Fatalf("ResolveStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveFavoriteStatus(t *testing.T) {
	cases := []struct {
		name              string
		found, deleted    bool
		variantCount      int
		inventoryDegraded bool
		available         int32
		want              FavoriteStatus
	}{
		{"available above low stock threshold", true, false, 2, false, 6, FavoriteStatusAvailable},
		{"limited at threshold", true, false, 1, false, 5, FavoriteStatusLimited},
		{"limited single unit", true, false, 1, false, 1, FavoriteStatusLimited},
		{"out of stock zero", true, false, 1, false, 0, FavoriteStatusOutOfStock},
		{"out of stock negative", true, false, 1, false, -2, FavoriteStatusOutOfStock},
		{"inventory degraded on active product", true, false, 1, true, 0, FavoriteStatusUnknown},
		{"soft deleted product", true, true, 1, false, 10, FavoriteStatusUnavailable},
		{"soft deleted overrides inventory degraded", true, true, 1, true, 0, FavoriteStatusUnavailable},
		{"variantless product", true, false, 0, false, 10, FavoriteStatusUnavailable},
		{"missing product", false, false, 0, false, 0, FavoriteStatusUnavailable},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveFavoriteStatus(tt.found, tt.deleted, tt.variantCount, tt.inventoryDegraded, tt.available)
			if got != tt.want {
				t.Fatalf("ResolveFavoriteStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
