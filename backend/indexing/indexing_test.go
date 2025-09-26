package indexing

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func BenchmarkFillIndex(b *testing.B) {
	Initialize(settings.Source{
		Name: "test",
		Path: "/srv",
	}, true)
	idx := GetIndex("test")
	if idx == nil {
		b.Fatal("index is nil")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		idx.CreateMockData(50, 3) // 1000 dirs, 3 files per dir
	}
}

// JSONBytesEqual compares the JSON in two byte slices.
func JSONBytesEqual(a, b []byte) (bool, error) {
	var j, j2 interface{}
	if err := json.Unmarshal(a, &j); err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, &j2); err != nil {
		return false, err
	}
	return reflect.DeepEqual(j2, j), nil
}

func TestGetIndex(t *testing.T) {
	tests := []struct {
		name string
		want *map[string][]string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetIndex("root"); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeIndexPath(t *testing.T) {
	tests := []struct {
		name     string
		subPath  string
		expected string
	}{
		// Linux
		{"Root path returns slash", "/", "/"},
		{"Dot-prefixed returns slash", ".", "/"},
		{"Double-dot prefix ignored", "./", "/"},
		{"Dot prefix followed by text", "./test", "/test/"},
		{"Dot prefix followed by text", ".test", "/.test/"},
		{"Hidden file at root", "/.test", "/.test/"},
		{"Trailing slash removed", "/test/", "/test/"},
		{"Subpath without root prefix", "/other/test", "/other/test/"},
		{"Complex nested paths", "/nested/path", "/nested/path/"},
		// TODO fix {"has source name as start", "/srv.tar.gz", "/srv.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &Index{Source: settings.Source{Path: "/srv"}, mock: true}
			result := idx.MakeIndexPath(tt.subPath)
			if result != tt.expected {
				t.Errorf("MakeIndexPath(%q)\ngot %q\nwant %q", tt.name, result, tt.expected)
			}
		})
	}

	tests = []struct {
		name     string
		subPath  string
		expected string
	}{
		// Windows
		{"Mixed slash", "/first\\second", "/first/second/"},
		{"Windows slash", "\\first\\second", "/first/second/"},
		{"Windows full path", "C:\\Users\\testfolder\\nestedfolder", "/testfolder/nestedfolder/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &Index{Source: settings.Source{Path: "C:\\Users"}, mock: true}
			result := idx.MakeIndexPath(tt.subPath)
			if result != tt.expected {
				t.Errorf("MakeIndexPath(%q)\ngot %q\nwant %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestMakeIndexPathRoot(t *testing.T) {
	tests := []struct {
		name     string
		subPath  string
		expected string
	}{
		// Linux
		{"Root path returns slash", "/rootpath", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &Index{Source: settings.Source{Path: "/rootpath", Name: "default"}, mock: true}
			result := idx.MakeIndexPath(tt.subPath)
			if result != tt.expected {
				t.Errorf("MakeIndexPath(%q)\ngot %q\nwant %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestCheckIndexExclude(t *testing.T) {
	tests := []struct {
		name     string
		isDir    bool
		isHidden bool
		fullPath string
		baseName string
		exclude  settings.ExcludeIndexFilter
		expect   bool
	}{
		// FilePaths exclusion (prefix match)
		{"FilePaths exclusion (file)", false, false, "/test/filepath/child.txt", "child.txt", settings.ExcludeIndexFilter{FilePaths: []string{"/test/filepath"}}, true},
		{"FilePaths exclusion (folder)", true, false, "/test/folderpath/sub", "sub", settings.ExcludeIndexFilter{FolderPaths: []string{"/test/folderpath"}}, true},

		// FileNames exclusion (exact match)
		{"FileNames exclusion (file)", false, false, "/test/abc.txt", "abc.txt", settings.ExcludeIndexFilter{FileNames: []string{"abc.txt"}}, true},
		{"FolderNames exclusion (folder)", true, false, "/test/.thumbnails", ".thumbnails", settings.ExcludeIndexFilter{FolderNames: []string{".thumbnails"}}, true},

		// FileEndsWith exclusion (suffix match)
		{"FileEndsWith exclusion (file)", false, false, "/test/archive.zip", "archive.zip", settings.ExcludeIndexFilter{FileEndsWith: []string{".zip"}}, true},
		// FolderEndsWith exclusion (suffix match)
		{"FolderEndsWith exclusion (folder)", true, false, "/test/special_folder", "special_folder", settings.ExcludeIndexFilter{FolderEndsWith: []string{"_folder"}}, true},

		// Negative cases (should not skip)
		{"FilePaths not excluded (file)", false, false, "/test/otherfile", "otherfile", settings.ExcludeIndexFilter{FilePaths: []string{"/test/filepath"}}, false},
		{"FileNames not excluded (file)", false, false, "/test/other.txt", "other.txt", settings.ExcludeIndexFilter{FileNames: []string{"abc.txt"}}, false},
		{"FolderNames not excluded (folder)", true, false, "/test/otherfolder", "otherfolder", settings.ExcludeIndexFilter{FolderNames: []string{".thumbnails"}}, false},
		{"FileEndsWith not excluded (file)", false, false, "/test/file.tar", "file.tar", settings.ExcludeIndexFilter{FileEndsWith: []string{".zip"}}, false},
		{"FolderEndsWith not excluded (folder)", true, false, "/test/normal", "normal", settings.ExcludeIndexFilter{FolderEndsWith: []string{"_folder"}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idx := Index{
				Source: settings.Source{
					Name: "files",
					Config: settings.SourceConfig{
						Exclude: tc.exclude,
					},
				},
			}
			result := idx.shouldSkip(tc.isDir, tc.isHidden, tc.fullPath, tc.baseName)
			if result != tc.expect {
				t.Errorf("shouldSkip(%v, %v, %q, %q) = %v; want %v", tc.isDir, tc.isHidden, tc.fullPath, tc.baseName, result, tc.expect)
			}
		})
	}
}

func TestCheckIndexInclude(t *testing.T) {
	tests := []struct {
		name     string
		isDir    bool
		fullPath string
		baseName string
		include  settings.IncludeIndexFilter
		expect   bool
	}{
		// RootFolders inclusion (prefix match)
		{"RootFolders include (folder)", true, "/folder1", "folder1", settings.IncludeIndexFilter{RootFolders: []string{"/folder1"}}, true},
		{"RootFolders include (subfolder)", true, "/folder1/sub", "sub", settings.IncludeIndexFilter{RootFolders: []string{"/folder1"}}, true},
		{"RootFolders not include (folder)", true, "/otherfolder", "otherfolder", settings.IncludeIndexFilter{RootFolders: []string{"/folder1"}}, false},

		// RootFiles inclusion (prefix match)
		{"RootFiles include (file)", false, "/file1.txt", "file1.txt", settings.IncludeIndexFilter{RootFiles: []string{"/file1.txt"}}, true},
		{"RootFiles include (file in subfolder)", false, "/folder1/file1.txt", "file1.txt", settings.IncludeIndexFilter{RootFiles: []string{"/folder1/file1.txt"}}, true},
		{"RootFiles not include (file)", false, "/otherfile.txt", "otherfile.txt", settings.IncludeIndexFilter{RootFiles: []string{"/file1.txt"}}, false},

		// No rules: should include everything
		{"No rules include (file)", false, "/anyfile.txt", "anyfile.txt", settings.IncludeIndexFilter{}, true},
		{"No rules include (folder)", true, "/anyfolder", "anyfolder", settings.IncludeIndexFilter{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idx := Index{
				Source: settings.Source{
					Name: "files",
					Config: settings.SourceConfig{
						Include: tc.include,
					},
				},
			}
			result := idx.shouldInclude(tc.isDir, tc.fullPath, tc.baseName)
			if result != tc.expect {
				t.Errorf("shouldInclude(%v, %q, %q) = %v; want %v", tc.isDir, tc.fullPath, tc.baseName, result, tc.expect)
			}
		})
	}
}
