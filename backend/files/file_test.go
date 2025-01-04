package files

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
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
	idx := Index{
		Source: settings.Source{
			Path: trimPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			realPath, isDir, _ := idx.GetRealPath(tt.paths...)
			fmt.Println(realPath, trimPrefix)
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
		input    FileInfo
		expected FileInfo
	}{
		{
			name: "Numeric and Lexicographical Sorting",
			input: FileInfo{
				Folders: []ItemInfo{
					{Name: "10.txt"},
					{Name: "2.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []ItemInfo{
					{Name: "File2.txt"},
					{Name: "File10.txt"},
					{Name: "File1"},
					{Name: "banana"},
				},
			},
			expected: FileInfo{
				Folders: []ItemInfo{
					{Name: "2.txt"},
					{Name: "10.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []ItemInfo{
					{Name: "banana"},
					{Name: "File1"},
					{Name: "File10.txt"},
					{Name: "File2.txt"},
				},
			},
		},
		{
			name: "Only Lexicographical Sorting",
			input: FileInfo{
				Folders: []ItemInfo{
					{Name: "dog.txt"},
					{Name: "Cat.txt"},
					{Name: "apple"},
				},
				Files: []ItemInfo{
					{Name: "Zebra"},
					{Name: "apple"},
					{Name: "cat"},
				},
			},
			expected: FileInfo{
				Folders: []ItemInfo{
					{Name: "apple"},
					{Name: "Cat.txt"},
					{Name: "dog.txt"},
				},
				Files: []ItemInfo{
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

			getNames := func(items []ItemInfo) []string {
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
