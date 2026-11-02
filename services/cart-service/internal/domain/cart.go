package domain

type Line struct {
	ID        string
	ProductID string
	SkuID     string
	Quantity  int32
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

type FavoriteRecord struct {
	ID        string
	ProductID string
}

type FavoriteStatus int

const (
	FavoriteStatusAvailable FavoriteStatus = iota + 1
	FavoriteStatusLimited
	FavoriteStatusOutOfStock
	FavoriteStatusUnavailable
	FavoriteStatusUnknown
)

const favoriteLowStockThreshold int32 = 5

func ResolveFavoriteStatus(found, deleted bool, variantCount int, inventoryDegraded bool, available int32) FavoriteStatus {
	if !found || deleted || variantCount == 0 {
		return FavoriteStatusUnavailable
	}
	if inventoryDegraded {
		return FavoriteStatusUnknown
	}
	if available <= 0 {
		return FavoriteStatusOutOfStock
	}
	if available <= favoriteLowStockThreshold {
		return FavoriteStatusLimited
	}
	return FavoriteStatusAvailable
}
