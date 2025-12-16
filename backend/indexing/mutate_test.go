package indexing

import (
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/stretchr/testify/assert"
)

var (
	testIndex            Index
	testIndexInitialized bool
	testIndexSetupMutex  sync.Mutex
)

func setupMutateTestIndex(t *testing.T) *Index {
	t.Helper()

	testIndexSetupMutex.Lock()
	defer testIndexSetupMutex.Unlock()

	if testIndexInitialized {
		return &testIndex
	}

	var err error
	testDB, err := dbsql.NewIndexDB("test_init")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	testIndex = Index{
		ReducedIndex: ReducedIndex{
			NumFiles: 10,
			NumDirs:  5,
		},
		Source: settings.Source{
			Path: "/",
			Name: "test",
		},
		db:   testDB,
		mock: true,
	}

	now := time.Now()
	testpath := &iteminfo.FileInfo{
		Path: "/testpath/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "testpath",
			Type:    "directory",
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "testfile.txt", Size: 100, ModTime: now}},
			{ItemInfo: iteminfo.ItemInfo{Name: "anotherfile.txt", Size: 100, ModTime: now}},
		},
	}
	err = testDB.BulkInsertItems("test", []*iteminfo.FileInfo{
		testpath,
		{Path: "/testpath/testfile.txt", ItemInfo: iteminfo.ItemInfo{Name: "testfile.txt", Size: 100, ModTime: now}},
		{Path: "/testpath/anotherfile.txt", ItemInfo: iteminfo.ItemInfo{Name: "anotherfile.txt", Size: 100, ModTime: now}},
	})
	if err != nil {
		t.Fatalf("Failed to insert test data for /testpath/: %v", err)
	}

	anotherpath := &iteminfo.FileInfo{
		Path: "/anotherpath/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "anotherpath",
			Type:    "directory",
			ModTime: now,
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "afile.txt", Size: 100, ModTime: now}},
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "directory", Type: "directory", Size: 100},
		},
	}
	err = testDB.BulkInsertItems("test", []*iteminfo.FileInfo{
		anotherpath,
		{Path: "/anotherpath/afile.txt", ItemInfo: iteminfo.ItemInfo{Name: "afile.txt", Size: 100, ModTime: now}},
		{Path: "/anotherpath/directory/", ItemInfo: iteminfo.ItemInfo{Name: "directory", Type: "directory", Size: 100, ModTime: now}},
	})
	if err != nil {
		t.Fatalf("Failed to insert test data for /anotherpath/: %v", err)
	}

	testIndexInitialized = true
	return &testIndex
}

func TestGetFileMetadataSize(t *testing.T) {
	idx := setupMutateTestIndex(t)
	t.Parallel()

	tests := []struct {
		name         string
		adjustedPath string
		expectedName string
		expectedSize int64
	}{
		{
			name:         "testpath exists",
			adjustedPath: "/testpath",
			expectedName: "testfile.txt",
			expectedSize: 100,
		},
		{
			name:         "testpath exists directory",
			adjustedPath: "/testpath",
			expectedName: "directory",
			expectedSize: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fileInfo, exists := idx.GetReducedMetadata(tt.adjustedPath, true)
			if !exists || fileInfo == nil {
				t.Fatalf("Failed to get metadata for %s", tt.adjustedPath)
			}
			for _, item := range fileInfo.Files {
				if item.Name == tt.expectedName {
					assert.Equal(t, tt.expectedSize, item.Size)
					break
				}
			}
		})
	}
}

func TestGetFileMetadata(t *testing.T) {
	idx := setupMutateTestIndex(t)
	t.Parallel()

	tests := []struct {
		name           string
		adjustedPath   string
		expectedName   string
		expectedExists bool
		isDir          bool
	}{
		{
			name:           "testpath exists",
			adjustedPath:   "/testpath/testfile.txt",
			expectedName:   "testfile.txt",
			expectedExists: true,
		},
		{
			name:           "testpath not exists",
			adjustedPath:   "/testpath/nonexistent.txt",
			expectedName:   "nonexistent.txt",
			expectedExists: false,
		},
		{
			name:           "File exists in /anotherpath",
			adjustedPath:   "/anotherpath/afile.txt",
			expectedName:   "afile.txt",
			expectedExists: true,
		},
		{
			name:           "File does not exist in /anotherpath",
			adjustedPath:   "/anotherpath/nonexistentfile.txt",
			expectedName:   "nonexistentfile.txt",
			expectedExists: false,
		},
		{
			name:           "Directory does not exist",
			adjustedPath:   "/nonexistentpath",
			expectedName:   "",
			expectedExists: false,
			isDir:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fileInfo, exists := idx.GetReducedMetadata(tt.adjustedPath, tt.isDir)
			if !exists {
				found := false
				assert.Equal(t, tt.expectedExists, found)
				return
			}
			found := false
			if tt.isDir {
				for _, item := range fileInfo.Files {
					if item.Name == tt.expectedName {
						found = true
						break
					}
				}
			} else {
				if fileInfo.Name == tt.expectedName {
					found = true
				}
			}

			assert.Equal(t, tt.expectedExists, found)
		})
	}
}

func TestUpdateFileMetadata(t *testing.T) {
	t.Parallel()
	setupMutateTestIndex(t)
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_mutate")
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	info := &iteminfo.FileInfo{
		Path: "/testpath/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "testpath",
			Type:    "directory",
			ModTime: time.Now(),
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "testfile.txt", ModTime: time.Now()}},
			{ItemInfo: iteminfo.ItemInfo{Name: "anotherfile.txt", ModTime: time.Now()}},
		},
	}

	index := &Index{
		Source: settings.Source{
			Name: "test_mutate",
			Path: "/mock/path",
		},
		db:   indexDB,
		mock: true,
	}

	success := index.UpdateMetadata(info)
	if !success {
		t.Fatalf("expected UpdateFileMetadata to succeed")
	}

	fileInfo, exists := index.GetReducedMetadata("/testpath/testfile.txt", false)
	if !exists || fileInfo.Name != "testfile.txt" {
		t.Fatalf("expected '%v' to exist in the directory metadata. %v ", info.Name, exists)
	}
}

func TestGetDirMetadata(t *testing.T) {
	idx := setupMutateTestIndex(t)
	t.Parallel()

	_, exists := idx.GetReducedMetadata("/testpath", true)
	if !exists {
		t.Fatalf("expected GetDirMetadata to return initialized metadata map")
	}

	_, exists = idx.GetReducedMetadata("/nonexistent", true)
	if exists {
		t.Fatalf("expected GetDirMetadata to return false for nonexistent directory")
	}
}

func TestSetDirectoryInfo(t *testing.T) {
	t.Parallel()
	setupMutateTestIndex(t)
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_mutate")
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	index := &Index{
		Source: settings.Source{
			Name: "test_mutate",
			Path: "/mock/path",
		},
		db:   indexDB,
		mock: true,
	}

	// Insert initial testpath directory
	testpath := &iteminfo.FileInfo{
		Path: "/testpath/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "testpath",
			Type:    "directory",
			ModTime: time.Now(),
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "testfile.txt", ModTime: time.Now()}},
			{ItemInfo: iteminfo.ItemInfo{Name: "anotherfile.txt", ModTime: time.Now()}},
		},
	}
	_ = index.db.InsertItem("test_mutate", "/testpath/", testpath)

	dir := &iteminfo.FileInfo{
		Path: "/newPath/",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "newPath",
			Type:    "directory",
			ModTime: time.Now(),
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "testfile.txt", ModTime: time.Now()}},
		},
	}
	index.UpdateMetadata(dir)
	storedDir, exists := index.GetMetadataInfo("/newPath/", true)
	if !exists || storedDir.Files[0].Name != "testfile.txt" {
		t.Fatalf("expected SetDirectoryInfo to store directory info correctly")
	}
}
