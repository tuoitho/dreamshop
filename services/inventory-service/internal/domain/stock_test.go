package domain

import (
	"errors"
	"testing"
)

func TestStockAvailableQuantity(t *testing.T) {
	tests := []struct {
		name     string
		quantity int32
		reserved int32
		want     int32
	}{
		{name: "subtracts reserved quantity", quantity: 7, reserved: 2, want: 5},
		{name: "clamps negative availability to zero", quantity: 2, reserved: 4, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := Stock{SkuID: "dt-1-v1", ProductID: "dt-1", Quantity: tt.quantity, Reserved: tt.reserved}
			if got := stock.AvailableQuantity(); got != tt.want {
				t.Fatalf("AvailableQuantity() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestVariantHasReservedStockErrorUnwrap(t *testing.T) {
	err := error(&VariantHasReservedStockError{SkuID: "dt-1-v2"})
	var target *VariantHasReservedStockError
	if !errors.As(err, &target) || target.SkuID != "dt-1-v2" {
		t.Fatalf("expected VariantHasReservedStockError with SkuID dt-1-v2, got %v", err)
	}
}
