package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidatePRTitleValid(t *testing.T) {
	validTitles := []string{
		"feat(order): add checkout validation",
		"fix!: reject reused refresh tokens",
		"docs: clarify local startup",
		"refactor(cart-service)!: simplify line item enrichment",
	}
	for _, title := range validTitles {
		var out, errOut bytes.Buffer
		code := run([]string{"-title", title}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit 0 for %q, got %d (stderr: %s)", title, code, errOut.String())
		}
		want := "Valid pull-request title: " + title
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected stdout %q, got %q", want, out.String())
		}
	}
}

func TestValidatePRTitleInvalid(t *testing.T) {
	invalidTitles := []string{
		"",
		"Feat(order): add checkout validation",
		"feat(order_api): add checkout validation",
		"feat(order) add checkout validation",
		"feat(order):",
		"feat(order): ",
	}
	for _, title := range invalidTitles {
		var out, errOut bytes.Buffer
		code := run([]string{"-Title", title}, &out, &errOut)
		if code != 1 {
			t.Fatalf("expected exit 1 for %q, got %d", title, code)
		}
		if !strings.Contains(errOut.String(), "Expected: <type>(<optional-scope>)<optional-breaking-marker>: <description>") {
			t.Fatalf("expected error message for %q, got %q", title, errOut.String())
		}
	}
}
