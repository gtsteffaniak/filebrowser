package files

import (
	"testing"

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
			// Iterate over fileInfo.Items to look for expectedName
			for _, item := range fileInfo.Items {
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
				// Iterate over fileInfo.Items to look for expectedName
				for _, item := range fileInfo.Items {
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
	info := &FileInfo{
		Path: "/testpath",
		Name: "testpath",
		Type: "directory",
		Files: []ReducedItem{
			{Name: "testfile.txt"},
			{Name: "anotherfile.txt"},
		},
	}

	index := &Index{
		Directories: map[string]*FileInfo{
			"/testpath": info,
		},
	}

	success := index.UpdateMetadata(info)
	if !success {
		t.Fatalf("expected UpdateFileMetadata to succeed")
	}

	fileInfo, exists := index.GetReducedMetadata("/testpath/testfile.txt", false)
	if !exists || fileInfo.Name != "testfile.txt" {
		t.Fatalf("expected testfile.txt to be updated in the directory metadata:%v %v", exists, info.Name)
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
		Directories: map[string]*FileInfo{
			"/testpath": {
				Path: "/testpath",
				Name: "testpath",
				Type: "directory",
				Items: []ReducedItem{
					{Name: "testfile.txt"},
					{Name: "anotherfile.txt"},
				},
			},
		},
	}
	dir := &FileInfo{
		Path: "/newPath",
		Name: "newPath",
		Type: "directory",
		Items: []ReducedItem{
			{Name: "testfile.txt"},
		},
	}
	index.UpdateMetadata(dir)
	storedDir, exists := index.Directories["/newPath"]
	if !exists || storedDir.Items[0].Name != "testfile.txt" {
		t.Fatalf("expected SetDirectoryInfo to store directory info correctly")
	}
}

// Test for RemoveDirectory
func TestRemoveDirectory(t *testing.T) {
	index := &Index{
		Directories: map[string]*FileInfo{
			"/testpath": {},
		},
	}
	index.RemoveDirectory("/testpath")
	_, exists := index.Directories["/testpath"]
	if exists {
		t.Fatalf("expected directory to be removed")
	}
}

// Test for UpdateCount
func TestUpdateCount(t *testing.T) {
	index := &Index{}
	index.UpdateCount("files")
	if index.NumFiles != 1 {
		t.Fatalf("expected NumFiles to be 1 after UpdateCount('files')")
	}
	if index.NumFiles != 1 {
		t.Fatalf("expected NumFiles to be 1 after UpdateCount('files')")
	}
	index.UpdateCount("dirs")
	if index.NumDirs != 1 {
		t.Fatalf("expected NumDirs to be 1 after UpdateCount('dirs')")
	}
	index.UpdateCount("unknown")
	// Just ensure it does not panic or update any counters
	if index.NumFiles != 1 || index.NumDirs != 1 {
		t.Fatalf("expected counts to remain unchanged for unknown type")
	}
	index.resetCount()
	if index.NumFiles != 0 || index.NumDirs != 0 || !index.inProgress {
		t.Fatalf("expected resetCount to reset counts and set inProgress to true")
	}
}

func init() {
	testIndex = Index{
		Root:       "/",
		NumFiles:   10,
		NumDirs:    5,
		inProgress: false,
		Directories: map[string]*FileInfo{
			"/testpath": {
				Path: "/testpath",
				Name: "testpath",
				Type: "directory",
				Files: []ReducedItem{
					{Name: "testfile.txt", Size: 100},
					{Name: "anotherfile.txt", Size: 100},
				},
			},
			"/anotherpath": {
				Path: "/anotherpath",
				Name: "anotherpath",
				Type: "directory",
				Files: []ReducedItem{
					{Name: "afile.txt", Size: 100},
				},
				Dirs: []ReducedItem{
					{Name: "directory", Type: "directory", Size: 100},
				},
			},
		},
	}
}
