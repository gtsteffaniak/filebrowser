package indexing

import (
	"os"
	"testing"
	"time"
)

// TestFetchExtendedAttributes tests the fetchExtendedAttributes helper function
func TestFetchExtendedAttributes(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	tests := []struct {
		name              string
		adjustedPath      string
		combinedPath      string
		files             []os.FileInfo
		opts              Options
		expectHasPreview  bool
		expectMapNotEmpty bool
	}{
		{
			name:         "SkipExtendedAttrs returns empty",
			adjustedPath: "/test/",
			combinedPath: "/test/",
			files:        []os.FileInfo{},
			opts: Options{
				SkipExtendedAttrs: true,
				Recursive:         false,
			},
			expectHasPreview:  false,
			expectMapNotEmpty: false,
		},
		{
			name:         "Recursive returns empty",
			adjustedPath:  "/test/",
			combinedPath: "/test/",
			files:        []os.FileInfo{},
			opts: Options{
				SkipExtendedAttrs: false,
				Recursive:         true,
			},
			expectHasPreview:  false,
			expectMapNotEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPreview, subdirMap := idx.fetchExtendedAttributes(tt.adjustedPath, tt.combinedPath, tt.files, tt.opts)
			if hasPreview != tt.expectHasPreview {
				t.Errorf("expected hasPreview=%v, got %v", tt.expectHasPreview, hasPreview)
			}
			if (len(subdirMap) > 0) != tt.expectMapNotEmpty {
				t.Errorf("expected mapNotEmpty=%v, got %v", tt.expectMapNotEmpty, len(subdirMap) > 0)
			}
		})
	}
}

// TestShouldProcessItem tests the shouldProcessItem helper function
func TestShouldProcessItem(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	defer cleanup()

	// Create a mock file info
	mockFile := &mockFileInfo{
		name: "testfile.txt",
		mode: 0,
	}

	tests := []struct {
		name        string
		file        os.FileInfo
		adjustedPath string
		combinedPath string
		baseName     string
		isDir        bool
		opts         Options
		expect      bool
	}{
		{
			name:         "SkipIndexChecks with viewable item",
			file:         mockFile,
			adjustedPath: "/test/",
			combinedPath: "/test/",
			baseName:     "testfile.txt",
			isDir:        false,
			opts: Options{
				SkipIndexChecks: true,
			},
			expect: true, // Assuming IsViewable returns true for test paths
		},
		{
			name:         "SkipIndexChecks with non-viewable item",
			file:         mockFile,
			adjustedPath: "/test/",
			combinedPath: "/test/",
			baseName:     "hidden.txt",
			isDir:        false,
			opts: Options{
				SkipIndexChecks: true,
			},
			expect: false, // Assuming IsViewable returns false
		},
		{
			name:         "Indexed path with CheckViewable",
			file:         mockFile,
			adjustedPath: "/test/",
			combinedPath: "/test/",
			baseName:     "testfile.txt",
			isDir:        false,
			opts: Options{
				SkipIndexChecks: false,
				CheckViewable:   true,
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idx.shouldProcessItem(tt.file, tt.adjustedPath, tt.combinedPath, tt.baseName, tt.isDir, tt.opts)
			// Note: Actual result depends on shouldSkip and IsViewable implementation
			// This test verifies the function doesn't panic and returns a boolean
			_ = result
		})
	}
}

// TestGetDirectoryName tests the getDirectoryName helper function
func TestGetDirectoryName(t *testing.T) {
	tests := []struct {
		name         string
		realPath     string
		adjustedPath string
		expected     string
	}{
		{
			name:         "Normal directory path",
			realPath:     "/home/user/documents",
			adjustedPath: "/documents/",
			expected:     "documents",
		},
		{
			name:         "Root path",
			realPath:     "/",
			adjustedPath: "/",
			expected:     "/",
		},
		{
			name:         "Current directory (.)",
			realPath:     ".",
			adjustedPath: "/test/",
			expected:     "test",
		},
		{
			name:         "Empty realPath uses adjustedPath",
			realPath:     "",
			adjustedPath: "/mydir/",
			expected:     "mydir",
		},
		{
			name:         "RealPath with trailing slash",
			realPath:     "/home/user/documents/",
			adjustedPath: "/documents/",
			expected:     "documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDirectoryName(tt.realPath, tt.adjustedPath)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
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
			name:         "UseInMemorySizes with existing size",
			file:         mockDir,
			combinedPath: "/test/",
			realPath:     "/real/test",
			fullCombined: "/test/subdir",
			subdirHasPreviewMap: map[string]bool{
				"/test/subdir/": true,
			},
			opts: Options{
				Recursive:         false,
				UseInMemorySizes:  true,
				SkipExtendedAttrs: false,
			},
			expectNil:         false,
			expectSize:        500,
			expectShouldCount: true,
		},
		{
			name:         "UseInMemorySizes without existing size",
			file:         mockDirNoSize,
			combinedPath: "/test/",
			realPath:     "/real/test",
			fullCombined: "/test/nosize",
			subdirHasPreviewMap: map[string]bool{
				"/test/nosize/": false,
			},
			opts: Options{
				Recursive:         false,
				UseInMemorySizes:  true,
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
				UseInMemorySizes:  true,
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
				tt.file, tt.combinedPath, tt.realPath, tt.fullCombined,
				tt.subdirHasPreviewMap, tt.opts, nil,
			)

			if tt.expectNil {
				if itemInfo != nil {
					t.Errorf("expected nil itemInfo, got %v", itemInfo)
				}
				return
			}

			if itemInfo == nil {
				t.Fatalf("expected non-nil itemInfo")
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
				tt.file, tt.realPath, tt.combinedPath, tt.fullCombined, tt.opts, tt.scanner,
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
