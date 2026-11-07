package indexing

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestItemMatchesSearchFiltersTypeOnly(t *testing.T) {
	image := iteminfo.ItemInfo{Name: "photo.jpg", Type: "image/jpeg", ModTime: time.Now()}
	text := iteminfo.ItemInfo{Name: "notes.txt", Type: "text/plain", ModTime: time.Now()}

	opts := iteminfo.SearchOptions{
		Conditions: map[string]bool{"image": true},
		Terms:      []string{},
	}

	if !itemMatchesSearchFilters(image, false, opts, false, false) {
		t.Fatal("type:image should match an image with no name terms")
	}
	if itemMatchesSearchFilters(text, false, opts, false, false) {
		t.Fatal("type:image should not match a text file")
	}
}
