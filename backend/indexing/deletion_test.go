package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// setupTestIndexForDeletion creates a test index with mock data for deletion testing
func setupTestIndexForDeletion(t *testing.T) *Index {
	t.Helper()

	// Create index with mock data
	idx := &Index{
		Source: settings.Source{
			Name: "test_delete",
			Path: "/mock/path",
			Config: settings.SourceConfig{
				DisableIndexing: false,
			},
		},
		Directories:       make(map[string]*iteminfo.FileInfo),
		DirectoriesLedger: make(map[string]struct{}),
		FoundHardLinks:    make(map[string]uint64),
		processedInodes:   make(map[uint64]struct{}),
		mock:              true, // Enable mock mode
	}

	// Create mock directory structure with predictable sizes
	idx.Directories["/"] = &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "/",
			Type: "directory",
			Size: 1000, // Total: 100 + 200 + 700 (subdir)
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Size: 100}},
			{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Size: 200}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "subdir", Type: "directory", Size: 700},
		},
	}

	idx.Directories["/subdir/"] = &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "subdir",
			Type: "directory",
			Size: 700, // 300 + 400 (deepdir)
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

	idx.DirectoriesLedger["/"] = struct{}{}
	idx.DirectoriesLedger["/subdir/"] = struct{}{}
	idx.DirectoriesLedger["/subdir/deepdir/"] = struct{}{}

	return idx
}

func TestDeleteMetadata_RecursiveDirectoryCleanup(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a directory recursively
	idx.DeleteMetadata("/subdir/deepdir/", true, true)

	// Verify it was removed from Directories
	if _, exists := idx.Directories["/subdir/deepdir/"]; exists {
		t.Error("Directory '/subdir/deepdir/' should have been removed from Directories")
	}

	// Verify it was removed from DirectoriesLedger
	if _, exists := idx.DirectoriesLedger["/subdir/deepdir/"]; exists {
		t.Error("Directory '/subdir/deepdir/' should have been removed from DirectoriesLedger")
	}

	// Verify parent still exists
	if _, exists := idx.Directories["/subdir/"]; !exists {
		t.Error("Parent directory '/subdir/' should still exist")
	}
}

func TestDeleteMetadata_RecursiveWithSubdirectories(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Add a deeper subdirectory
	idx.Directories["/subdir/deepdir/verydeep/"] = &iteminfo.FileInfo{
		Path: "/subdir/deepdir/verydeep/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "verydeep",
			Type: "directory",
			Size: 50,
		},
	}
	idx.DirectoriesLedger["/subdir/deepdir/verydeep/"] = struct{}{}

	// Delete the parent recursively
	idx.DeleteMetadata("/subdir/deepdir/", true, true)

	// Verify all subdirectories were removed
	if _, exists := idx.Directories["/subdir/deepdir/"]; exists {
		t.Error("Directory '/subdir/deepdir/' should have been removed")
	}
	if _, exists := idx.Directories["/subdir/deepdir/verydeep/"]; exists {
		t.Error("Subdirectory '/subdir/deepdir/verydeep/' should have been removed")
	}

	// Verify ledger was cleaned
	if _, exists := idx.DirectoriesLedger["/subdir/deepdir/"]; exists {
		t.Error("Directory '/subdir/deepdir/' should have been removed from ledger")
	}
	if _, exists := idx.DirectoriesLedger["/subdir/deepdir/verydeep/"]; exists {
		t.Error("Subdirectory '/subdir/deepdir/verydeep/' should have been removed from ledger")
	}
}

func TestDeleteMetadata_FileRemoval(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a file
	idx.DeleteMetadata("/subdir/file3.txt", false, false)

	// Verify file was removed from parent's Files slice
	parentInfo, exists := idx.Directories["/subdir/"]
	if !exists {
		t.Fatal("Parent directory not found")
	}

	for _, file := range parentInfo.Files {
		if file.Name == "file3.txt" {
			t.Error("File 'file3.txt' should have been removed from parent's Files slice")
		}
	}
}

func TestDeleteMetadata_DirectoryFromParentFolder(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a directory (non-recursive)
	idx.DeleteMetadata("/subdir/deepdir/", true, false)

	// Verify directory was removed from Directories
	if _, exists := idx.Directories["/subdir/deepdir/"]; exists {
		t.Error("Directory '/subdir/deepdir/' should have been removed")
	}

	// Verify it was removed from parent's Folders slice
	parentInfo, exists := idx.Directories["/subdir/"]
	if !exists {
		t.Fatal("Parent directory not found")
	}

	for _, folder := range parentInfo.Folders {
		if folder.Name == "deepdir" {
			t.Error("Folder 'deepdir' should have been removed from parent's Folders slice")
		}
	}
}

