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
	Initialize(settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")

	idx.CreateMockData(50, 3) // 50 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := utils.GenerateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			idx.Search(term, "/", "test")
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
	Initialize(settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")

	searchTerms := utils.GenerateRandomSearchTerms(10)
	for i := 0; i < 5; i++ {
		go idx.CreateMockData(100, 100) // Creating mock data concurrently
		for _, term := range searchTerms {
			go idx.Search(term, "/", "test") // Search concurrently
		}
	}
}

func TestSearchIndexes(t *testing.T) {
	index := Index{
		Directories: map[string]*iteminfo.FileInfo{
			"/":           {Files: []iteminfo.ItemInfo{{Name: "audio-one.wav", Type: "audio"}}},
			"/test/":      {Files: []iteminfo.ItemInfo{{Name: "audio-one.wav", Type: "audio"}}},
			"/test/path/": {Files: []iteminfo.ItemInfo{{Name: "file.txt", Type: "text"}}},
			"/new/test/": {Files: []iteminfo.ItemInfo{
				{Name: "audio.wav", Type: "audio"},
				{Name: "video.mp4", Type: "video"},
				{Name: "video.MP4", Type: "video"},
			}},
			"/first Dir/": {
				Files: []iteminfo.ItemInfo{
					{Name: "space jam.zip", Size: 100, Type: "archive"},
				},
			},
			"/new/test/path/": {Files: []iteminfo.ItemInfo{{Name: "archive.zip", Type: "archive"}}},
			"/firstDir/": {
				Files: []iteminfo.ItemInfo{
					{Name: "archive.zip", Size: 100, Type: "archive"},
				},
				Folders: []iteminfo.ItemInfo{
					{Name: "thisIsDir", Type: "directory", Size: 2 * 1024 * 1024},
				},
			},
			"/firstDir/thisIsDir/": {
				Files: []iteminfo.ItemInfo{
					{Name: "hi.txt", Type: "text"},
				},
				ItemInfo: iteminfo.ItemInfo{
					Size: 2 * 1024 * 1024,
				},
			},
		},
	}

	tests := []struct {
		search         string
		scope          string
		expectedResult []SearchResult
	}{
		{
			search: "audio",
			scope:  "/new/",
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
			},
		},
		{
			search: "archive",
			scope:  "/",
			expectedResult: []SearchResult{
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
			scope:  "/firstDir",
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
			expectedResult: []SearchResult{
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
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			result := index.Search(tt.search, tt.scope, "")
			assert.ElementsMatch(t, tt.expectedResult, result)
		})
	}
}
