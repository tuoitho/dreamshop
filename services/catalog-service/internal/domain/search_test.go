package domain

import (
	"math"
	"testing"
)

func testProducts() []*Product {
	return []*Product{
		{
			ID:                 "p-1",
			Name:               "Điện thoại Nova X15 Pro",
			Brand:              "Nova",
			CategoryID:         "dien-thoai",
			CategoryName:       "Điện thoại",
			PriceMinor:         18990000,
			OriginalPriceMinor: 21990000,
			Rating:             4.9,
			ReviewCount:        1284,
			SoldCount:          2310,
			IsNew:              false,
			Badge:              "hot",
			Featured:           true,
			ImageHue:           []uint32{220, 260},
		},
		{
			ID:                 "p-2",
			Name:               "Điện thoại Lotus 12A",
			Brand:              "Lotus",
			CategoryID:         "dien-thoai",
			CategoryName:       "Điện thoại",
			PriceMinor:         5490000,
			OriginalPriceMinor: 6990000,
			Rating:             4.7,
			ReviewCount:        3521,
			SoldCount:          8920,
			IsNew:              false,
			Badge:              "sale",
			ImageHue:           []uint32{150, 190},
		},
		{
			ID:                 "p-3",
			Name:               "Laptop UltraBook 14 Air",
			Brand:              "MaxBook",
			CategoryID:         "laptop",
			CategoryName:       "Laptop",
			PriceMinor:         26990000,
			OriginalPriceMinor: 29990000,
			Rating:             4.8,
			ReviewCount:        763,
			SoldCount:          1240,
			IsNew:              true,
			Featured:           true,
			ImageHue:           []uint32{0, 30},
		},
		{
			ID:                 "p-4",
			Name:               "Áo sơ mi Unisex Cotton 260",
			Brand:              "UrbanWear",
			CategoryID:         "thoi-trang",
			CategoryName:       "Thời trang",
			PriceMinor:         399000,
			OriginalPriceMinor: 550000,
			Rating:             4.5,
			ReviewCount:        4210,
			SoldCount:          21300,
			IsNew:              false,
			Badge:              "sale",
			ImageHue:           []uint32{200, 230},
		},
		{
			ID:                 "p-5",
			Name:               "Đồng hồ cơ Zenith 42mm",
			Brand:              "Zenith",
			CategoryID:         "dong-ho",
			CategoryName:       "Đồng hồ",
			PriceMinor:         8790000,
			OriginalPriceMinor: 8790000,
			Rating:             4.9,
			ReviewCount:        198,
			SoldCount:          450,
			IsNew:              true,
			Badge:              "new",
			ImageHue:           []uint32{30, 60},
		},
	}
}

func ids(items []*Summary) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		out = append(out, s.ID)
	}
	return out
}

func TestSearchProductsExcludesDeletedProducts(t *testing.T) {
	products := []*Product{
		{ID: "active", Name: "Active", Brand: "Nova", CategoryName: "Phone", PriceMinor: 1},
		{ID: "deleted", Name: "Deleted", Brand: "Nova", CategoryName: "Phone", PriceMinor: 1, Deleted: true},
	}

	result := SearchProducts(products, SearchFilter{})
	if result.TotalItems != 1 || result.Items[0].ID != "active" {
		t.Fatalf("expected only active product, got %+v", result)
	}
}

func TestSearchProductsAllWithDefaultSort(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{})

	if result.TotalItems != int64(len(products)) {
		t.Fatalf("expected %d total items, got %d", len(products), result.TotalItems)
	}
	// Default sort is popularity (soldCount desc): p-4 (21300), p-2 (8920),
	// p-1 (2310), p-3 (1240), p-5 (450).
	want := []string{"p-4", "p-2", "p-1", "p-3", "p-5"}
	got := ids(result.Items)
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
	if result.Page != 1 || result.PageSize != 20 || result.TotalPages != 1 {
		t.Fatalf("unexpected pagination metadata: %+v", result)
	}
}

func TestSearchProductsEmptyQueryMatchesAll(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{Query: "   "})
	if result.TotalItems != int64(len(products)) {
		t.Fatalf("expected all products for empty/blank query, got %d", result.TotalItems)
	}
}

func TestSearchProductsQueryMatchesNameBrandAndCategory(t *testing.T) {
	products := testProducts()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "name lowercase", query: "nova", want: []string{"p-1"}},
		{name: "name unicode lowercase", query: "điện thoại", want: []string{"p-2", "p-1"}},
		{name: "brand", query: "maxbook", want: []string{"p-3"}},
		{name: "category name", query: "đồng hồ", want: []string{"p-5"}},
		{name: "no match", query: "nonexistent", want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := SearchProducts(products, SearchFilter{Query: tc.query})
			if !equalStrings(ids(result.Items), tc.want) {
				t.Fatalf("query %q: expected %v, got %v", tc.query, tc.want, ids(result.Items))
			}
		})
	}
}

func TestSearchProductsCategoryFilter(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{CategoryID: "dien-thoai"})
	if result.TotalItems != 2 {
		t.Fatalf("expected 2 phones, got %d", result.TotalItems)
	}
	if !equalStrings(ids(result.Items), []string{"p-2", "p-1"}) {
		t.Fatalf("unexpected items: %v", ids(result.Items))
	}
}

func TestSearchProductsPriceBounds(t *testing.T) {
	products := testProducts()

	// PriceMin only (>= 5.490.000).
	result := SearchProducts(products, SearchFilter{PriceMin: 5490000})
	if result.TotalItems != 4 {
		t.Fatalf("priceMin 5490000: expected 4, got %d", result.TotalItems)
	}

	// PriceMax only (<= 5.490.000); inclusive.
	result = SearchProducts(products, SearchFilter{PriceMax: 5490000})
	if result.TotalItems != 2 {
		t.Fatalf("priceMax 5490000: expected 2, got %d", result.TotalItems)
	}

	// Both bounds inclusive.
	result = SearchProducts(products, SearchFilter{PriceMin: 4000000, PriceMax: 9000000})
	if result.TotalItems != 2 {
		t.Fatalf("bounded range: expected 2, got %d", result.TotalItems)
	}
}

func TestSearchProductsPriceMaxZeroIsUnbounded(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{PriceMax: 0})
	if result.TotalItems != int64(len(products)) {
		t.Fatalf("priceMax 0 must be unbounded, got %d items", result.TotalItems)
	}
}

func TestSearchProductsOnSale(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{OnSale: true})
	// p-5 has originalPrice == price, so it is excluded.
	if result.TotalItems != 4 {
		t.Fatalf("expected 4 on-sale products, got %d", result.TotalItems)
	}
	for _, s := range result.Items {
		if s.ID == "p-5" {
			t.Fatalf("p-5 is not on sale but was returned")
		}
	}
}

func TestSearchProductsRatingMin(t *testing.T) {
	products := testProducts()
	result := SearchProducts(products, SearchFilter{RatingMin: 5})
	if result.TotalItems != 0 {
		t.Fatalf("rating >= 5 should match none, got %d", result.TotalItems)
	}
	result = SearchProducts(products, SearchFilter{RatingMin: 4})
	if result.TotalItems != 5 {
		t.Fatalf("rating >= 4 should match all, got %d", result.TotalItems)
	}
	result = SearchProducts(products, SearchFilter{RatingMin: 0})
	if result.TotalItems != 5 {
		t.Fatalf("ratingMin 0 should be unset, got %d", result.TotalItems)
	}
}

func TestSearchProductsSortKeys(t *testing.T) {
	products := testProducts()

	cases := []struct {
		sort string
		want []string
	}{
		{sort: "pho-bien", want: []string{"p-4", "p-2", "p-1", "p-3", "p-5"}},
		{sort: "gia-thap", want: []string{"p-4", "p-2", "p-5", "p-1", "p-3"}},
		{sort: "gia-cao", want: []string{"p-3", "p-1", "p-5", "p-2", "p-4"}},
		{sort: "moi-nhat", want: []string{"p-3", "p-5", "p-1", "p-2", "p-4"}},
		{sort: "danh-gia", want: []string{"p-1", "p-5", "p-3", "p-2", "p-4"}},
		{sort: "unknown-key", want: []string{"p-4", "p-2", "p-1", "p-3", "p-5"}},
		{sort: "", want: []string{"p-4", "p-2", "p-1", "p-3", "p-5"}},
	}

	for _, tc := range cases {
		t.Run(tc.sort, func(t *testing.T) {
			result := SearchProducts(products, SearchFilter{Sort: tc.sort})
			if !equalStrings(ids(result.Items), tc.want) {
				t.Fatalf("sort %q: expected %v, got %v", tc.sort, tc.want, ids(result.Items))
			}
		})
	}
}

func TestSearchProductsPagination(t *testing.T) {
	products := testProducts()

	// Page 1 of size 2, popularity order.
	result := SearchProducts(products, SearchFilter{Page: 1, PageSize: 2})
	if !equalStrings(ids(result.Items), []string{"p-4", "p-2"}) {
		t.Fatalf("page 1: expected [p-4 p-2], got %v", ids(result.Items))
	}
	if result.Page != 1 || result.PageSize != 2 || result.TotalItems != 5 || result.TotalPages != 3 {
		t.Fatalf("unexpected metadata: %+v", result)
	}

	// Page 3 of size 2 has one item.
	result = SearchProducts(products, SearchFilter{Page: 3, PageSize: 2})
	if !equalStrings(ids(result.Items), []string{"p-5"}) {
		t.Fatalf("page 3: expected [p-5], got %v", ids(result.Items))
	}

	// Out-of-range page returns empty items but keeps totals.
	result = SearchProducts(products, SearchFilter{Page: 99, PageSize: 2})
	if len(result.Items) != 0 {
		t.Fatalf("page 99 should be empty, got %v", ids(result.Items))
	}
	if result.TotalItems != 5 || result.TotalPages != 3 {
		t.Fatalf("page 99 must keep totals, got %+v", result)
	}
}

func TestSearchProductsPaginationOverflowDoesNotPanic(t *testing.T) {
	products := testProducts()

	for _, page := range []int32{50_000_000, math.MaxInt32} {
		result := SearchProducts(products, SearchFilter{Page: page, PageSize: 50})
		if len(result.Items) != 0 {
			t.Fatalf("page %d: expected empty items, got %v", page, ids(result.Items))
		}
		if result.Page != page || result.PageSize != 50 || result.TotalItems != 5 || result.TotalPages != 1 {
			t.Fatalf("page %d: unexpected metadata: %+v", page, result)
		}
	}
}

func TestSearchProductsEmptyResult(t *testing.T) {
	result := SearchProducts(nil, SearchFilter{Page: 2, PageSize: 10})
	if len(result.Items) != 0 || result.TotalItems != 0 || result.TotalPages != 0 {
		t.Fatalf("expected empty result with zero totals, got %+v", result)
	}
}

func TestSearchFilterNormalize(t *testing.T) {
	cases := []struct {
		name  string
		input SearchFilter
		want  SearchFilter
	}{
		{
			name:  "defaults",
			input: SearchFilter{},
			want:  SearchFilter{Page: 1, PageSize: 20, Sort: "pho-bien"},
		},
		{
			name:  "clamp page and pageSize",
			input: SearchFilter{Page: 0, PageSize: 500, Sort: "nope"},
			want:  SearchFilter{Page: 1, PageSize: 50, Sort: "pho-bien"},
		},
		{
			name:  "preserve valid values and priceMax zero",
			input: SearchFilter{Page: 3, PageSize: 8, Sort: "gia-thap", PriceMax: 0, Query: "  nova  "},
			want:  SearchFilter{Page: 3, PageSize: 8, Sort: "gia-thap", PriceMax: 0, Query: "nova"},
		},
		{
			name:  "preserve positive priceMax",
			input: SearchFilter{PageSize: 10, PriceMax: 100000},
			want:  SearchFilter{Page: 1, PageSize: 10, Sort: "pho-bien", PriceMax: 100000},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.Normalize()
			if got != tc.want {
				t.Fatalf("normalize: expected %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestSortKeyResolver(t *testing.T) {
	cases := map[string]string{
		"pho-bien": "pho-bien",
		"gia-thap": "gia-thap",
		"gia-cao":  "gia-cao",
		"moi-nhat": "moi-nhat",
		"danh-gia": "danh-gia",
		"":         "pho-bien",
		"random":   "pho-bien",
	}
	for input, want := range cases {
		if got := SortKey(input); got != want {
			t.Fatalf("SortKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProductSummaryProjection(t *testing.T) {
	p := &Product{
		ID:                 "p-1",
		Name:               "Nova",
		Brand:              "Nova",
		CategoryID:         "dien-thoai",
		CategoryName:       "Điện thoại",
		PriceMinor:         18990000,
		OriginalPriceMinor: 21990000,
		Images:             []string{"a", "b"},
		Rating:             4.9,
		ReviewCount:        1,
		SoldCount:          2,
		IsNew:              false,
		Badge:              "hot",
		Featured:           true,
		Emoji:              "📱",
		ImageHue:           []uint32{220, 260},
		Variants:           []ProductVariant{{Label: "Đen", ColorHex: "#111827"}},
		Details:            []string{"detail"},
	}
	s := p.Summary()
	if s.ID != p.ID || s.PriceMinor != p.PriceMinor || s.Name != p.Name {
		t.Fatalf("summary did not project core fields: %+v", s)
	}
	if len(s.Images) != 2 || len(s.ImageHue) != 2 {
		t.Fatalf("summary must keep images and imageHue")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
