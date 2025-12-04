package indexing

import (
	"database/sql"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// setupTestIndexForDeletion creates a test index with mock data for deletion testing
func setupTestIndexForDeletion(t *testing.T) *Index {
	t.Helper()

	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_deletion")
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	// Create index with mock data
	idx := &Index{
		Source: settings.Source{
			Name: "test_delete",
			Path: "/mock/path",
			Config: settings.SourceConfig{
				DisableIndexing: false,
			},
		},
		db:              indexDB,
		FoundHardLinks:  make(map[string]uint64),
		processedInodes: make(map[uint64]struct{}),
		mock:            true, // Enable mock mode
	}

	// Create mock directory structure with predictable sizes using database
	now := time.Now()

	// Root directory
	rootDir := &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "/",
			Type:    "directory",
			Size:    1000, // Total: 100 + 200 + 700 (subdir)
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Size: 100, ModTime: now}},
			{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Size: 200, ModTime: now}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "subdir", Type: "directory", Size: 700},
		},
	}
	_ = idx.db.InsertItem("test_delete", "/", rootDir)

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
	_ = idx.db.InsertItem("test_delete", "/file1.txt", file1)

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
	_ = idx.db.InsertItem("test_delete", "/file2.txt", file2)

	// Subdir
	subdir := &iteminfo.FileInfo{
		Path: "/subdir/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "subdir",
			Type:    "directory",
			Size:    700, // 300 + 400 (deepdir)
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "file3.txt", Size: 300, ModTime: now}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "deepdir", Type: "directory", Size: 400},
		},
	}
	_ = idx.db.InsertItem("test_delete", "/subdir/", subdir)

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
	_ = idx.db.InsertItem("test_delete", "/subdir/file3.txt", file3)

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
	_ = idx.db.InsertItem("test_delete", "/subdir/deepdir/", deepdir)

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
	_ = idx.db.InsertItem("test_delete", "/subdir/deepdir/file4.txt", file4)

	return idx
}

func TestDeleteMetadata_RecursiveDirectoryCleanup(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a directory recursively
	idx.DeleteMetadata("/subdir/deepdir/", true, true)

	// Verify it was removed from database
	item, err := idx.db.GetItem("test_delete", "/subdir/deepdir/")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if item != nil {
		t.Error("Directory '/subdir/deepdir/' should have been removed from database")
	}

	// Verify file4 was also removed
	file4, err := idx.db.GetItem("test_delete", "/subdir/deepdir/file4.txt")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if file4 != nil {
		t.Error("File '/subdir/deepdir/file4.txt' should have been removed from database")
	}

	// Verify parent still exists
	parent, err := idx.db.GetItem("test_delete", "/subdir/")
	if err != nil {
		t.Fatalf("Error checking database: %v", err)
	}
	if parent == nil {
		t.Error("Parent directory '/subdir/' should still exist")
	}
}

func TestDeleteMetadata_RecursiveWithSubdirectories(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Add a deeper subdirectory
	verydeep := &iteminfo.FileInfo{
		Path: "/subdir/deepdir/verydeep/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "verydeep",
			Type:    "directory",
			Size:    50,
			ModTime: time.Now(),
		},
	}
	_ = idx.db.InsertItem("test_delete", "/subdir/deepdir/verydeep/", verydeep)

	// Delete the parent recursively
	idx.DeleteMetadata("/subdir/deepdir/", true, true)

	// Verify all subdirectories were removed
	deepdir, err := idx.db.GetItem("test_delete", "/subdir/deepdir/")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if deepdir != nil {
		t.Error("Directory '/subdir/deepdir/' should have been removed")
	}

	verydeepItem, err := idx.db.GetItem("test_delete", "/subdir/deepdir/verydeep/")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if verydeepItem != nil {
		t.Error("Subdirectory '/subdir/deepdir/verydeep/' should have been removed")
	}
}

func TestDeleteMetadata_FileRemoval(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a file
	idx.DeleteMetadata("/subdir/file3.txt", false, false)

	// Verify file was removed from database
	file3, err := idx.db.GetItem("test_delete", "/subdir/file3.txt")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if file3 != nil {
		t.Error("File '/subdir/file3.txt' should have been removed from database")
	}

	// Verify parent directory still exists
	parent, err := idx.db.GetItem("test_delete", "/subdir/")
	if err != nil {
		t.Fatalf("Error checking database: %v", err)
	}
	if parent == nil {
		t.Fatal("Parent directory not found")
	}
}

func TestDeleteMetadata_DirectoryFromParentFolder(t *testing.T) {
	idx := setupTestIndexForDeletion(t)

	// Delete a directory (non-recursive)
	idx.DeleteMetadata("/subdir/deepdir/", true, false)

	// Verify directory was removed from database
	deepdir, err := idx.db.GetItem("test_delete", "/subdir/deepdir/")
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Error checking database: %v", err)
	}
	if deepdir != nil {
		t.Error("Directory '/subdir/deepdir/' should have been removed")
	}

	// Verify parent directory still exists
	parent, err := idx.db.GetItem("test_delete", "/subdir/")
	if err != nil {
		t.Fatalf("Error checking database: %v", err)
	}
	if parent == nil {
		t.Fatal("Parent directory not found")
	}

	// Verify child file still exists (non-recursive delete)
	file4, err := idx.db.GetItem("test_delete", "/subdir/deepdir/file4.txt")
	if err != nil {
		t.Fatalf("Error checking database: %v", err)
	}
	if file4 == nil {
		t.Error("File '/subdir/deepdir/file4.txt' should still exist (non-recursive delete)")
	}
}

