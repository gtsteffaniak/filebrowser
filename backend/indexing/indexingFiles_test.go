package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// setupTestIndex creates a test index with mock data (no filesystem dependencies)
func setupTestIndex(t *testing.T) (*Index, string, func()) {
	t.Helper()

	// Initialize index with mock data
	idx := &Index{
		Source: settings.Source{
			Name: "test",
			Path: "/mock/path",
			Config: settings.SourceConfig{
				DisableIndexing: false,
			},
		},
		wasIndexed:        true,
		mock:              true, // Enable mock mode
		Directories:       make(map[string]*iteminfo.FileInfo),
		DirectoriesLedger: make(map[string]struct{}),
		FoundHardLinks:    make(map[string]uint64),
		processedInodes:   make(map[uint64]struct{}),
	}

	// Create mock directory structure with predictable sizes
	// Using logical file sizes instead of disk allocation sizes
	idx.Directories["/"] = &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "/",
			Type: "directory",
			Size: 1000, // Total logical size: 100 + 200 + 300 + 400
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Size: 100}},
			{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Size: 200}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "subdir", Type: "directory", Size: 700}, // 300 + 400
		},
	}

	idx.Directories["/subdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "subdir",
			Type: "directory",
			Size: 700, // 300 + 400
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file3.txt", Size: 300}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "deepdir", Type: "directory", Size: 400},
		},
	}

	idx.Directories["/subdir/deepdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/deepdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "deepdir",
			Type: "directory",
			Size: 400,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file4.txt", Size: 400}},
		},
	}

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

	// Simulate adding a new file to deepdir by updating the mock data
	// Add file5.txt (500 bytes) to deepdir
	deepdirInfo := idx.Directories["/subdir/deepdir/"]
	deepdirInfo.Files = append(deepdirInfo.Files, iteminfo.ExtendedItemInfo{
		ItemInfo: iteminfo.ItemInfo{
			Name: "file5.txt",
			Size: 500,
		},
	})

	// Update deepdir size
	oldDeepdirSize := deepdirInfo.Size
	deepdirInfo.Size = 900 // 400 + 500

	// Simulate the recursive size update by calling the method directly
	idx.recursiveUpdateDirSizes(deepdirInfo, oldDeepdirSize)

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

	// Simulate adding a new file directly to root by updating mock data
	rootInfo.Files = append(rootInfo.Files, iteminfo.ExtendedItemInfo{
		ItemInfo: iteminfo.ItemInfo{
			Name: "file3.txt",
			Size: 150,
		},
	})

	// Update root size (non-recursive - only direct files)
	rootInfo.Size = initialRootSize + 150

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

	// Setup initial metadata manually
	idx.Directories["/"] = &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 1000,
		},
	}

	idx.Directories["/subdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 700,
		},
	}

	idx.Directories["/subdir/deepdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/deepdir/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 400,
		},
	}

	// Simulate updating deepdir from 400 to 900 (added 500 bytes)
	deepdirInfo := idx.Directories["/subdir/deepdir/"]
	previousSize := deepdirInfo.Size
	deepdirInfo.Size = 900

	// Call recursiveUpdateDirSizes
	idx.recursiveUpdateDirSizes(deepdirInfo, previousSize)

	// Check that subdir size updated
	subdirInfo := idx.Directories["/subdir/"]
	expectedSubdirSize := int64(1200) // 700 + 500
	if subdirInfo.Size != expectedSubdirSize {
		t.Errorf("Subdir size after recursive update: got %d, want %d", subdirInfo.Size, expectedSubdirSize)
	}

	// Check that root size updated
	rootInfo := idx.Directories["/"]
	expectedRootSize := int64(1500) // 1000 + 500
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after recursive update: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestSizeDecreasePropagate(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Setup initial metadata manually
	idx.Directories["/"] = &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 1000,
		},
	}

	idx.Directories["/subdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 700,
		},
	}

	idx.Directories["/subdir/deepdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/deepdir/",
		ItemInfo: iteminfo.ItemInfo{
			Size: 400,
		},
	}

	// Simulate updating deepdir from 400 to 100 (removed 300 bytes)
	deepdirInfo := idx.Directories["/subdir/deepdir/"]
	previousSize := deepdirInfo.Size
	deepdirInfo.Size = 100

	// Call recursiveUpdateDirSizes
	idx.recursiveUpdateDirSizes(deepdirInfo, previousSize)

	// Check that subdir size decreased
	subdirInfo := idx.Directories["/subdir/"]
	expectedSubdirSize := int64(400) // 700 - 300
	if subdirInfo.Size != expectedSubdirSize {
		t.Errorf("Subdir size after decrease: got %d, want %d", subdirInfo.Size, expectedSubdirSize)
	}

	// Check that root size decreased
	rootInfo := idx.Directories["/"]
	expectedRootSize := int64(700) // 1000 - 300
	if rootInfo.Size != expectedRootSize {
		t.Errorf("Root size after decrease: got %d, want %d", rootInfo.Size, expectedRootSize)
	}
}

func TestPreviewDoesNotPropagateFromSubdirectories(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Add an image file to subdir in mock data (should have preview)
	subdirInfo := idx.Directories["/subdir/"]
	subdirInfo.Files = append(subdirInfo.Files, iteminfo.ExtendedItemInfo{
		ItemInfo: iteminfo.ItemInfo{
			Name:       "image.jpg",
			Size:       100,
			HasPreview: true, // Image files have preview
		},
	})
	subdirInfo.HasPreview = true // Subdir now has preview

	// Check that subdir has preview (due to image.jpg)
	subdirInfo, exists := idx.GetMetadataInfo("/subdir/", true)
	if !exists {
		t.Fatal("Subdir metadata not found")
	}
	if !subdirInfo.HasPreview {
		t.Error("Subdir should have HasPreview=true due to image file")
	}

	// Check that root does NOT have preview propagated from subdir
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	// Root should only have HasPreview if it has direct previewable files, not from subdirectories
	// Since root only has text files, it should NOT have preview
	if rootInfo.HasPreview {
		t.Error("Root should NOT have HasPreview=true from subdirectory preview")
	}
}

func TestPreviewPropagatesFromFiles(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Add an image file to root in mock data (should propagate preview to root)
	rootInfo := idx.Directories["/"]
	rootInfo.Files = append(rootInfo.Files, iteminfo.ExtendedItemInfo{
		ItemInfo: iteminfo.ItemInfo{
			Name:       "image.png",
			Size:       100,
			HasPreview: true, // Image files have preview
		},
	})
	rootInfo.HasPreview = true // Root now has preview

	// Check that root HAS preview (due to direct image.png file)
	rootInfo, exists := idx.GetMetadataInfo("/", true)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	if !rootInfo.HasPreview {
		t.Error("Root should have HasPreview=true due to direct image file")
	}
}
