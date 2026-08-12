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
