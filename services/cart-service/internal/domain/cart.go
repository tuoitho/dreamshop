package domain

type Line struct {
	ID           string
	ProductID    string
	VariantLabel string
	Quantity     int32
}

type ItemStatus int

const (
	StatusAvailable ItemStatus = iota + 1
	StatusLimited
	StatusOutOfStock
	StatusUnavailable
)

func ResolveStatus(found, deleted, variantFound bool, available, quantity int32) ItemStatus {
	if !found || deleted || !variantFound {
		return StatusUnavailable
	}
	if available <= 0 {
		return StatusOutOfStock
	}
	if available < quantity {
		return StatusLimited
	}
	return StatusAvailable
}
