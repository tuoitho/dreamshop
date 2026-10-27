package domain

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var validSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

func TestValidateAndPrepareCreateGeneratesVietnameseSlugWhenIDBlank(t *testing.T) {
	p := &Product{
		Name:         "  Tai nghe chống ồn Đỉnh Cao 2026!  ",
		Brand:        " SoundMax ",
		CategoryID:   "tai-nghe",
		CategoryName: "Tai nghe",
		PriceMinor:   2590000,
		Images:       []string{"/product-media/products/2026/10/img1.webp"},
		Videos:       []string{"/product-media/products/2026/10/demo.mp4"},
		Variants: []ProductVariant{
			{Label: "Đen nhám", ColorHex: "#1F2937"},
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
	if out.OriginalPriceMinor != 2590000 {
		t.Fatalf("expected OriginalPriceMinor defaulted to PriceMinor (2590000), got %d", out.OriginalPriceMinor)
	}
	if out.Name != "Tai nghe chống ồn Đỉnh Cao 2026!" || out.Brand != "SoundMax" {
		t.Fatalf("expected trimmed strings, got name=%q brand=%q", out.Name, out.Brand)
	}
}

func TestValidateAndPrepareCreateRejectsInvalidFields(t *testing.T) {
	baseValid := func() *Product {
		return &Product{
			ID:           "laptop-ultra-x1",
			Name:         "Laptop Ultra X1",
			Brand:        "Apex",
			CategoryID:   "laptop",
			CategoryName: "Laptop",
			PriceMinor:   19990000,
			Variants:     []ProductVariant{{Label: "16GB/512GB"}},
		}
	}

	cases := []struct {
		name   string
		mutate func(*Product)
	}{
		{name: "invalid custom id uppercase", mutate: func(p *Product) { p.ID = "Invalid_ID" }},
		{name: "original price below price", mutate: func(p *Product) { p.OriginalPriceMinor = 15000000 }},
		{name: "non-positive price", mutate: func(p *Product) { p.PriceMinor = 0 }},
		{name: "empty variants", mutate: func(p *Product) { p.Variants = nil }},
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
		{ID: "alpha-item", Name: "Alpha", CategoryID: "laptop", CategoryName: "Laptop", IsNew: true, CreatedAt: older},
		{ID: "zeta-item", Name: "Zeta", CategoryID: "laptop", CategoryName: "Laptop", IsNew: true, CreatedAt: newer},
	}, AdminSearchFilter{Sort: SortNewest, Page: 1, PageSize: 20})

	if len(res.Items) != 2 || res.Items[0].ID != "zeta-item" {
		t.Fatalf("expected newer CreatedAt first under SortNewest, got %+v", res.Items)
	}
}
