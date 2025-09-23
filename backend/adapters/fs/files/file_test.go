package files

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
)

func Test_GetRealPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	trimPrefix := filepath.Dir(filepath.Dir(cwd))
	tests := []struct {
		name  string
		paths []string
		want  struct {
			path  string
			isDir bool
		}
	}{
		{
			name: "current directory",
			paths: []string{
				"./",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "",
				isDir: true,
			},
		},
		{
			name: "current directory",
			paths: []string{
				"./files/file.go",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "/files/file.go",
				isDir: false,
			},
		},
		{
			name: "other test case",
			paths: []string{
				"/mnt/doesnt/exist",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "/mnt/doesnt/exist",
				isDir: false,
			},
		},
	}
	idx := indexing.Index{
		Source: settings.Source{
			Path: trimPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			realPath, isDir, _ := idx.GetRealPath(tt.paths...)
			adjustedRealPath := strings.TrimPrefix(realPath, trimPrefix)
			if tt.want.path != adjustedRealPath || tt.want.isDir != isDir {
				t.Errorf("expected %v:%v but got: %v:%v", tt.want.path, tt.want.isDir, adjustedRealPath, isDir)
			}
		})
	}
}

func TestSortItems(t *testing.T) {
	tests := []struct {
		name     string
		input    iteminfo.FileInfo
		expected iteminfo.FileInfo
	}{
		{
			name: "Numeric and Lexicographical Sorting",
			input: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "10.txt"},
					{Name: "2.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []iteminfo.ItemInfo{
					{Name: "File2.txt"},
					{Name: "File10.txt"},
					{Name: "File1"},
					{Name: "banana"},
				},
			},
			expected: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "2.txt"},
					{Name: "10.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []iteminfo.ItemInfo{
					{Name: "banana"},
					{Name: "File1"},
					{Name: "File10.txt"},
					{Name: "File2.txt"},
				},
			},
		},
		{
			name: "Only Lexicographical Sorting",
			input: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "dog.txt"},
					{Name: "Cat.txt"},
					{Name: "apple"},
				},
				Files: []iteminfo.ItemInfo{
					{Name: "Zebra"},
					{Name: "apple"},
					{Name: "cat"},
				},
			},
			expected: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "apple"},
					{Name: "Cat.txt"},
					{Name: "dog.txt"},
				},
				Files: []iteminfo.ItemInfo{
					{Name: "apple"},
					{Name: "cat"},
					{Name: "Zebra"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.input.SortItems()

			getNames := func(items []iteminfo.ItemInfo) []string {
				names := []string{}
				for _, folder := range items {
					names = append(names, folder.Name)
				}
				return names
			}

			actualFolderNames := getNames(test.input.Folders)
			expectedFolderNames := getNames(test.expected.Folders)

			if !reflect.DeepEqual(actualFolderNames, expectedFolderNames) {
				t.Errorf("Folders not sorted correctly.\nGot: %v\nExpected: %v", actualFolderNames, expectedFolderNames)
			}

			actualFileNames := getNames(test.input.Files)
			expectedFileNames := getNames(test.expected.Files)

			if !reflect.DeepEqual(actualFileNames, expectedFileNames) {
				t.Errorf("Files not sorted correctly.\nGot: %v\nExpected: %v", actualFileNames, expectedFileNames)
			}
		})
	}
}

func TestDeleteFilesCacheClearing(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filebrowser_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "Test Object")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize a mock index
	idx := indexing.Index{
		Source: settings.Source{
			Path: tempDir,
		},
	}

	// Get the real path and cache it
	_, isDir, err := idx.GetRealPath("/Test Object")
	if err != nil {
		t.Fatalf("Failed to get real path: %v", err)
	}

	// Verify the file is detected as a file (not directory)
	if isDir {
		t.Errorf("Expected file to be detected as file, but got directory")
	}

	// Initialize the index in the indexing system
	indexing.Initialize(settings.Source{
		Name: "test",
		Path: tempDir,
	}, true) // true for mock mode

	// Delete the file
	err = DeleteFiles("test", testFile, tempDir)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Create a directory with the same name
	testDir := filepath.Join(tempDir, "Test Object")
	err = os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Get the real path again - it should now detect it as a directory
	// The cache should have been cleared, so it should re-detect the type
	realPath2, isDir2, err := idx.GetRealPath("/Test Object")
	if err != nil {
		t.Fatalf("Failed to get real path after recreation: %v", err)
	}

	// Verify the directory is detected as a directory
	if !isDir2 {
		t.Errorf("Expected directory to be detected as directory, but got file. Real path: %s", realPath2)
	}

	// Clean up
	os.RemoveAll(tempDir)
}

func TestOverrideDirectoryToFile(t *testing.T) {
	// Initialize default file permissions
	fileutils.PermFile = 0644
	fileutils.PermDir = 0755

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filebrowser_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize the index in the indexing system
	indexing.Initialize(settings.Source{
		Name: "test",
		Path: tempDir,
	}, true) // true for mock mode

	// Create a directory first
	testDir := filepath.Join(tempDir, "Test Object")
	err = os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Now try to create a file with the same name (should work with override)
	fileOpts := iteminfo.FileOptions{
		Path:   "/Test Object",
		Source: "test",
	}

	// Create a test reader with some content
	testContent := "This is test file content"
	reader := strings.NewReader(testContent)

	err = WriteFile(fileOpts, reader)
	if err != nil {
		t.Fatalf("Failed to create file over directory: %v", err)
	}

	// Verify the file was created and the directory was removed
	stat, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("File should exist but got error: %v", err)
	}

	if stat.IsDir() {
		t.Errorf("Expected file but got directory")
	}

	// Verify the content
	content, err := os.ReadFile(testDir)
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %q but got %q", testContent, string(content))
	}

	// Clean up
	os.RemoveAll(tempDir)
}

func TestOverrideFileToDirectory(t *testing.T) {
	// Initialize default file permissions
	fileutils.PermFile = 0644
	fileutils.PermDir = 0755

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filebrowser_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize the index in the indexing system
	indexing.Initialize(settings.Source{
		Name: "test",
		Path: tempDir,
	}, true) // true for mock mode

	// Create a file first
	testFile := filepath.Join(tempDir, "Test Object")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Now try to create a directory with the same name (should work with override)
	fileOpts := iteminfo.FileOptions{
		Path:   "/Test Object/",
		Source: "test",
	}

	err = WriteDirectory(fileOpts)
	if err != nil {
		t.Fatalf("Failed to create directory over file: %v", err)
	}

	// Verify the directory was created and the file was removed
	stat, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Directory should exist but got error: %v", err)
	}

	if !stat.IsDir() {
		t.Errorf("Expected directory but got file")
	}

	// Clean up
	os.RemoveAll(tempDir)
}
