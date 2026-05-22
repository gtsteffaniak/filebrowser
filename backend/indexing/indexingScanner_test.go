package indexing

import (
	"os"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func TestPrepareScanStateInitializesMaps(t *testing.T) {
	idx := &Index{
		Source: settings.Source{Name: "test", Path: t.TempDir()},
	}
	s := &Scanner{scanPath: "/music/", idx: idx}

	s.prepareScanState()

	idx.mu.Lock()
	idx.scanUpdatedPaths["/music/"] = true
	idx.mu.Unlock()

	if s.processedInodes == nil {
		t.Fatal("processedInodes should be initialized")
	}
	s.processedInodes[1] = struct{}{}
}

func TestIsPathGone(t *testing.T) {
	if isPathGone(nil) {
		t.Fatal("nil error should not be path gone")
	}
	if !isPathGone(os.ErrNotExist) {
		t.Fatal("os.ErrNotExist should be path gone")
	}
}

// TestCheckFolderModtime_unchangedPreservesHasPreviewAndHidden verifies quick-scan touch updates
// do not clobber has_preview/hidden when folder modtime is unchanged.
func TestCheckFolderModtime_unchangedPreservesHasPreviewAndHidden(t *testing.T) {
	if indexDB == nil {
		var err error
		indexDB, _, err = dbsql.NewIndexDB("test_scanner_modtime", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	root := t.TempDir()
	photosDir := root + "/photos"
	if err := os.Mkdir(photosDir, 0o755); err != nil {
		t.Fatalf("mkdir photos: %v", err)
	}

	idx := &Index{
		ReducedIndex: ReducedIndex{},
		Source: settings.Source{
			Name: "test_modtime",
			Path: root,
		},
		db:                  indexDB,
		scanUpdatedPaths:    make(map[string]bool),
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
	}

	folderPath := "/photos/"
	now := time.Now()
	stored := &iteminfo.FileInfo{
		Path: folderPath,
		ItemInfo: iteminfo.ItemInfo{
			Name:       "photos",
			Type:       "directory",
			Size:       42,
			ModTime:    now.Add(-time.Hour),
			Hidden:     true,
			HasPreview: true,
		},
	}
	if err := idx.db.InsertItem("test_modtime", folderPath, stored); err != nil {
		t.Fatalf("InsertItem: %v", err)
	}
	idx.SetFolderSize(folderPath, 42)

	s := &Scanner{
		scanPath: "/",
		idx:      idx,
	}
	s.withStatsLock(func() {
		s.lastScanned = time.Now()
	})
	s.batchItems = make([]*iteminfo.FileInfo, 0, idx.db.BatchSize)

	changed, err := s.checkFolderModtime(folderPath)
	if err != nil {
		t.Fatalf("checkFolderModtime: %v", err)
	}
	if changed {
		t.Fatal("expected unchanged folder")
	}
	s.flushBatch()

	got, ok := idx.GetReducedMetadata(folderPath, true)
	if !ok || got == nil {
		t.Fatal("expected folder metadata after touch update")
	}
	if !got.HasPreview {
		t.Error("HasPreview should be preserved on unchanged modtime touch update")
	}
	if !got.Hidden {
		t.Error("Hidden should be preserved on unchanged modtime touch update")
	}
}
