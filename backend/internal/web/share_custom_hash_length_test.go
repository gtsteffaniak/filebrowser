package web

import (
	"strings"
	"testing"
)

func TestValidateCustomShareHashLength(t *testing.T) {
	valid := []string{
		strings.Repeat("x", 16),
		"workfiles-q3-review-2026",
		strings.Repeat("a1-_", 128),
	}
	for _, h := range valid {
		if err := validateCustomShareHash(h); err != nil {
			t.Errorf("validateCustomShareHash(len=%d) unexpected error: %v", len(h), err)
		}
	}

	invalid := []string{
		"",
		"a",
		"workfiles",
		strings.Repeat("x", 15),
		strings.Repeat("x", 513),
		"bad/hash-with-enough-length",
	}
	for _, h := range invalid {
		if err := validateCustomShareHash(h); err == nil {
			t.Errorf("validateCustomShareHash(%q) expected error", h)
		}
	}
}
