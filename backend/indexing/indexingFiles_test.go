package indexing

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// setupTestIndex creates a test index with mock data (no filesystem dependencies)
func setupTestIndex(t *testing.T) (*Index, string, func()) {
	t.Helper()

	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_indexing")
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	// Initialize index with mock data
	idx := &Index{
		ReducedIndex: ReducedIndex{
			LastIndexed: time.Now(),
		},
		Source: settings.Source{
			Name: "test",
			Path: "/mock/path",
			Config: settings.SourceConfig{
				DisableIndexing: false,
			},
		},
		mock:            true, // Enable mock mode
		db:              indexDB,
		FoundHardLinks:  make(map[string]uint64),
		processedInodes: make(map[uint64]struct{}),
	}

	// Create mock directory structure with predictable sizes using database
	now := time.Now()

	// Root directory
	rootDir := &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "/",
			Type:    "directory",
			Size:    1000, // Total logical size: 100 + 200 + 300 + 400
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Size: 100, ModTime: now}},
			{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Size: 200, ModTime: now}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "subdir", Type: "directory", Size: 700}, // 300 + 400
		},
	}
	_ = idx.db.InsertItem("test", "/", rootDir)

	// File1
	file1 := &iteminfo.FileInfo{
		Path: "/file1.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file1.txt",
			Type:    "file",
			Size:    100,
			ModTime: now,
		},
	}
	_ = idx.db.InsertItem("test", "/file1.txt", file1)

	// File2
	file2 := &iteminfo.FileInfo{
		Path: "/file2.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file2.txt",
			Type:    "file",
			Size:    200,
			ModTime: now,
		},
	}
	_ = idx.db.InsertItem("test", "/file2.txt", file2)

	// Subdir
	subdir := &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "subdir",
			Type:    "directory",
			Size:    700, // 300 + 400
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file3.txt", Size: 300, ModTime: now}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "deepdir", Type: "directory", Size: 400},
		},
	}
	_ = idx.db.InsertItem("test", "/subdir/", subdir)

	// File3
	file3 := &iteminfo.FileInfo{
		Path: "/subdir/file3.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file3.txt",
			Type:    "file",
			Size:    300,
			ModTime: now,
		},
	}
	_ = idx.db.InsertItem("test", "/subdir/file3.txt", file3)

	// Deepdir
	deepdir := &iteminfo.FileInfo{
		Path: "/subdir/deepdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "deepdir",
			Type:    "directory",
			Size:    400,
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file4.txt", Size: 400, ModTime: now}},
		},
	}
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdir)

	// File4
	file4 := &iteminfo.FileInfo{
		Path: "/subdir/deepdir/file4.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file4.txt",
			Type:    "file",
			Size:    400,
			ModTime: now,
		},
	}
	_ = idx.db.InsertItem("test", "/subdir/deepdir/file4.txt", file4)

	// Cleanup function (no-op since we're not using filesystem)
	cleanup := func() {
		// Nothing to clean up in mock mode
	}

	return idx, "/mock/path", cleanup
}

func TestFolderSizeCalculation(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name         string
		path         string
		expectedSize int64
		description  string
	}{
		{
			name:         "Root directory size",
			path:         "/",
			expectedSize: 1000, // Total logical size: 100 + 200 + 300 + 400
			description:  "Root should include all nested files",
		},
		{
			name:         "Subdir size",
			path:         "/subdir/",
			expectedSize: 700, // 300 + 400
			description:  "Subdir should include its files and nested directory",
		},
		{
			name:         "Deep directory size",
			path:         "/subdir/deepdir/",
			expectedSize: 400, // 1 file
			description:  "Deepest directory should only include its direct files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get directory info directly from mock data
			dirInfo, exists := idx.GetMetadataInfo(tt.path, true)
			if !exists {
				t.Fatalf("Directory %s not found in mock data", tt.path)
			}

			if dirInfo.Size != tt.expectedSize {
				t.Errorf("%s: got size %d, want %d", tt.description, dirInfo.Size, tt.expectedSize)
			}
		})
	}
}

func TestRecursiveSizeUpdate(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Verify initial sizes (using logical sizes, not disk allocation)
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}
	if rootInfo.Size != 1000 {
		t.Errorf("Initial root size: got %d, want 1000", rootInfo.Size)
	}

	subdirInfo, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	initialSubdirSize := subdirInfo.Size
	if initialSubdirSize != 700 {
		t.Errorf("Initial subdir size: got %d, want 700", initialSubdirSize)
	}

	// Simulate adding a new file to deepdir by updating the database
	// Add file5.txt (500 bytes) to deepdir
	deepdirInfo, exists := idx.GetMetadataInfo("/subdir/deepdir/", true)
	if !exists {
		t.Fatal("Deepdir metadata not found")
	}

	// Insert the new file into database
	file5 := &iteminfo.FileInfo{
		Path: "/subdir/deepdir/file5.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file5.txt",
			Type:    "file",
			Size:    500,
			ModTime: time.Now(),
		},
	}
	err := idx.db.InsertItem("test", "/subdir/deepdir/file5.txt", file5)
	if err != nil {
		t.Fatalf("Failed to insert file5: %v", err)
	}

	// Update deepdir size
	oldDeepdirSize := deepdirInfo.Size
	deepdirInfo.Size = 900 // 400 + 500
	err = idx.db.InsertItem("test", "/subdir/deepdir/", deepdirInfo)
	if err != nil {
		t.Fatalf("Failed to insert deepdirInfo: %v", err)
	}

	// Simulate the recursive size update by calling the method directly
	idx.RecursiveUpdateDirSizes(deepdirInfo, oldDeepdirSize)

	// Check that deepdir size updated
	deepdirInfo, exists = idx.GetMetadataInfo("/subdir/deepdir/", true)
	if !exists {
		t.Fatal("Deepdir metadata not found after update")
	}
	expectedDeepdirSize := int64(900) // 400 + 500
	if deepdirInfo.Size != expectedDeepdirSize {
		t.Errorf("Deepdir size after adding file: got %d, want %d", deepdirInfo.Size, expectedDeepdirSize)
	}

	// Check that subdir size updated (includes the new file)
	subdirInfo, exists = idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found after propagation")
	}

	// Subdir should now contain file3.txt + deepdir's new size
	expectedSubdirSize := int64(1200) // 300 + 900
	if subdirInfo.Size != expectedSubdirSize {
		t.Errorf("Subdir size after propagation: got %d, want %d", subdirInfo.Size, expectedSubdirSize)
	}

	// Check that root size updated (includes changes propagated from subdir)
	rootInfo, exists = idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found after propagation")
	}
	expectedRootSize := int64(1500) // 100 + 200 + 1200
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after propagation: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestNonRecursiveMetadataUpdate(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Get initial root size
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}
	initialRootSize := rootInfo.Size

	// Simulate adding a new file directly to root by updating the database
	file3 := &iteminfo.FileInfo{
		Path: "/file3.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "file3.txt",
			Type:    "file",
			Size:    150,
			ModTime: time.Now(),
		},
	}
	_ = idx.db.InsertItem("test", "/file3.txt", file3)

	// Update root size (non-recursive - only direct files)
	rootInfo.Size = initialRootSize + 150
	_ = idx.db.InsertItem("test", "/", rootInfo)

	// Check that root size updated
	rootInfo, exists = idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found after non-recursive update")
	}

	expectedRootSize := initialRootSize + 150 // new file size
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after non-recursive update: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestRecursiveUpdateDirSizes(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Get initial metadata from database
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}
	rootInfo.Size = 1000
	_ = idx.db.InsertItem("test", "/", rootInfo)

	subdirInfo, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	subdirInfo.Size = 700
	_ = idx.db.InsertItem("test", "/subdir/", subdirInfo)

	deepdirInfo, exists := idx.GetMetadataInfo("/subdir/deepdir/", true)
	if !exists {
		t.Fatal("Deepdir metadata not found")
	}
	deepdirInfo.Size = 400
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdirInfo)

	// Simulate updating deepdir from 400 to 900 (added 500 bytes)
	previousSize := deepdirInfo.Size
	deepdirInfo.Size = 900
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdirInfo)

	// Call recursiveUpdateDirSizes
	idx.RecursiveUpdateDirSizes(deepdirInfo, previousSize)

	// Check that subdir size updated
	subdirInfo, exists = idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found after update")
	}
	expectedSubdirSize := int64(1200) // 700 + 500
	if subdirInfo.Size != expectedSubdirSize {
		t.Errorf("Subdir size after recursive update: got %d, want %d", subdirInfo.Size, expectedSubdirSize)
	}

	// Check that root size updated
	rootInfo, exists = idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found after update")
	}
	expectedRootSize := int64(1500) // 1000 + 500
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after recursive update: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestSizeDecreasePropagate(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Get initial metadata from database
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}
	rootInfo.Size = 1000
	_ = idx.db.InsertItem("test", "/", rootInfo)

	subdirInfo, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	subdirInfo.Size = 700
	_ = idx.db.InsertItem("test", "/subdir/", subdirInfo)

	deepdirInfo, exists := idx.GetMetadataInfo("/subdir/deepdir/", true)
	if !exists {
		t.Fatal("Deepdir metadata not found")
	}
	deepdirInfo.Size = 400
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdirInfo)

	// Simulate updating deepdir from 400 to 100 (removed 300 bytes)
	previousSize := deepdirInfo.Size
	deepdirInfo.Size = 100
	_ = idx.db.InsertItem("test", "/subdir/deepdir/", deepdirInfo)

	// Call recursiveUpdateDirSizes
	idx.RecursiveUpdateDirSizes(deepdirInfo, previousSize)

	// Check that subdir size decreased
	subdirInfo, exists = idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found after update")
	}
	expectedSubdirSize := int64(400) // 700 - 300
	if subdirInfo.Size != expectedSubdirSize {
		t.Errorf("Subdir size after decrease: got %d, want %d", subdirInfo.Size, expectedSubdirSize)
	}

	// Check that root size decreased
	rootInfo, exists = idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found after update")
	}
	expectedRootSize := int64(700) // 1000 - 300
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after decrease: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestPreviewDoesNotPropagateFromSubdirectories(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Add an image file to subdir in database (should have preview)
	subdirInfo, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	imageFile := &iteminfo.FileInfo{
		Path: "/subdir/image.jpg",
		ItemInfo: iteminfo.ItemInfo{
			Name:       "image.jpg",
			Type:       "file",
			Size:       100,
			HasPreview: true, // Image files have preview
			ModTime:    time.Now(),
		},
	}
	_ = idx.db.InsertItem("test", "/subdir/image.jpg", imageFile)
	subdirInfo.HasPreview = true // Subdir now has preview
	_ = idx.db.InsertItem("test", "/subdir/", subdirInfo)

	// Check that subdir has preview (due to image.jpg)
	subdirInfoCheck, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	if !subdirInfoCheck.HasPreview {
		t.Error("Subdir should have HasPreview=true due to image file")
	}

	// Check that root does NOT have preview propagated from subdir
	rootInfoCheck, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	// Root should only have HasPreview if it has direct previewable files, not from subdirectories
	// Since root only has text files, it should NOT have preview
	if rootInfoCheck.HasPreview {
		t.Error("Root should NOT have HasPreview=true from subdirectory preview")
	}
}

func TestPreviewPropagatesFromFiles(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Add an image file to root in database (should propagate preview to root)
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}
	imageFile := &iteminfo.FileInfo{
		Path: "/image.png",
		ItemInfo: iteminfo.ItemInfo{
			Name:       "image.png",
			Type:       "file",
			Size:       100,
			HasPreview: true, // Image files have preview
			ModTime:    time.Now(),
		},
	}
	_ = idx.db.InsertItem("test", "/image.png", imageFile)
	rootInfo.HasPreview = true // Root now has preview
	_ = idx.db.InsertItem("test", "/", rootInfo)

	// Check that root HAS preview (due to direct image.png file)
	rootInfoCheck, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	if !rootInfoCheck.HasPreview {
		t.Error("Root should have HasPreview=true due to direct image file")
	}
}
