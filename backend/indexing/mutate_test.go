package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/stretchr/testify/assert"
)

var testIndex Index

// Test for GetFileMetadata// Test for GetFileMetadata
func TestGetFileMetadataSize(t *testing.T) {
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
			name:         "testpath exists",
			adjustedPath: "/testpath",
			expectedName: "directory",
			expectedSize: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileInfo, _ := testIndex.GetReducedMetadata(tt.adjustedPath, true)
			// Iterate over iteminfo.Items to look for expectedName
			for _, item := range fileInfo.Files {
				// Assert the existence and the name
				if item.Name == tt.expectedName {
					assert.Equal(t, tt.expectedSize, item.Size)
					break
				}
			}
		})
	}
}

// Test for GetFileMetadata// Test for GetFileMetadata
func TestGetFileMetadata(t *testing.T) {
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
			fileInfo, exists := testIndex.GetReducedMetadata(tt.adjustedPath, tt.isDir)
			if !exists {
				found := false
				assert.Equal(t, tt.expectedExists, found)
				return
			}
			found := false
			if tt.isDir {
				// Iterate over iteminfo.Items to look for expectedName
				for _, item := range fileInfo.Files {
					// Assert the existence and the name
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

// Test for UpdateFileMetadata
func TestUpdateFileMetadata(t *testing.T) {
	info := &iteminfo.FileInfo{
		Path: "/testpath/",
		Files: []iteminfo.ItemInfo{
			{Name: "testfile.txt"},
			{Name: "anotherfile.txt"},
		},
	}

	index := &Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/testpath/": info,
		},
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

// Test for GetDirMetadata
func TestGetDirMetadata(t *testing.T) {
	t.Parallel()
	_, exists := testIndex.GetReducedMetadata("/testpath", true)
	if !exists {
		t.Fatalf("expected GetDirMetadata to return initialized metadata map")
	}

	_, exists = testIndex.GetReducedMetadata("/nonexistent", true)
	if exists {
		t.Fatalf("expected GetDirMetadata to return false for nonexistent directory")
	}
}

// Test for SetDirectoryInfo
func TestSetDirectoryInfo(t *testing.T) {
	index := &Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/testpath/": {
				Path: "/testpath/",
				ItemInfo: iteminfo.ItemInfo{
					Name: "testpath",
					Type: "directory",
				},
				Files: []iteminfo.ItemInfo{
					{Name: "testfile.txt"},
					{Name: "anotherfile.txt"},
				},
			},
		},
	}
	dir := &iteminfo.FileInfo{
		Path: "/newPath/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "newPath",
			Type: "directory",
		},
		Files: []iteminfo.ItemInfo{
			{Name: "testfile.txt"},
		},
	}
	index.UpdateMetadata(dir)
	storedDir, exists := index.Directories["/newPath/"]
	if !exists || storedDir.Files[0].Name != "testfile.txt" {
		t.Fatalf("expected SetDirectoryInfo to store directory info correctly")
	}
}

func init() {
	testIndex = Index{
		ReducedIndex: ReducedIndex{
			NumFiles: 10,
			NumDirs:  5,
		},
		Source: settings.Source{
			Path: "/",
			Name: "test",
		},

		Directories: map[string]*iteminfo.FileInfo{
			"/testpath/": {
				Path: "/testpath/",
				ItemInfo: iteminfo.ItemInfo{
					Name: "testpath",
					Type: "directory",
				},
				Files: []iteminfo.ItemInfo{
					{Name: "testfile.txt", Size: 100},
					{Name: "anotherfile.txt", Size: 100},
				},
			},
			"/anotherpath/": {
				Path: "/anotherpath/",
				ItemInfo: iteminfo.ItemInfo{
					Name: "anotherpath",
					Type: "directory",
				},
				Files: []iteminfo.ItemInfo{
					{Name: "afile.txt", Size: 100},
				},
				Folders: []iteminfo.ItemInfo{
					{Name: "directory", Type: "directory", Size: 100},
				},
			},
		},
	}
}
