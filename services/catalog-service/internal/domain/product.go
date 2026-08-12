package domain

// ProductVariant is a variant of a product (for example a color or an option
// group label). Pure domain value.
type ProductVariant struct {
	Label    string
	ColorHex string
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
	Images             []string
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
	Images             []string
	Rating             float64
	ReviewCount        int64
	SoldCount          int64
	IsNew              bool
	Badge              string
	Featured           bool
	Emoji              string
	ImageHue           []uint32
}

// Summary projects a Product into its search-result representation.
func (p *Product) Summary() *Summary {
	return &Summary{
		ID:                 p.ID,
		Name:               p.Name,
		Brand:              p.Brand,
		CategoryID:         p.CategoryID,
		CategoryName:       p.CategoryName,
		PriceMinor:         p.PriceMinor,
		OriginalPriceMinor: p.OriginalPriceMinor,
		Images:             p.Images,
		Rating:             p.Rating,
		ReviewCount:        p.ReviewCount,
		SoldCount:          p.SoldCount,
		IsNew:              p.IsNew,
		Badge:              p.Badge,
		Featured:           p.Featured,
		Emoji:              p.Emoji,
		ImageHue:           p.ImageHue,
	}
}
