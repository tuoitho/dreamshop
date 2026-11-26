package domain

import (
	"strings"
	"testing"
)

func TestValidateCreateReview(t *testing.T) {
	valid, err := ValidateCreateReview(CreateReviewInput{
		UserID:     "  u-1 ",
		ProductID:  " dt-1 ",
		OrderID:    " ord-1 ",
		AuthorName: "  Nguyen Van A  ",
		Rating:     5,
		Comment:    "  San pham tot  ",
		Images:     []string{"/product-media/reviews/2026/11/a.png"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid.UserID != "u-1" || valid.ProductID != "dt-1" || valid.OrderID != "ord-1" || valid.AuthorName != "Nguyen Van A" || valid.Comment != "San pham tot" {
		t.Fatalf("unexpected normalized values: %+v", valid)
	}

	if _, err := ValidateCreateReview(CreateReviewInput{UserID: "u-1", ProductID: "dt-1", OrderID: "ord-1", Rating: 0}); err == nil {
		t.Fatal("expected error for rating 0")
	}
	if _, err := ValidateCreateReview(CreateReviewInput{UserID: "u-1", ProductID: "dt-1", OrderID: "ord-1", Rating: 6}); err == nil {
		t.Fatal("expected error for rating 6")
	}
	if _, err := ValidateCreateReview(CreateReviewInput{
		UserID:    "u-1",
		ProductID: "dt-1",
		OrderID:   "ord-1",
		Rating:    4,
		Images: []string{
			"https://cdn.example.com/1.png",
			"https://cdn.example.com/2.png",
			"https://cdn.example.com/3.png",
			"https://cdn.example.com/4.png",
			"https://cdn.example.com/5.png",
			"https://cdn.example.com/6.png",
		},
	}); err == nil {
		t.Fatal("expected error for >5 review images")
	}
	if _, err := ValidateCreateReview(CreateReviewInput{
		UserID:    "u-1",
		ProductID: "dt-1",
		OrderID:   "ord-1",
		Rating:    4,
		Comment:   strings.Repeat("a", MaxReviewCommentRunes+1),
	}); err == nil {
		t.Fatal("expected error for overly long comment")
	}
}

func TestValidateReviewMediaUploadRejectsVideo(t *testing.T) {
	spec, err := ValidateReviewMediaUpload("photo.webp", "image/webp", 1024)
	if err != nil {
		t.Fatalf("expected image upload to pass, got %v", err)
	}
	if spec.MediaType != MediaTypeImage || spec.Extension != ".webp" {
		t.Fatalf("unexpected spec: %+v", spec)
	}

	if _, err := ValidateReviewMediaUpload("clip.mp4", "video/mp4", 1024); err == nil {
		t.Fatal("expected review media upload to reject video/mp4")
	}
}

func TestReviewFilterNormalization(t *testing.T) {
	f := (ReviewListFilter{ProductID: " p-1 ", Rating: 9, Sort: "bogus", Page: 0, PageSize: 200}).Normalize()
	if f.ProductID != "p-1" || f.Rating != 0 || f.Sort != ReviewSortNewest || f.Page != 1 || f.PageSize != maxReviewPageSize {
		t.Fatalf("unexpected normalized filter: %+v", f)
	}

	af := (AdminReviewFilter{Status: "HIDDEN", ReplyStatus: "UNREPLIED", Page: -1, PageSize: 0}).Normalize()
	if af.Status != AdminReviewStatusHidden || af.ReplyStatus != AdminReplyStatusUnreplied || af.Page != 1 || af.PageSize != defaultPageSize {
		t.Fatalf("unexpected normalized admin filter: %+v", af)
	}
}
