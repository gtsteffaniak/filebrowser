package indexing

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/stretchr/testify/assert"
)

func BenchmarkSearchAllIndexes(b *testing.B) {
	Initialize(&settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")

	idx.CreateMockData(50, 3) // 50 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := utils.GenerateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			idx.Search(term, "/", "test", false)
		}
	}
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		input string
		want  iteminfo.SearchOptions
	}{
		{
			input: "my test search",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": false},
				Terms:      []string{"my test search"},
			},
		},
		{
			input: "case:exact my|test|search",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": true},
				Terms:      []string{"my", "test", "search"},
			},
		},
		{
			input: "type:largerThan=100 type:smallerThan=1000 test",
			want: iteminfo.SearchOptions{
				Conditions:  map[string]bool{"exact": false, "larger": true, "smaller": true},
				Terms:       []string{"test"},
				LargerThan:  100,
				SmallerThan: 1000,
			},
		},
		{
			input: "type:audio thisfile",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": false, "audio": true},
				Terms:      []string{"thisfile"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value := iteminfo.ParseSearch(tt.input)
			if !reflect.DeepEqual(value, tt.want) {
				t.Fatalf("\n got:  %+v\n want: %+v", value, tt.want)
			}
		})
	}
}

func TestSearchWhileIndexing(t *testing.T) {
	Initialize(&settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")

	searchTerms := utils.GenerateRandomSearchTerms(10)
	for i := 0; i < 5; i++ {
		go idx.CreateMockData(100, 100) // Creating mock data concurrently
		for _, term := range searchTerms {
			go idx.Search(term, "/", "test", false) // Search concurrently
		}
	}
}

func TestSearchIndexes(t *testing.T) {
	index := Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/":           {Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "audio-one.wav", Type: "audio"}}}},
			"/test/":      {Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "audio-one.wav", Type: "audio"}}}},
			"/test/path/": {Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "file.txt", Type: "text"}}}},
			"/new/test/": {Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "audio.wav", Type: "audio"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "video.mp4", Type: "video"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "video.MP4", Type: "video"}},
			}},
			"/first Dir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "space jam.zip", Size: 100, Type: "archive"}},
				},
			},
			"/new/test/path/": {Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "archive.zip", Type: "archive"}}}},
			"/firstDir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "archive.zip", Size: 100, Type: "archive"}},
				},
				Folders: []iteminfo.ItemInfo{
					{Name: "thisIsDir", Type: "directory", Size: 2 * 1024 * 1024},
				},
			},
			"/firstDir/thisIsDir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "hi.txt", Type: "text"}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 2 * 1024 * 1024,
				},
			},
			"/new+folder/Pictures/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "consoletest.mp4", Size: 196091904, Type: "video/mp4"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "playwright.gif", Size: 2416640, Type: "image/gif"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "toggle.gif", Size: 65536, Type: "image/gif"}},
				},
			},
		},
	}

	tests := []struct {
		search         string
		scope          string
		expectedResult []*SearchResult
	}{
		{
			search: "audio",
			scope:  "/new/",
			expectedResult: []*SearchResult{
				{
					Path: "/new/test/audio.wav",
					Type: "audio",
					Size: 0,
				},
			},
		},
		{
			search: "test",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/test/",
					Type: "directory",
					Size: 0,
				},
				{
					Path: "/new/test/",
					Type: "directory",
					Size: 0,
				},
				{
					Path: "/new+folder/Pictures/consoletest.mp4",
					Type: "video/mp4",
					Size: 196091904,
				},
			},
		},
		{
			search: "archive",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/archive.zip",
					Type: "archive",
					Size: 100,
				},
				{
					Path: "/new/test/path/archive.zip",
					Type: "archive",
					Size: 0,
				},
			},
		},
		{
			search: "arch",
			scope:  "/firstDir/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/archive.zip",
					Type: "archive",
					Size: 100,
				},
			},
		},
		{
			search: "space jam",
			scope:  "/first Dir/",
			expectedResult: []*SearchResult{
				{
					Path: "/first Dir/space jam.zip",
					Type: "archive",
					Size: 100,
				},
			},
		},
		{
			search: "isdir",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/thisIsDir/",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "IsDir type:largerThan=1",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/thisIsDir/",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "video",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/new/test/video.MP4",
					Type: "video",
					Size: 0,
				},
				{
					Path: "/new/test/video.mp4",
					Type: "video",
					Size: 0,
				},
			},
		},
		{
			search: "audio",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/audio-one.wav",
					Type: "audio",
					Size: 0,
				},
				{
					Path: "/test/audio-one.wav",
					Type: "audio",
					Size: 0,
				},
				{
					Path: "/new/test/audio.wav",
					Type: "audio",
					Size: 0,
				},
			},
		},
		{
			search: "cons",
			scope:  "/new+folder/Pictures/",
			expectedResult: []*SearchResult{
				{
					Path: "/new+folder/Pictures/consoletest.mp4",
					Type: "video/mp4",
					Size: 196091904,
				},
			},
		},
		{
			search: "toggle",
			scope:  "/new+folder/Pictures/",
			expectedResult: []*SearchResult{
				{
					Path: "/new+folder/Pictures/toggle.gif",
					Type: "image/gif",
					Size: 65536,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			result := index.Search(tt.search, tt.scope, "", false)
			// Convert results to comparable format (without Modified field)
			expected := make([]SearchResult, len(tt.expectedResult))
			for i, r := range tt.expectedResult {
				expected[i] = SearchResult{Path: r.Path, Type: r.Type, Size: r.Size}
			}
			actual := make([]SearchResult, len(result))
			for i, r := range result {
				actual[i] = SearchResult{Path: r.Path, Type: r.Type, Size: r.Size}
			}
			assert.ElementsMatch(t, expected, actual)
		})
	}
}

func TestSearchLargestModeExcludesRoot(t *testing.T) {
	index := Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "root-file.txt", Type: "text", Size: 100}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 3209322496, // Large root directory size
				},
			},
			"/subdir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "sub-file.txt", Type: "text", Size: 200}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 5 * 1024 * 1024, // 5MB subdirectory
				},
			},
			"/another-dir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "large-file.bin", Type: "binary", Size: 10 * 1024 * 1024}}, // 10MB file
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 10 * 1024 * 1024,
				},
			},
		},
	}

	// Test that when largest=true and scope="/", the root directory "/" is NOT included
	result := index.Search("", "/", "test-session", true)

	// Verify that "/" is NOT in the results
	rootFound := false
	for _, r := range result {
		if r.Path == "/" {
			rootFound = true
			t.Errorf("Root directory '/' should not be included in results when largest=true, but found: %+v", r)
		}
	}

	if rootFound {
		t.Error("Root directory was incorrectly included in results")
	}

	// The test passes if root is excluded
	// (subdirectories and files may or may not be included depending on size filtering logic)
}

func TestSearchLargestModeExcludesScopeDirectory(t *testing.T) {
	index := Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "root-file.txt", Type: "text", Size: 50}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 100,
				},
			},
			"/test/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Type: "text", Size: 100}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 2 * 1024 * 1024, // 2MB directory
				},
			},
			"/test/subdir/": {
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Type: "text", Size: 200}},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 3 * 1024 * 1024, // 3MB subdirectory
				},
			},
		},
	}

	// Test that when largest=true and scope="/test/", the scope directory "/test/" is NOT included
	result := index.Search("", "/test/", "test-session", true)

	// Verify that "/test/" is NOT in the results
	scopeDirFound := false
	for _, r := range result {
		if r.Path == "/test/" {
			scopeDirFound = true
			t.Errorf("Scope directory '/test/' should not be included in results when largest=true, but found: %+v", r)
		}
	}

	if scopeDirFound {
		t.Error("Scope directory was incorrectly included in results")
	}

	// The main assertion: scope directory should be excluded
	// Note: When search is empty, scope gets cleared, so we search all directories
	// but still exclude the original scope directory "/test/" from results
	// Files and subdirectories may or may not be included depending on size/type filtering
}
