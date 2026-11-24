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

func TestItemMatchesSearchFiltersTypeFileOnly(t *testing.T) {
	image := iteminfo.ItemInfo{Name: "photo.jpg", Type: "image/jpeg", ModTime: time.Now()}
	folder := iteminfo.ItemInfo{Name: "pics", Type: "directory", ModTime: time.Now()}

	opts := iteminfo.SearchOptions{
		Conditions: map[string]bool{"dir": false},
		Terms:      []string{},
	}

	if !itemMatchesSearchFilters(image, false, opts, false, false) {
		t.Fatal("type:file should match a non-directory file")
	}
	if itemMatchesSearchFilters(folder, true, opts, false, false) {
		t.Fatal("type:file should not match a directory")
	}
}

func TestItemMatchesSearchFiltersDateOnly(t *testing.T) {
	old := iteminfo.ItemInfo{Name: "old.txt", Type: "text/plain", ModTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := iteminfo.ItemInfo{Name: "new.txt", Type: "text/plain", ModTime: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	opts := iteminfo.SearchOptions{
		Terms:             []string{},
		ModifiedOlderThan: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	if !itemMatchesSearchFilters(old, false, opts, false, false) {
		t.Fatal("olderThan-only search should match older files")
	}
	if itemMatchesSearchFilters(newer, false, opts, false, false) {
		t.Fatal("olderThan-only search should not match newer files")
	}
}
