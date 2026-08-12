package domain

import (
	"sort"
	"strings"
)

// Sort keys accepted by SearchFilter.Sort. Empty or unknown values resolve to
// SortPopularity.
const (
	SortPopularity = "pho-bien"
	SortPriceAsc   = "gia-thap"
	SortPriceDesc  = "gia-cao"
	SortNewest     = "moi-nhat"
	SortRating     = "danh-gia"
)

const (
	defaultPageSize = 20
	maxPageSize     = 50
)

// SearchFilter carries the user-facing search criteria. Page and PageSize use
// 1-based page numbering; see Normalize for the accepted bounds.
type SearchFilter struct {
	Query      string
	CategoryID string
	Sort       string
	PriceMin   int64
	PriceMax   int64
	OnSale     bool
	RatingMin  int32
	Page       int32
	PageSize   int32
}

// Normalize clamps the filter to the documented bounds: page >= 1, pageSize in
// [1, 50] with a default of 20, and priceMax 0 meaning unbounded (left as is).
// It returns a copy and never mutates the receiver.
func (f SearchFilter) Normalize() SearchFilter {
	f.Query = strings.TrimSpace(f.Query)
	f.Sort = SortKey(f.Sort)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultPageSize
	}
	if f.PageSize > maxPageSize {
		f.PageSize = maxPageSize
	}
	return f
}

// SortKey resolves a sort key to one of the canonical values. Unknown or empty
// input resolves to SortPopularity.
func SortKey(sort string) string {
	switch sort {
	case SortPopularity, SortPriceAsc, SortPriceDesc, SortNewest, SortRating:
		return sort
	default:
		return SortPopularity
	}
}

// SearchResult is the paginated outcome of a search.
type SearchResult struct {
	Items      []*Summary
	Page       int32
	PageSize   int32
	TotalItems int64
	TotalPages int32
}

// SearchProducts is the pure, executable search specification. It filters,
// sorts, and paginates an in-memory product set. Store implementations must
// behave consistently with it. The filter is normalized first.
func SearchProducts(products []*Product, filter SearchFilter) *SearchResult {
	filter = filter.Normalize()

	matched := make([]*Product, 0, len(products))
	for _, p := range products {
		if matches(p, filter) {
			matched = append(matched, p)
		}
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if c := compare(matched[i], matched[j], filter.Sort); c != 0 {
			return c < 0
		}
		return matched[i].ID < matched[j].ID
	})

	totalItems := int64(len(matched))
	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	start := int((filter.Page - 1) * filter.PageSize)
	if start > len(matched) {
		start = len(matched)
	}
	end := start + int(filter.PageSize)
	if end > len(matched) {
		end = len(matched)
	}

	items := make([]*Summary, 0, end-start)
	for _, p := range matched[start:end] {
		items = append(items, p.Summary())
	}

	return &SearchResult{
		Items:      items,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

func matches(p *Product, f SearchFilter) bool {
	if p.Deleted {
		return false
	}
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(p.Name), q) &&
			!strings.Contains(strings.ToLower(p.Brand), q) &&
			!strings.Contains(strings.ToLower(p.CategoryName), q) {
			return false
		}
	}
	if f.CategoryID != "" && p.CategoryID != f.CategoryID {
		return false
	}
	if p.PriceMinor < f.PriceMin {
		return false
	}
	if f.PriceMax > 0 && p.PriceMinor > f.PriceMax {
		return false
	}
	if f.OnSale && p.OriginalPriceMinor <= p.PriceMinor {
		return false
	}
	if f.RatingMin > 0 && p.Rating < float64(f.RatingMin) {
		return false
	}
	return true
}

// compare orders a and b according to the canonical sort key and returns a
// negative, zero, or positive value following the convention of sort.Compare.
func compare(a, b *Product, sortKey string) int {
	switch sortKey {
	case SortPriceAsc:
		return int64Compare(a.PriceMinor, b.PriceMinor)
	case SortPriceDesc:
		return int64Compare(b.PriceMinor, a.PriceMinor)
	case SortNewest:
		return boolDescCompare(a.IsNew, b.IsNew)
	case SortRating:
		return floatDescCompare(a.Rating, b.Rating)
	case SortPopularity:
		return int64Compare(b.SoldCount, a.SoldCount)
	default:
		return int64Compare(b.SoldCount, a.SoldCount)
	}
}

func int64Compare(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func floatDescCompare(a, b float64) int {
	switch {
	case b < a:
		return -1
	case b > a:
		return 1
	default:
		return 0
	}
}

func boolDescCompare(a, b bool) int {
	switch {
	case a && !b:
		return -1
	case !a && b:
		return 1
	default:
		return 0
	}
}
