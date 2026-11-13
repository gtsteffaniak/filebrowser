package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// A wider candidate set must never apply to largest searches: they are ranked by size
// and the handler keeps results in depth order, so widening would let small shallow
// files displace larger deeper ones.
func TestSearchPageSizeDoesNotWidenLargest(t *testing.T) {
	original := settings.Config
	t.Cleanup(func() { settings.Config = original })
	settings.Config.Server.SearchResultsLimit = 200

	opts := &searchOptions{
		largest: true,
		parsed:  iteminfo.SearchOptions{Terms: []string{"budget"}},
		sources: []string{"one"},
	}
	for _, explicit := range []bool{false, true} {
		if got := searchPageSize(opts, 200, explicit); got != 200 {
			t.Fatalf("largest page size = %d, want 200 (explicitLimit=%t)", got, explicit)
		}
	}

	opts.largest = false
	if got := searchPageSize(opts, 200, true); got != accessFilterPageSize {
		t.Fatalf("explicit-limit page size = %d, want %d", got, accessFilterPageSize)
	}
	if got := searchPageSize(opts, 100, false); got != 200 {
		t.Fatalf("per-source page size = %d, want 200", got)
	}
}
