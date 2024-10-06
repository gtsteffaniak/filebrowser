package files

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

// Mock for fs.FileInfo
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

var testIndex Index

// Test for GetFileMetadata
//func TestGetFileMetadata(t *testing.T) {
//	t.Parallel()
//	tests := []struct {
//		name           string
//		adjustedPath   string
//		fileName       string
//		expectedName   string
//		expectedExists bool
//	}{
//		{
//			name:           "testpath exists",
//			adjustedPath:   "/testpath",
//			fileName:       "testfile.txt",
//			expectedName:   "testfile.txt",
//			expectedExists: true,
//		},
//		{
//			name:           "testpath not exists",
//			adjustedPath:   "/testpath",
//			fileName:       "nonexistent.txt",
//			expectedName:   "",
//			expectedExists: false,
//		},
//		{
//			name:           "File exists in /anotherpath",
//			adjustedPath:   "/anotherpath",
//			fileName:       "afile.txt",
//			expectedName:   "afile.txt",
//			expectedExists: true,
//		},
//		{
//			name:           "File does not exist in /anotherpath",
//			adjustedPath:   "/anotherpath",
//			fileName:       "nonexistentfile.txt",
//			expectedName:   "",
//			expectedExists: false,
//		},
//		{
//			name:           "Directory does not exist",
//			adjustedPath:   "/nonexistentpath",
//			fileName:       "testfile.txt",
//			expectedName:   "",
//			expectedExists: false,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			fileInfo, exists := testIndex.GetFileMetadata(tt.adjustedPath)
//			if exists != tt.expectedExists || fileInfo.Name != tt.expectedName {
//				t.Errorf("expected %v:%v but got: %v:%v", tt.expectedName, tt.expectedExists, //fileInfo.Name, exists)
//			}
//		})
//	}
//}

// Test for UpdateFileMetadata
func TestUpdateFileMetadata(t *testing.T) {
	index := &Index{
		Directories: map[string]Directory{
			"/testpath": {
				Metadata: map[string]FileInfo{
					"testfile.txt":    {Name: "testfile.txt"},
					"anotherfile.txt": {Name: "anotherfile.txt"},
				},
			},
		},
	}

	info := FileInfo{Name: "testfile.txt"}

	success := index.UpdateFileMetadata("/testpath", info)
	if !success {
		t.Fatalf("expected UpdateFileMetadata to succeed")
	}

	dir, exists := index.Directories["/testpath"]
	if !exists || dir.Metadata["testfile.txt"].Name != "testfile.txt" {
		t.Fatalf("expected testfile.txt to be updated in the directory metadata")
	}
}

// Test for GetDirMetadata
func TestGetDirMetadata(t *testing.T) {
	t.Parallel()
	_, exists := testIndex.GetMetadataInfo("/testpath")
	if !exists {
		t.Fatalf("expected GetDirMetadata to return initialized metadata map")
	}

	_, exists = testIndex.GetMetadataInfo("/nonexistent")
	if exists {
		t.Fatalf("expected GetDirMetadata to return false for nonexistent directory")
	}
}

// Test for SetDirectoryInfo
func TestSetDirectoryInfo(t *testing.T) {
	index := &Index{
		Directories: map[string]Directory{
			"/testpath": {
				Metadata: map[string]FileInfo{
					"testfile.txt":    {Name: "testfile.txt"},
					"anotherfile.txt": {Name: "anotherfile.txt"},
				},
			},
		},
	}
	dir := Directory{Metadata: map[string]FileInfo{"testfile.txt": {Name: "testfile.txt"}}}
	index.SetDirectoryInfo("/newPath", dir)
	storedDir, exists := index.Directories["/newPath"]
	if !exists || storedDir.Metadata["testfile.txt"].Name != "testfile.txt" {
		t.Fatalf("expected SetDirectoryInfo to store directory info correctly")
	}
}

// Test for GetDirectoryInfo
func TestGetDirectoryInfo(t *testing.T) {
	t.Parallel()
	dir, exists := testIndex.GetDirectoryInfo("/testpath")
	if !exists || dir.Metadata["testfile.txt"].Name != "testfile.txt" {
		t.Fatalf("expected GetDirectoryInfo to return correct directory info")
	}

	_, exists = testIndex.GetDirectoryInfo("/nonexistent")
	if exists {
		t.Fatalf("expected GetDirectoryInfo to return false for nonexistent directory")
	}
}

// Test for RemoveDirectory
func TestRemoveDirectory(t *testing.T) {
	index := &Index{
		Directories: map[string]Directory{
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
		NumFiles:   10,
		NumDirs:    5,
		inProgress: false,
		Directories: map[string]Directory{
			"/testpath": {
				Metadata: map[string]FileInfo{
					"testfile.txt":    {Name: "testfile.txt"},
					"anotherfile.txt": {Name: "anotherfile.txt"},
				},
			},
			"/anotherpath": {
				Metadata: map[string]FileInfo{
					"afile.txt": {Name: "afile.txt"},
				},
			},
		},
	}

	files := []fs.FileInfo{
		mockFileInfo{name: "file1.txt", isDir: false},
		mockFileInfo{name: "dir1", isDir: true},
	}
}
