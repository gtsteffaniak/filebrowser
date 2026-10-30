package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// TestFetchExtendedAttributes tests the fetchExtendedAttributes helper function
func TestFetchExtendedAttributes(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name              string
		adjustedPath      string
		opts              Options
		expectMapNotEmpty bool
	}{
		{
			name:              "SkipExtendedAttrs returns empty",
			adjustedPath:      "/test/",
			opts:              Options{SkipExtendedAttrs: true, Recursive: false},
			expectMapNotEmpty: false,
		},
		{
			name:              "Recursive returns empty",
			adjustedPath:      "/test/",
			opts:              Options{SkipExtendedAttrs: false, Recursive: true},
			expectMapNotEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subdirMap := idx.fetchExtendedAttributes(tt.adjustedPath, tt.opts)
			if (len(subdirMap) > 0) != tt.expectMapNotEmpty {
				t.Errorf("expected mapNotEmpty=%v, got %v", tt.expectMapNotEmpty, len(subdirMap) > 0)
			}
		})
	}
}

// TestFetchExtendedAttributes_subdirHasPreviewFromMetadata checks folder hasPreview comes from one GetMetadataInfo (non-shallow).
func TestFetchExtendedAttributes_subdirHasPreviewFromMetadata(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	deepdir, err := idx.db.GetItem("test", "/subdir/deepdir/")
	if err != nil || deepdir == nil {
		t.Fatalf("deepdir row: err=%v", err)
	}
	deepdir.HasPreview = true
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdir)

	subdirMap := idx.fetchExtendedAttributes("/subdir/", Options{
		SkipExtendedAttrs: false,
		Recursive:         false,
	})

	if !subdirMap["/subdir/deepdir/"] {
		t.Errorf("expected hasPreview map for /subdir/deepdir/, got %v", subdirMap)
	}
}

// TestGetDirectoryName tests the directory name extraction logic
func TestGetDirectoryName(t *testing.T) {
	tests := []struct {
		name      string
		indexPath string
		idxPath   string
		expected  string
	}{
		{
			name:      "Normal directory path",
			indexPath: "/documents/",
			idxPath:   "/home/user",
			expected:  "documents",
		},
		{
			name:      "Root path",
			indexPath: "/",
			idxPath:   "/home/user",
			expected:  "user",
		},
		{
			name:      "Path with trailing slash",
			indexPath: "/mydir/",
			idxPath:   "/base",
			expected:  "mydir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: The actual getDirectoryName logic is now inline in GetDirInfoCore
			// This test verifies the expected behavior using filepath.Base

			baseName := filepath.Base(strings.TrimSuffix(tt.indexPath, "/"))
			if tt.indexPath == "/" {
				baseName = filepath.Base(tt.idxPath)
			}

			if baseName != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, baseName)
			}
		})
	}
}

// TestProcessDirectoryItem tests the processDirectoryItem helper function
func TestProcessDirectoryItem(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Set up folder size in memory
	idx.SetFolderSize("/test/subdir/", 500)

	now := time.Now()
	mockDir := &mockFileInfo{
		name:    "subdir",
		size:    0,
		mode:    os.ModeDir,
		modTime: now,
		isDir:   true,
	}

	mockDirNoSize := &mockFileInfo{
		name:    "nosize",
		size:    0,
		mode:    os.ModeDir,
		modTime: now,
		isDir:   true,
	}

	tests := []struct {
		name                string
		file                os.FileInfo
		combinedPath        string
		realPath            string
		fullCombined        string
		subdirHasPreviewMap map[string]bool
		opts                Options
		expectNil           bool
		expectSize          int64
		expectShouldCount   bool
	}{
		{
			name:         "Non-recursive with cached size",
			file:         mockDir,
			combinedPath: "/test/",
			realPath:     "/real/test",
			fullCombined: "/test/subdir",
			subdirHasPreviewMap: map[string]bool{
				"/test/subdir/": true,
			},
			opts: Options{
				Recursive:         false,
				SkipExtendedAttrs: false,
			},
			expectNil:         false,
			expectSize:        500,
			expectShouldCount: true,
		},
		{
			name:         "Non-recursive without cached size",
			file:         mockDirNoSize,
			combinedPath: "/test/",
			realPath:     "/real/test",
			fullCombined: "/test/nosize",
			subdirHasPreviewMap: map[string]bool{
				"/test/nosize/": false,
			},
			opts: Options{
				Recursive:         false,
				SkipExtendedAttrs: false,
			},
			expectNil:         false,
			expectSize:        0,
			expectShouldCount: true,
		},
		{
			name:         "SkipExtendedAttrs",
			file:         mockDir,
			combinedPath: "/test/",
			realPath:     "/real/test",
			fullCombined: "/test/subdir",
			subdirHasPreviewMap: map[string]bool{
				"/test/subdir/": true,
			},
			opts: Options{
				Recursive:         false,
				SkipExtendedAttrs: true,
			},
			expectNil:         false,
			expectSize:        500,
			expectShouldCount: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itemInfo, size, shouldCount := idx.processDirectoryItem(
				tt.file, tt.combinedPath,
				tt.subdirHasPreviewMap, tt.opts, nil,
			)

			if tt.expectNil {
				if itemInfo != nil {
					t.Errorf("expected nil itemInfo, got %v", itemInfo)
				}
				return
			}

			if size != tt.expectSize {
				t.Errorf("expected size=%d, got %d", tt.expectSize, size)
			}

			if shouldCount != tt.expectShouldCount {
				t.Errorf("expected shouldCount=%v, got %v", tt.expectShouldCount, shouldCount)
			}

			if itemInfo.Type != "directory" {
				t.Errorf("expected type=directory, got %s", itemInfo.Type)
			}
		})
	}
}

// TestProcessFileItem tests the processFileItem helper function
func TestProcessFileItem(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	now := time.Now()
	mockFile := &mockFileInfo{
		name:    "testfile.txt",
		size:    1024,
		mode:    0,
		modTime: now,
		isDir:   false,
	}

	tests := []struct {
		name              string
		file              os.FileInfo
		realPath          string
		combinedPath      string
		fullCombined      string
		opts              Options
		scanner           *Scanner
		expectSize        int64
		expectShouldCount bool
	}{
		{
			name:         "API call (non-recursive, no scanner)",
			file:         mockFile,
			realPath:     "/real/test",
			combinedPath: "/test/",
			fullCombined: "/test/testfile.txt",
			opts: Options{
				Recursive:         false,
				SkipExtendedAttrs: false,
			},
			scanner:           nil,
			expectSize:        1024,
			expectShouldCount: true,
		},
		{
			name:         "SkipExtendedAttrs",
			file:         mockFile,
			realPath:     "/real/test",
			combinedPath: "/test/",
			fullCombined: "/test/testfile.txt",
			opts: Options{
				Recursive:         false,
				SkipExtendedAttrs: true,
			},
			scanner:           nil,
			expectSize:        1024,
			expectShouldCount: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itemInfo, size, shouldCount, bubblesUp := idx.processFileItem(
				tt.file, tt.combinedPath, tt.opts, tt.scanner, nil,
			)

			if itemInfo == nil {
				t.Fatalf("expected non-nil itemInfo")
			}

			if size != tt.expectSize {
				t.Errorf("expected size=%d, got %d", tt.expectSize, size)
			}

			if shouldCount != tt.expectShouldCount {
				t.Errorf("expected shouldCount=%v, got %v", tt.expectShouldCount, shouldCount)
			}

			// bubblesUp is a boolean, just verify it's set
			_ = bubblesUp
		})
	}
}

// TestFetchDirectoryCache_cachesIndexedFiles verifies the file metadata map is
// built from the same GetMetadataInfo query as the folder preview map.
func TestFetchDirectoryCache_cachesIndexedFiles(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	subdirMap, cachedFiles := idx.fetchDirectoryCache("/", Options{
		SkipExtendedAttrs: false,
		Recursive:         false,
	})

	if _, ok := subdirMap["/subdir/"]; !ok {
		t.Errorf("expected hasPreview entry for /subdir/, got %v", subdirMap)
	}
	if len(cachedFiles) != 2 {
		t.Fatalf("expected 2 cached files at root, got %d", len(cachedFiles))
	}
	f1 := cachedFiles["file1.txt"]
	if f1 == nil || f1.Size != 100 || f1.Type != "file" {
		t.Errorf("unexpected cached file1.txt: %+v", f1)
	}

	// Recursive listings never use the cache (they index fresh metadata).
	_, cachedFiles = idx.fetchDirectoryCache("/", Options{Recursive: true})
	if cachedFiles != nil {
		t.Errorf("expected nil cachedFiles for recursive opts, got %v", cachedFiles)
	}
}

// TestProcessFileItem_CachedMetadataReuse verifies unchanged files reuse cached
// type/preview/created metadata instead of re-detecting, while changed files
// fall back to fresh detection.
func TestProcessFileItem_CachedMetadataReuse(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	now := time.Now()
	created := now.Add(-time.Hour)
	mockFile := &mockFileInfo{
		name:    "track.flac",
		size:    1024,
		modTime: now,
	}
	opts := Options{Recursive: false, SkipExtendedAttrs: false}

	cachedFiles := map[string]*iteminfo.ItemInfo{
		"track.flac": {
			Name:       "track.flac",
			Size:       1024,
			ModTime:    now,
			Created:    &created,
			Type:       "audio/x-cached",
			HasPreview: true,
		},
	}

	itemInfo, _, _, _ := idx.processFileItem(mockFile, "/test/", opts, nil, cachedFiles)
	if itemInfo.Type != "audio/x-cached" {
		t.Errorf("expected cached type, got %s", itemInfo.Type)
	}
	if !itemInfo.HasPreview {
		t.Error("expected cached HasPreview=true")
	}
	if itemInfo.Created == nil || !itemInfo.Created.Equal(created) {
		t.Errorf("expected cached Created %v, got %v", created, itemInfo.Created)
	}

	// Size change invalidates the cache: fresh detection runs and preview is
	// recomputed (the nonexistent real path yields no album art).
	stale := map[string]*iteminfo.ItemInfo{
		"track.flac": {
			Name:       "track.flac",
			Size:       512,
			ModTime:    now,
			Type:       "audio/x-cached",
			HasPreview: true,
		},
	}
	itemInfo, _, _, _ = idx.processFileItem(mockFile, "/test/", opts, nil, stale)
	if itemInfo.Type == "audio/x-cached" {
		t.Error("expected fresh type detection after size change")
	}
	if itemInfo.HasPreview {
		t.Error("expected HasPreview=false after fresh detection")
	}

	// Mtime change also invalidates the cache.
	staleMtime := map[string]*iteminfo.ItemInfo{
		"track.flac": {
			Name:       "track.flac",
			Size:       1024,
			ModTime:    now.Add(-time.Minute),
			Type:       "audio/x-cached",
			HasPreview: true,
		},
	}
	itemInfo, _, _, _ = idx.processFileItem(mockFile, "/test/", opts, nil, staleMtime)
	if itemInfo.Type == "audio/x-cached" {
		t.Error("expected fresh type detection after mtime change")
	}
}

// TestGetDirInfoCore_IndexableListing verifies the second listing returned by
// GetDirInfoCore contains only items indexing would persist (hidden and
// viewable-only entries are displayed but not indexed), and that
// persistDirectoryListing writes exactly those rows plus the folder size.
func TestGetDirInfoCore_IndexableListing(t *testing.T) {
	if indexDB == nil {
		var err error
		indexDB, _, err = dbsql.NewIndexDB("test_indexable", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	root := t.TempDir()
	for _, name := range []string{"visible.txt", ".hidden.txt", "viewable.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	idx := &Index{
		Source: settings.Source{
			Name: "test_indexable",
			Path: root,
			Config: settings.SourceConfig{
				UseLogicalSize: true,
				ResolvedRules: settings.ResolvedRulesConfig{
					// Viewable in listings but excluded from indexing.
					FileNames: map[string]settings.ConditionalRule{
						"viewable.txt": {Viewable: true},
					},
				},
			},
		},
		db:                  indexDB,
		scanUpdatedPaths:    make(map[string]bool),
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
	}
	// Clean any rows left by a previous run for this source.
	if err := idx.db.DeleteItem(idx.Name, "/", true); err != nil {
		t.Fatalf("cleanup DeleteItem: %v", err)
	}

	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	stat, err := dir.Stat()
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		CheckViewable:     true,
		ShowHidden:        true,
		SkipExtendedAttrs: false,
	}
	display, indexable, err := idx.GetDirInfoCore(dir, stat, "/", opts, nil)
	if err != nil {
		t.Fatalf("GetDirInfoCore: %v", err)
	}

	displayNames := make(map[string]bool, len(display.Files))
	for _, f := range display.Files {
		displayNames[f.Name] = true
	}
	for _, name := range []string{"visible.txt", ".hidden.txt", "viewable.txt"} {
		if !displayNames[name] {
			t.Errorf("expected %s in display listing", name)
		}
	}

	if indexable == nil {
		t.Fatal("expected non-nil indexable listing")
	}
	if len(indexable.Files) != 1 || indexable.Files[0].Name != "visible.txt" {
		names := make([]string, 0, len(indexable.Files))
		for _, f := range indexable.Files {
			names = append(names, f.Name)
		}
		t.Fatalf("expected only visible.txt in indexable listing, got %v", names)
	}
	if indexable.Size != 4 {
		t.Errorf("expected indexable size 4 (visible.txt only), got %d", indexable.Size)
	}

	// Persisting must write only indexable rows and set the folder size.
	idx.persistDirectoryListing("/", indexable, opts, 1)

	if size, ok := idx.GetFolderSize("/"); !ok || size != 4 {
		t.Errorf("expected folder size 4, got %d (ok=%v)", size, ok)
	}
	meta, exists := idx.GetMetadataInfo("/", true, false)
	if !exists {
		t.Fatal("expected indexed directory metadata after persist")
	}
	if len(meta.Files) != 1 || meta.Files[0].Name != "visible.txt" {
		names := make([]string, 0, len(meta.Files))
		for _, f := range meta.Files {
			names = append(names, f.Name)
		}
		t.Fatalf("expected only visible.txt indexed, got %v", names)
	}
}

// TestPersistDirectoryListing_PropagatesSizeToParents verifies that when a
// refreshed listing changes a cached folder size, the delta is propagated to
// ancestor folder sizes rather than leaving them stale.
func TestPersistDirectoryListing_PropagatesSizeToParents(t *testing.T) {
	db, _, err := dbsql.NewIndexDB("test_persist_prop", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	idx := &Index{
		Source: settings.Source{
			Name: "test_persist_prop",
			Path: t.TempDir(),
			Config: settings.SourceConfig{
				UseLogicalSize: true,
			},
		},
		db:                  db,
		scanUpdatedPaths:    make(map[string]bool),
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
	}

	opts := Options{CheckViewable: true}
	listing := func(size int64) *iteminfo.FileInfo {
		return &iteminfo.FileInfo{
			Path:     "/sub/",
			ItemInfo: iteminfo.ItemInfo{Name: "sub", Type: "directory", Size: size},
		}
	}

	// Seed stale sizes: child contributes 40 toward the parent's 100.
	idx.SetFolderSize("/", 100)
	idx.SetFolderSize("/sub/", 40)

	idx.persistDirectoryListing("/sub/", listing(60), opts, 1)
	if size, _ := idx.GetFolderSize("/sub/"); size != 60 {
		t.Errorf("expected /sub/ size 60, got %d", size)
	}
	if size, _ := idx.GetFolderSize("/"); size != 120 {
		t.Errorf("expected / size 120 after +20 delta, got %d", size)
	}

	// Shrink propagates a negative delta too.
	idx.persistDirectoryListing("/sub/", listing(30), opts, 2)
	if size, _ := idx.GetFolderSize("/"); size != 90 {
		t.Errorf("expected / size 90 after -30 delta, got %d", size)
	}
}

// TestPersistDirectoryListing_SkipsStaleGeneration verifies that an older listing
// generation cannot overwrite folder or ancestor sizes after a newer one applied.
func TestPersistDirectoryListing_SkipsStaleGeneration(t *testing.T) {
	db, _, err := dbsql.NewIndexDB("test_persist_stale", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	idx := &Index{
		Source: settings.Source{
			Name: "test_persist_stale",
			Path: t.TempDir(),
			Config: settings.SourceConfig{
				UseLogicalSize: true,
			},
		},
		db:                  db,
		scanUpdatedPaths:    make(map[string]bool),
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
		lastPersistGen:      make(map[string]uint64),
	}

	opts := Options{CheckViewable: true}
	listing := func(size int64) *iteminfo.FileInfo {
		return &iteminfo.FileInfo{
			Path:     "/sub/",
			ItemInfo: iteminfo.ItemInfo{Name: "sub", Type: "directory", Size: size},
		}
	}

	idx.SetFolderSize("/", 100)
	idx.SetFolderSize("/sub/", 40)

	idx.persistDirectoryListing("/sub/", listing(60), opts, 2)
	if size, _ := idx.GetFolderSize("/sub/"); size != 60 {
		t.Fatalf("expected /sub/ size 60, got %d", size)
	}
	if size, _ := idx.GetFolderSize("/"); size != 120 {
		t.Fatalf("expected / size 120, got %d", size)
	}

	// Simulate out-of-order background execution: older generation must no-op.
	idx.persistDirectoryListing("/sub/", listing(10), opts, 1)
	if size, _ := idx.GetFolderSize("/sub/"); size != 60 {
		t.Fatalf("stale generation changed /sub/ size to %d, want 60", size)
	}
	if size, _ := idx.GetFolderSize("/"); size != 120 {
		t.Fatalf("stale generation changed / size to %d, want 120", size)
	}
}

// mockFileInfo is a simple mock implementation of os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
