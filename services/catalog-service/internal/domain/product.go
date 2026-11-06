package domain

import "time"

// ProductVariant is an independently priced and stocked SKU option of a product.
// Pure domain value.
type ProductVariant struct {
	SkuID              string
	Label              string
	ColorHex           string
	PriceMinor         int64
	OriginalPriceMinor int64
}

// Product is the full catalog product aggregate. Pure domain value: it has no
// infrastructure imports and does not carry stock or availability (Inventory
// owns that fact).
type Product struct {
	ID                 string
	Name               string
	ShortDescription   string
	Description        string
	Brand              string
	CategoryID         string
	CategoryName       string
	PriceMinor         int64
	OriginalPriceMinor int64
	MaxPriceMinor      int64
	Images             []string
	Videos             []string
	Rating             float64
	ReviewCount        int64
	SoldCount          int64
	IsNew              bool
	Badge              string
	Featured           bool
	Emoji              string
	ImageHue           []uint32
	Variants           []ProductVariant
	Details            []string
	Deleted            bool
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Summary is the search-result projection of a Product. It carries the same
// core fields so cards and lists can render without loading the full detail.
type Summary struct {
	ID                 string
	Name               string
	Brand              string
	CategoryID         string
	CategoryName       string
	PriceMinor         int64
	OriginalPriceMinor int64
	MaxPriceMinor      int64
	Images             []string
	Rating             float64
	ReviewCount        int64
	SoldCount          int64
	IsNew              bool
	Badge              string
	Featured           bool
	Emoji              string
	ImageHue           []uint32
	SkuIDs             []string
	Version            int64
}

// Summary projects a Product into its search-result representation.
func (p *Product) Summary() *Summary {
	maxPrice := p.MaxPriceMinor
	if maxPrice < p.PriceMinor {
		maxPrice = p.PriceMinor
	}
	version := p.Version
	if version <= 0 {
		version = 1
	}
	var skuIDs []string
	if len(p.Variants) > 0 {
		skuIDs = make([]string, 0, len(p.Variants))
		for _, v := range p.Variants {
			if v.SkuID != "" {
				skuIDs = append(skuIDs, v.SkuID)
			}
		}
	}
	return &Summary{
		ID:                 p.ID,
		Name:               p.Name,
		Brand:              p.Brand,
		CategoryID:         p.CategoryID,
		CategoryName:       p.CategoryName,
		PriceMinor:         p.PriceMinor,
		OriginalPriceMinor: p.OriginalPriceMinor,
		MaxPriceMinor:      maxPrice,
		Images:             p.Images,
		Rating:             p.Rating,
		ReviewCount:        p.ReviewCount,
		SoldCount:          p.SoldCount,
		IsNew:              p.IsNew,
		Badge:              p.Badge,
		Featured:           p.Featured,
		Emoji:              p.Emoji,
		ImageHue:           p.ImageHue,
		SkuIDs:             skuIDs,
		Version:            version,
	}
}
