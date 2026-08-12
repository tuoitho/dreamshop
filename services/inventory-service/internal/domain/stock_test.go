package domain

import "testing"

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
			stock := Stock{Quantity: tt.quantity, Reserved: tt.reserved}
			if got := stock.AvailableQuantity(); got != tt.want {
				t.Fatalf("AvailableQuantity() = %d, want %d", got, tt.want)
			}
		})
	}
}
