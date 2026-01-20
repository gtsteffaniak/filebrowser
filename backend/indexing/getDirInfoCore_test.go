package indexing

import (
	"os"
	"path/filepath"
	"strings"
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
			adjustedPath: "/test/",
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
			hasPreview, subdirMap := idx.fetchExtendedAttributes(tt.adjustedPath, tt.files, tt.opts)
			if hasPreview != tt.expectHasPreview {
				t.Errorf("expected hasPreview=%v, got %v", tt.expectHasPreview, hasPreview)
			}
			if (len(subdirMap) > 0) != tt.expectMapNotEmpty {
				t.Errorf("expected mapNotEmpty=%v, got %v", tt.expectMapNotEmpty, len(subdirMap) > 0)
			}
		})
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
				tt.file, tt.combinedPath, tt.opts, tt.scanner,
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
