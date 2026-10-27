package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MediaTypeImage = "IMAGE"
	MediaTypeVideo = "VIDEO"

	MediaStatusPending  = "PENDING"
	MediaStatusAttached = "ATTACHED"
	MediaStatusOrphaned = "ORPHANED"

	MaxImageSizeBytes = 5 * 1024 * 1024
	MaxVideoSizeBytes = 30 * 1024 * 1024

	maxImagesCount   = 10
	maxVideosCount   = 3
	maxVariantsCount = 20
	maxDetailsCount  = 20

	AdminStatusAll     = "all"
	AdminStatusActive  = "active"
	AdminStatusDeleted = "deleted"

	AdminSortUpdatedDesc = "updated-desc"
	adminMaxPageSize     = 200
)

var (
	productIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)
	colorHexRegex  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// ValidationError carries field-level validation failures.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// MediaAsset tracks the lifecycle of an object in the product-media bucket.
type MediaAsset struct {
	ID               string
	Bucket           string
	ObjectKey        string
	PublicURL        string
	MediaType        string
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	Status           string
	UploadedBy       string
	ProductID        *string
	CreatedAt        time.Time
	AttachedAt       *time.Time
	ExpiresAt        *time.Time
}

// MediaUploadSpec describes a validated media upload request.
type MediaUploadSpec struct {
	MediaType        string
	ContentType      string
	Extension        string
	OriginalFilename string
	SizeBytes        int64
}

// AdminSummary is the administrative table projection of a Product.
type AdminSummary struct {
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
	Deleted            bool
	VariantCount       int32
	VideoCount         int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CategoryMetric summarizes active products per category.
type CategoryMetric struct {
	CategoryID         string
	CategoryName       string
	ActiveProductCount int64
}

// InventoryCatalogItem provides lightweight catalog enrichment for Inventory.
type InventoryCatalogItem struct {
	ID           string
	Name         string
	Brand        string
	CategoryID   string
	CategoryName string
	ImageURL     string
	Emoji        string
	ImageHue     []uint32
	Deleted      bool
}

// AdminSearchFilter carries admin list filter criteria.
type AdminSearchFilter struct {
	Query      string
	CategoryID string
	Status     string
	Sort       string
	Page       int32
	PageSize   int32
}

func (f AdminSearchFilter) Normalize() AdminSearchFilter {
	f.Query = strings.TrimSpace(f.Query)
	f.CategoryID = strings.TrimSpace(f.CategoryID)
	switch strings.ToLower(strings.TrimSpace(f.Status)) {
	case AdminStatusActive:
		f.Status = AdminStatusActive
	case AdminStatusDeleted:
		f.Status = AdminStatusDeleted
	default:
		f.Status = AdminStatusAll
	}
	switch f.Sort {
	case SortPopularity, SortPriceAsc, SortPriceDesc, SortNewest, SortRating, AdminSortUpdatedDesc:
	default:
		f.Sort = AdminSortUpdatedDesc
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultPageSize
	}
	if f.PageSize > adminMaxPageSize {
		f.PageSize = adminMaxPageSize
	}
	return f
}

// AdminSearchResult is the paginated outcome of ListAdmin.
type AdminSearchResult struct {
	Items        []*AdminSummary
	Page         int32
	PageSize     int32
	TotalItems   int64
	TotalPages   int32
	ActiveCount  int64
	DeletedCount int64
	Categories   []*CategoryMetric
}

// InventoryCatalogFilter filters ListInventoryItems.
type InventoryCatalogFilter struct {
	Query          string
	CategoryID     string
	IncludeDeleted bool
}

// ValidateMediaUpload checks MIME whitelist and size bounds for image/video uploads.
func ValidateMediaUpload(filename, contentType string, sizeBytes int64) (*MediaUploadSpec, error) {
	cleanFilename := strings.TrimSpace(filename)
	if utf8.RuneCountInString(cleanFilename) > 255 {
		cleanFilename = string([]rune(cleanFilename)[:255])
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if sizeBytes <= 0 {
		return nil, &ValidationError{Field: "size_bytes", Message: "must be positive"}
	}

	switch ct {
	case "image/jpeg":
		if sizeBytes > MaxImageSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "image must not exceed 5 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeImage, ContentType: ct, Extension: ".jpg", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	case "image/png":
		if sizeBytes > MaxImageSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "image must not exceed 5 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeImage, ContentType: ct, Extension: ".png", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	case "image/webp":
		if sizeBytes > MaxImageSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "image must not exceed 5 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeImage, ContentType: ct, Extension: ".webp", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	case "image/avif":
		if sizeBytes > MaxImageSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "image must not exceed 5 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeImage, ContentType: ct, Extension: ".avif", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	case "video/mp4":
		if sizeBytes > MaxVideoSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "video must not exceed 30 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeVideo, ContentType: ct, Extension: ".mp4", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	case "video/webm":
		if sizeBytes > MaxVideoSizeBytes {
			return nil, &ValidationError{Field: "size_bytes", Message: "video must not exceed 30 MB"}
		}
		return &MediaUploadSpec{MediaType: MediaTypeVideo, ContentType: ct, Extension: ".webm", OriginalFilename: cleanFilename, SizeBytes: sizeBytes}, nil
	default:
		return nil, &ValidationError{
			Field:   "content_type",
			Message: "unsupported content_type; allowed: image/jpeg, image/png, image/webp, image/avif, video/mp4, video/webm",
		}
	}
}

// ValidateAndPrepareCreate validates and normalizes a Product for creation,
// generating a URL-safe slug ID when ID is empty.
func ValidateAndPrepareCreate(in *Product) (*Product, error) {
	if in == nil {
		return nil, &ValidationError{Message: "product is required"}
	}
	cp := *in
	cp.ID = strings.TrimSpace(cp.ID)
	cp.Name = strings.TrimSpace(cp.Name)
	if err := validateLength("name", cp.Name, 1, 200); err != nil {
		return nil, err
	}
	if cp.ID == "" {
		slug, err := generateProductSlug(cp.Name)
		if err != nil {
			return nil, err
		}
		cp.ID = slug
	} else if !productIDRegex.MatchString(cp.ID) {
		return nil, &ValidationError{Field: "id", Message: "must match ^[a-z0-9][a-z0-9-]{1,63}$"}
	}
	return validateCommonFields(&cp)
}

// ValidateAndPrepareUpdate validates and normalizes an existing Product update.
func ValidateAndPrepareUpdate(in *Product) (*Product, error) {
	if in == nil {
		return nil, &ValidationError{Message: "product is required"}
	}
	cp := *in
	cp.ID = strings.TrimSpace(cp.ID)
	if cp.ID == "" {
		return nil, &ValidationError{Field: "id", Message: "is required"}
	}
	if len(cp.ID) > 64 {
		return nil, &ValidationError{Field: "id", Message: "must not exceed 64 characters"}
	}
	cp.Name = strings.TrimSpace(cp.Name)
	if err := validateLength("name", cp.Name, 1, 200); err != nil {
		return nil, err
	}
	return validateCommonFields(&cp)
}

func validateCommonFields(p *Product) (*Product, error) {
	p.ShortDescription = strings.TrimSpace(p.ShortDescription)
	if utf8.RuneCountInString(p.ShortDescription) > 500 {
		return nil, &ValidationError{Field: "short_description", Message: "must not exceed 500 characters"}
	}
	p.Description = strings.TrimSpace(p.Description)
	if utf8.RuneCountInString(p.Description) > 8000 {
		return nil, &ValidationError{Field: "description", Message: "must not exceed 8000 characters"}
	}
	p.Brand = strings.TrimSpace(p.Brand)
	if err := validateLength("brand", p.Brand, 1, 100); err != nil {
		return nil, err
	}
	p.CategoryID = strings.TrimSpace(p.CategoryID)
	if !productIDRegex.MatchString(p.CategoryID) {
		return nil, &ValidationError{Field: "category_id", Message: "must match ^[a-z0-9][a-z0-9-]{1,63}$"}
	}
	p.CategoryName = strings.TrimSpace(p.CategoryName)
	if err := validateLength("category_name", p.CategoryName, 1, 100); err != nil {
		return nil, err
	}
	if p.PriceMinor <= 0 {
		return nil, &ValidationError{Field: "price", Message: "must be positive"}
	}
	if p.OriginalPriceMinor < 0 || (p.OriginalPriceMinor > 0 && p.OriginalPriceMinor < p.PriceMinor) {
		return nil, &ValidationError{Field: "original_price", Message: "must be 0 or greater than or equal to price"}
	}
	if p.OriginalPriceMinor == 0 {
		p.OriginalPriceMinor = p.PriceMinor
	}
	p.Badge = strings.TrimSpace(p.Badge)
	if utf8.RuneCountInString(p.Badge) > 40 {
		return nil, &ValidationError{Field: "badge", Message: "must not exceed 40 characters"}
	}
	p.Emoji = strings.TrimSpace(p.Emoji)
	if utf8.RuneCountInString(p.Emoji) > 16 {
		return nil, &ValidationError{Field: "emoji", Message: "must not exceed 16 characters"}
	}
	if len(p.ImageHue) > 4 {
		return nil, &ValidationError{Field: "image_hue", Message: "must not exceed 4 values"}
	}
	for _, h := range p.ImageHue {
		if h > 360 {
			return nil, &ValidationError{Field: "image_hue", Message: "values must be between 0 and 360"}
		}
	}

	images, err := sanitizeMediaURLs("images", p.Images, maxImagesCount)
	if err != nil {
		return nil, err
	}
	p.Images = images

	videos, err := sanitizeMediaURLs("videos", p.Videos, maxVideosCount)
	if err != nil {
		return nil, err
	}
	p.Videos = videos

	if len(p.Variants) == 0 || len(p.Variants) > maxVariantsCount {
		return nil, &ValidationError{Field: "variants", Message: "must contain between 1 and 20 variants"}
	}
	cleanVariants := make([]ProductVariant, 0, len(p.Variants))
	for _, v := range p.Variants {
		label := strings.TrimSpace(v.Label)
		if err := validateLength("variants.label", label, 1, 80); err != nil {
			return nil, err
		}
		hexColor := strings.TrimSpace(v.ColorHex)
		if hexColor != "" && !colorHexRegex.MatchString(hexColor) {
			return nil, &ValidationError{Field: "variants.color_hex", Message: "must match ^#[0-9a-fA-F]{6}$"}
		}
		cleanVariants = append(cleanVariants, ProductVariant{Label: label, ColorHex: hexColor})
	}
	p.Variants = cleanVariants

	if len(p.Details) > maxDetailsCount {
		return nil, &ValidationError{Field: "details", Message: "must not exceed 20 entries"}
	}
	cleanDetails := make([]string, 0, len(p.Details))
	for _, d := range p.Details {
		trimmed := strings.TrimSpace(d)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > 300 {
			return nil, &ValidationError{Field: "details", Message: "entry must not exceed 300 characters"}
		}
		cleanDetails = append(cleanDetails, trimmed)
	}
	p.Details = cleanDetails

	return p, nil
}

func validateLength(field, value string, minRunes, maxRunes int) error {
	n := utf8.RuneCountInString(value)
	if n < minRunes || n > maxRunes {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be between %d and %d characters", minRunes, maxRunes),
		}
	}
	return nil
}

func sanitizeMediaURLs(field string, urls []string, maxCount int) ([]string, error) {
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if len(u) > 1024 || (!strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "/product-media/")) {
			return nil, &ValidationError{
				Field:   field,
				Message: "must be http(s):// or /product-media/ URL",
			}
		}
		out = append(out, u)
	}
	if len(out) > maxCount {
		return nil, &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must not exceed %d items", maxCount),
		}
	}
	return out, nil
}

// NewUUIDv4 generates an RFC 4122 version 4 UUID string using crypto/rand.
func NewUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func generateProductSlug(name string) (string, error) {
	base := slugifyVietnamese(name)
	if base == "" {
		base = "san-pham"
	}
	if len(base) > 48 {
		base = strings.TrimRight(base[:48], "-")
	}
	var suffix [3]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate slug suffix: %w", err)
	}
	return base + "-" + hex.EncodeToString(suffix[:]), nil
}

var vietnameseRuneMap = map[rune]rune{
	'à': 'a', 'á': 'a', 'ạ': 'a', 'ả': 'a', 'ã': 'a',
	'â': 'a', 'ầ': 'a', 'ấ': 'a', 'ậ': 'a', 'ẩ': 'a', 'ẫ': 'a',
	'ă': 'a', 'ằ': 'a', 'ắ': 'a', 'ặ': 'a', 'ẳ': 'a', 'ẵ': 'a',
	'è': 'e', 'é': 'e', 'ẹ': 'e', 'ẻ': 'e', 'ẽ': 'e',
	'ê': 'e', 'ề': 'e', 'ế': 'e', 'ệ': 'e', 'ể': 'e', 'ễ': 'e',
	'ì': 'i', 'í': 'i', 'ị': 'i', 'ỉ': 'i', 'ĩ': 'i',
	'ò': 'o', 'ó': 'o', 'ọ': 'o', 'ỏ': 'o', 'õ': 'o',
	'ô': 'o', 'ồ': 'o', 'ố': 'o', 'ộ': 'o', 'ổ': 'o', 'ỗ': 'o',
	'ơ': 'o', 'ờ': 'o', 'ớ': 'o', 'ợ': 'o', 'ở': 'o', 'ỡ': 'o',
	'ù': 'u', 'ú': 'u', 'ụ': 'u', 'ủ': 'u', 'ũ': 'u',
	'ư': 'u', 'ừ': 'u', 'ứ': 'u', 'ự': 'u', 'ử': 'u', 'ữ': 'u',
	'ỳ': 'y', 'ý': 'y', 'ỵ': 'y', 'ỷ': 'y', 'ỹ': 'y',
	'đ': 'd',
}

func slugifyVietnamese(input string) string {
	lower := strings.ToLower(input)
	var b strings.Builder
	prevDash := true
	for _, r := range lower {
		if mapped, ok := vietnameseRuneMap[r]; ok {
			r = mapped
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ListAdminProducts is the pure in-memory reference specification for ListAdmin.
func ListAdminProducts(products []*Product, filter AdminSearchFilter) *AdminSearchResult {
	filter = filter.Normalize()

	var activeCount, deletedCount int64
	catMap := make(map[string]*CategoryMetric)
	matched := make([]*Product, 0, len(products))

	q := strings.ToLower(filter.Query)
	for _, p := range products {
		if p.Deleted {
			deletedCount++
		} else {
			activeCount++
			cm, ok := catMap[p.CategoryID]
			if !ok {
				cm = &CategoryMetric{CategoryID: p.CategoryID, CategoryName: p.CategoryName}
				catMap[p.CategoryID] = cm
			}
			cm.ActiveProductCount++
		}

		if filter.Status == AdminStatusActive && p.Deleted {
			continue
		}
		if filter.Status == AdminStatusDeleted && !p.Deleted {
			continue
		}
		if filter.CategoryID != "" && p.CategoryID != filter.CategoryID {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(p.ID), q) &&
			!strings.Contains(strings.ToLower(p.Name), q) &&
			!strings.Contains(strings.ToLower(p.Brand), q) &&
			!strings.Contains(strings.ToLower(p.CategoryName), q) {
			continue
		}
		matched = append(matched, p)
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if filter.Sort == AdminSortUpdatedDesc {
			if !matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
				return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
			}
			return matched[i].ID < matched[j].ID
		}
		if filter.Sort == SortNewest {
			if c := boolDescCompare(matched[i].IsNew, matched[j].IsNew); c != 0 {
				return c < 0
			}
			if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
				return matched[i].CreatedAt.After(matched[j].CreatedAt)
			}
			return matched[i].ID < matched[j].ID
		}
		if c := compare(matched[i], matched[j], filter.Sort); c != 0 {
			return c < 0
		}
		return matched[i].ID < matched[j].ID
	})

	categories := make([]*CategoryMetric, 0, len(catMap))
	for _, cm := range catMap {
		categories = append(categories, cm)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].CategoryID < categories[j].CategoryID
	})

	totalItems := int64(len(matched))
	totalPages := int32(0)
	if totalItems > 0 {
		totalPages = int32((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}
	offset := int(filter.Page-1) * int(filter.PageSize)
	if offset > len(matched) {
		offset = len(matched)
	}
	end := offset + int(filter.PageSize)
	if end > len(matched) {
		end = len(matched)
	}

	items := make([]*AdminSummary, 0, end-offset)
	for _, p := range matched[offset:end] {
		items = append(items, &AdminSummary{
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
			Deleted:            p.Deleted,
			VariantCount:       int32(len(p.Variants)),
			VideoCount:         int32(len(p.Videos)),
			CreatedAt:          p.CreatedAt,
			UpdatedAt:          p.UpdatedAt,
		})
	}

	return &AdminSearchResult{
		Items:        items,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
		TotalItems:   totalItems,
		TotalPages:   totalPages,
		ActiveCount:  activeCount,
		DeletedCount: deletedCount,
		Categories:   categories,
	}
}

// ListInventoryCatalogItems is the pure in-memory reference for ListInventoryItems.
func ListInventoryCatalogItems(products []*Product, filter InventoryCatalogFilter) []*InventoryCatalogItem {
	q := strings.ToLower(strings.TrimSpace(filter.Query))
	catID := strings.TrimSpace(filter.CategoryID)
	out := make([]*InventoryCatalogItem, 0, len(products))
	for _, p := range products {
		if !filter.IncludeDeleted && p.Deleted {
			continue
		}
		if catID != "" && p.CategoryID != catID {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(p.ID), q) &&
			!strings.Contains(strings.ToLower(p.Name), q) &&
			!strings.Contains(strings.ToLower(p.Brand), q) &&
			!strings.Contains(strings.ToLower(p.CategoryName), q) {
			continue
		}
		var primaryImg string
		if len(p.Images) > 0 {
			primaryImg = p.Images[0]
		}
		out = append(out, &InventoryCatalogItem{
			ID:           p.ID,
			Name:         p.Name,
			Brand:        p.Brand,
			CategoryID:   p.CategoryID,
			CategoryName: p.CategoryName,
			ImageURL:     primaryImg,
			Emoji:        p.Emoji,
			ImageHue:     p.ImageHue,
			Deleted:      p.Deleted,
		})
	}
	return out
}
