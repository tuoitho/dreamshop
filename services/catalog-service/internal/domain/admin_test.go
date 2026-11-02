package domain

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var (
	validSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)
	validSkuPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,95}$`)
)

func TestValidateAndPrepareCreateGeneratesVietnameseSlugWhenIDBlank(t *testing.T) {
	p := &Product{
		Name:         "  Tai nghe chống ồn Đỉnh Cao 2026!  ",
		Brand:        " SoundMax ",
		CategoryID:   "tai-nghe",
		CategoryName: "Tai nghe",
		Images:       []string{"/product-media/products/2026/10/img1.webp"},
		Videos:       []string{"/product-media/products/2026/10/demo.mp4"},
		Variants: []ProductVariant{
			{Label: "Đen nhám", ColorHex: "#1F2937", PriceMinor: 2590000},
		},
		Details: []string{"Pin 40 giờ"},
	}

	out, err := ValidateAndPrepareCreate(p)
	if err != nil {
		t.Fatalf("ValidateAndPrepareCreate() error = %v", err)
	}
	if !validSlugPattern.MatchString(out.ID) {
		t.Fatalf("generated ID %q does not match slug pattern", out.ID)
	}
	if !strings.HasPrefix(out.ID, "tai-nghe-chong-on-dinh-cao-2026-") {
		t.Fatalf("expected Vietnamese transliterated prefix, got %q", out.ID)
	}
	if out.PriceMinor != 2590000 || out.OriginalPriceMinor != 2590000 || out.MaxPriceMinor != 2590000 {
		t.Fatalf("expected derived prices 2590000/2590000/2590000, got %d/%d/%d", out.PriceMinor, out.OriginalPriceMinor, out.MaxPriceMinor)
	}
	if !validSkuPattern.MatchString(out.Variants[0].SkuID) || out.Variants[0].OriginalPriceMinor != 2590000 {
		t.Fatalf("expected generated SkuID and defaulted variant OriginalPriceMinor, got %+v", out.Variants[0])
	}
	if out.Name != "Tai nghe chống ồn Đỉnh Cao 2026!" || out.Brand != "SoundMax" {
		t.Fatalf("expected trimmed strings, got name=%q brand=%q", out.Name, out.Brand)
	}
}

func TestValidateVariantsDerivesMinAndMaxProductPrices(t *testing.T) {
	p := &Product{
		ID:           "dt-flagship",
		Name:         "Nova Flagship",
		Brand:        "Nova",
		CategoryID:   "dien-thoai",
		CategoryName: "Điện thoại",
		Variants: []ProductVariant{
			{Label: "16GB/512GB", PriceMinor: 20990000, OriginalPriceMinor: 22990000},
			{Label: "12GB/256GB", PriceMinor: 18990000, OriginalPriceMinor: 21990000},
			{SkuID: "dt-flagship-1tb", Label: "16GB/1TB", PriceMinor: 23990000, OriginalPriceMinor: 25990000},
		},
	}

	out, err := ValidateAndPrepareCreate(p)
	if err != nil {
		t.Fatalf("ValidateAndPrepareCreate() error = %v", err)
	}
	if out.PriceMinor != 18990000 || out.OriginalPriceMinor != 21990000 || out.MaxPriceMinor != 23990000 {
		t.Fatalf("derived product prices = (%d, %d, %d), want (18990000, 21990000, 23990000)",
			out.PriceMinor, out.OriginalPriceMinor, out.MaxPriceMinor)
	}
	for i, v := range out.Variants {
		if !validSkuPattern.MatchString(v.SkuID) {
			t.Fatalf("variant[%d].SkuID = %q invalid", i, v.SkuID)
		}
	}
	if out.Variants[2].SkuID != "dt-flagship-1tb" {
		t.Fatalf("explicit SkuID was overwritten: %q", out.Variants[2].SkuID)
	}
}

func TestValidateVariantsRejectsDuplicateLabelsInvalidPricesAndDuplicateSkus(t *testing.T) {
	baseValid := func() *Product {
		return &Product{
			ID:           "laptop-ultra-x1",
			Name:         "Laptop Ultra X1",
			Brand:        "Apex",
			CategoryID:   "laptop",
			CategoryName: "Laptop",
			Variants: []ProductVariant{
				{SkuID: "laptop-ultra-x1-v1", Label: "16GB/512GB", PriceMinor: 19990000, OriginalPriceMinor: 21990000},
			},
		}
	}

	cases := []struct {
		name   string
		mutate func(*Product)
	}{
		{name: "invalid custom id uppercase", mutate: func(p *Product) { p.ID = "Invalid_ID" }},
		{name: "variant original price below price", mutate: func(p *Product) { p.Variants[0].OriginalPriceMinor = 15000000 }},
		{name: "non-positive variant price", mutate: func(p *Product) { p.Variants[0].PriceMinor = 0 }},
		{name: "empty variants", mutate: func(p *Product) { p.Variants = nil }},
		{name: "duplicate labels case-insensitive", mutate: func(p *Product) {
			p.Variants = []ProductVariant{
				{Label: "Đen Nhám", PriceMinor: 1000000},
				{Label: "  đen nhám ", PriceMinor: 1200000},
			}
		}},
		{name: "duplicate sku ids in payload", mutate: func(p *Product) {
			p.Variants = []ProductVariant{
				{SkuID: "same-sku", Label: "Bản A", PriceMinor: 1000000},
				{SkuID: "same-sku", Label: "Bản B", PriceMinor: 1200000},
			}
		}},
		{name: "invalid sku id format", mutate: func(p *Product) { p.Variants[0].SkuID = "BAD_SKU!" }},
		{name: "invalid color hex", mutate: func(p *Product) { p.Variants[0].ColorHex = "red" }},
		{name: "disallowed media scheme", mutate: func(p *Product) { p.Images = []string{"javascript:alert(1)"} }},
		{name: "too many videos", mutate: func(p *Product) {
			p.Videos = []string{
				"https://cdn.example.com/1.mp4",
				"https://cdn.example.com/2.mp4",
				"https://cdn.example.com/3.mp4",
				"https://cdn.example.com/4.mp4",
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := baseValid()
			tc.mutate(p)
			_, err := ValidateAndPrepareCreate(p)
			var valErr *ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("expected *ValidationError, got %v", err)
			}
		})
	}
}

func TestValidateMediaUploadWhitelistsImagesAndVideos(t *testing.T) {
	imgSpec, err := ValidateMediaUpload("  Hero Banner.WEBP ", "image/webp", 5*1024*1024)
	if err != nil {
		t.Fatalf("ValidateMediaUpload(image/webp) error = %v", err)
	}
	if imgSpec.MediaType != MediaTypeImage || imgSpec.Extension != ".webp" {
		t.Fatalf("unexpected image spec: %+v", imgSpec)
	}

	vidSpec, err := ValidateMediaUpload("review.mp4", "video/mp4", 30*1024*1024)
	if err != nil {
		t.Fatalf("ValidateMediaUpload(video/mp4) error = %v", err)
	}
	if vidSpec.MediaType != MediaTypeVideo || vidSpec.Extension != ".mp4" {
		t.Fatalf("unexpected video spec: %+v", vidSpec)
	}

	rejectCases := []struct {
		name        string
		filename    string
		contentType string
		sizeBytes   int64
	}{
		{name: "svg xss vector", filename: "icon.svg", contentType: "image/svg+xml", sizeBytes: 1024},
		{name: "html payload", filename: "x.html", contentType: "text/html", sizeBytes: 512},
		{name: "zero bytes", filename: "a.png", contentType: "image/png", sizeBytes: 0},
		{name: "oversized image", filename: "a.png", contentType: "image/png", sizeBytes: 5*1024*1024 + 1},
		{name: "oversized video", filename: "v.webm", contentType: "video/webm", sizeBytes: 30*1024*1024 + 1},
	}

	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateMediaUpload(tc.filename, tc.contentType, tc.sizeBytes); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
		})
	}
}

func TestValidateMediaUploadTruncatesVietnameseFilenameOnRuneBoundary(t *testing.T) {
	longName := "a" + strings.Repeat("ệ", 260) + ".webp"
	spec, err := ValidateMediaUpload(longName, "image/webp", 1024)
	if err != nil {
		t.Fatalf("ValidateMediaUpload() error = %v", err)
	}
	if !utf8.ValidString(spec.OriginalFilename) {
		t.Fatalf("OriginalFilename contains invalid UTF-8 after truncation")
	}
	if got := utf8.RuneCountInString(spec.OriginalFilename); got != 255 {
		t.Fatalf("rune count = %d, want 255", got)
	}
}

func TestListAdminProductsSortNewestTieBreakerAndPageSize200(t *testing.T) {
	norm := (AdminSearchFilter{Page: 1, PageSize: 200}).Normalize()
	if norm.PageSize != 200 {
		t.Fatalf("Normalize().PageSize = %d, want 200", norm.PageSize)
	}

	older := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	res := ListAdminProducts([]*Product{
		{
			ID: "alpha-item", Name: "Alpha", CategoryID: "laptop", CategoryName: "Laptop",
			PriceMinor: 10000000, MaxPriceMinor: 12000000, IsNew: true, CreatedAt: older,
			Variants: []ProductVariant{{SkuID: "alpha-v1", Label: "8GB", PriceMinor: 10000000}},
		},
		{
			ID: "zeta-item", Name: "Zeta", CategoryID: "laptop", CategoryName: "Laptop",
			PriceMinor: 15000000, MaxPriceMinor: 18000000, IsNew: true, CreatedAt: newer,
			Variants: []ProductVariant{{SkuID: "zeta-v1", Label: "16GB", PriceMinor: 15000000}},
		},
	}, AdminSearchFilter{Sort: SortNewest, Page: 1, PageSize: 20})

	if len(res.Items) != 2 || res.Items[0].ID != "zeta-item" {
		t.Fatalf("expected newer CreatedAt first under SortNewest, got %+v", res.Items)
	}
	if res.Items[0].MaxPriceMinor != 18000000 || len(res.Items[0].SkuIDs) != 1 || res.Items[0].SkuIDs[0] != "zeta-v1" {
		t.Fatalf("expected MaxPriceMinor and SkuIDs on AdminSummary, got %+v", res.Items[0])
	}
}

func TestListInventoryCatalogItemsMatchesSkuAndVariantLabel(t *testing.T) {
	products := []*Product{
		{
			ID: "dt-1", Name: "Nova X15", Brand: "Nova", CategoryID: "dien-thoai", CategoryName: "Điện thoại",
			Variants: []ProductVariant{
				{SkuID: "dt-1-v1", Label: "12GB/256GB", PriceMinor: 18990000},
				{SkuID: "dt-1-v3", Label: "16GB/1TB Titan", PriceMinor: 23990000},
			},
		},
		{
			ID: "lt-1", Name: "UltraBook", Brand: "MaxBook", CategoryID: "laptop", CategoryName: "Laptop",
			Variants: []ProductVariant{{SkuID: "lt-1-v1", Label: "16GB/1TB", PriceMinor: 26990000}},
		},
	}

	bySku := ListInventoryCatalogItems(products, InventoryCatalogFilter{Query: "dt-1-v3", IncludeDeleted: true})
	if len(bySku) != 1 || bySku[0].ID != "dt-1" || len(bySku[0].Variants) != 2 {
		t.Fatalf("expected dt-1 with 2 variants on SKU query, got %+v", bySku)
	}

	byLabel := ListInventoryCatalogItems(products, InventoryCatalogFilter{Query: "titan", IncludeDeleted: true})
	if len(byLabel) != 1 || byLabel[0].ID != "dt-1" {
		t.Fatalf("expected dt-1 on variant label query, got %+v", byLabel)
	}
}
