//go:build mupdf

package fulltext

import "testing"

func TestTruncateRunesKeepsCompleteRune(t *testing.T) {
	// "é" is two bytes; a two-byte budget must keep it whole, not drop it.
	if got := truncateRunes("éx", 2); got != "é" {
		t.Fatalf("truncateRunes = %q, want %q", got, "é")
	}
	if got := truncateRunes("éx", 1); got != "" {
		t.Fatalf("truncateRunes = %q, want empty", got)
	}
	if got := truncateRunes("abc", 2); got != "ab" {
		t.Fatalf("truncateRunes = %q, want %q", got, "ab")
	}
}
