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
		{"Dot prefix followed by text", "./test", "/test"},
		{"Dot prefix followed by text", ".test", "/.test"},
		{"Hidden file at root", "/.test", "/.test"},
		{"Trailing slash removed", "/test/", "/test"},
		{"Subpath without root prefix", "/other/test", "/other/test"},
		{"Complex nested paths", "/nested/path", "/nested/path"},
		// TODO fix {"has source name as start", "/srv.tar.gz", "/srv.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &Index{Source: settings.Source{Path: "/srv"}}
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
		{"Mixed slash", "/first\\second", "/first/second"},
		{"Windows slash", "\\first\\second", "/first/second"},
		{"Windows full path", "C:\\Users\\testfolder\\nestedfolder", "/testfolder/nestedfolder"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &Index{Source: settings.Source{Path: "C:\\Users"}}
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
			idx := &Index{Source: settings.Source{Path: "/rootpath", Name: "default"}}
			result := idx.MakeIndexPath(tt.subPath)
			if result != tt.expected {
				t.Errorf("MakeIndexPath(%q)\ngot %q\nwant %q", tt.name, result, tt.expected)
			}
		})
	}
}

func BenchmarkCheckIndexExclude(b *testing.B) {
	tests := []struct {
		isDir    bool
		isHidden bool
		fullPath string
	}{
		{false, false, "/test/.test"},
		{true, false, "/test/.test"},
		{true, true, "/test/.test"},
		{false, false, "/test/filepath"},
		{false, true, "/test/filepath"},
		{true, true, "/test/filepath"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	idx := Index{
		Source: settings.Source{
			Name: "files",
			Config: settings.SourceConfig{
				IgnoreHidden: true,
				Exclude: settings.IndexFilter{
					Files:        []string{"test", "filepath", ".test", ".filepath", "test", "filepath", ".test", ".filepath"},
					Folders:      []string{"test", "filepath", ".test", ".filepath", "test", "filepath", ".test", ".filepath"},
					FileEndsWith: []string{".zip", ".tar", ".jpeg"},
				},
			},
		},
	}

	for i := 0; i < b.N; i++ {
		for _, v := range tests {
			idx.shouldSkip(v.isDir, v.isHidden, v.fullPath)
		}
	}

}
func BenchmarkCheckIndexConditionsInclude(b *testing.B) {
	tests := []struct {
		isDir    bool
		isHidden bool
		fullPath string
	}{
		{false, false, "/test/.test"},
		{true, false, "/test/.test"},
		{true, true, "/test/.test"},
		{false, false, "/test/filepath"},
		{false, true, "/test/filepath"},
		{true, true, "/test/filepath"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	idx2 := Index{
		Source: settings.Source{
			Name: "files",
			Config: settings.SourceConfig{
				IgnoreHidden: true,
				Include: settings.IndexFilter{
					Files:        []string{"test", "filepath", ".test", ".filepath", "test", "filepath", ".test", ".filepath"},
					Folders:      []string{"test", "filepath", ".test", ".filepath", "test", "filepath", ".test", ".filepath"},
					FileEndsWith: []string{".zip", ".tar", ".jpeg"},
				},
			},
		},
	}

	for i := 0; i < b.N; i++ {
		for _, v := range tests {
			idx2.shouldSkip(v.isDir, v.isHidden, v.fullPath)
		}
	}

}
