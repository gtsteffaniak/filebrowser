package files

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkSearchAllIndexes(b *testing.B) {
	InitializeIndex(5, false)
	si := GetIndex(rootPath)

	si.createMockData(50, 3) // 50 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := generateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			si.Search(term, "/", "test")
		}
	}
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		input string
		want  *SearchOptions
	}{
		{
			input: "my test search",
			want: &SearchOptions{
				Conditions: map[string]bool{"exact": false},
				Terms:      []string{"my test search"},
			},
		},
		{
			input: "case:exact my|test|search",
			want: &SearchOptions{
				Conditions: map[string]bool{"exact": true},
				Terms:      []string{"my", "test", "search"},
			},
		},
		{
			input: "type:largerThan=100 type:smallerThan=1000 test",
			want: &SearchOptions{
				Conditions:  map[string]bool{"exact": false, "larger": true, "smaller": true},
				Terms:       []string{"test"},
				LargerThan:  100,
				SmallerThan: 1000,
			},
		},
		{
			input: "type:audio thisfile",
			want: &SearchOptions{
				Conditions: map[string]bool{"exact": false, "audio": true},
				Terms:      []string{"thisfile"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value := ParseSearch(tt.input)
			if !reflect.DeepEqual(value, tt.want) {
				t.Fatalf("\n got:  %+v\n want: %+v", value, tt.want)
			}
		})
	}
}

func TestSearchWhileIndexing(t *testing.T) {
	InitializeIndex(5, false)
	si := GetIndex(rootPath)

	searchTerms := generateRandomSearchTerms(10)
	for i := 0; i < 5; i++ {
		go si.createMockData(100, 100) // Creating mock data concurrently
		for _, term := range searchTerms {
			go si.Search(term, "/", "test") // Search concurrently
		}
	}
}

func TestSearchIndexes(t *testing.T) {
	index := Index{
		Directories: map[string]FileInfo{
			"test":      {Items: []ReducedItem{{Name: "audio1.wav"}}},
			"test/path": {Items: []ReducedItem{{Name: "file.txt"}}},
			"new/test": {Items: []ReducedItem{
				{Name: "audio.wav"},
				{Name: "video.mp4"},
				{Name: "video.MP4"},
			}},
			"new/test/path": {Items: []ReducedItem{{Name: "archive.zip"}}},
			"/firstDir": {Items: []ReducedItem{
				{Name: "archive.zip", Size: 100},
				{Name: "thisIsDir", Type: "directory", Size: 2 * 1024 * 1024},
			}},
			"/firstDir/thisIsDir": {
				Items: []ReducedItem{
					{Name: "hi.txt"},
				},
				Size: 2 * 1024 * 1024,
			},
		},
	}

	tests := []struct {
		search         string
		scope          string
		expectedResult []searchResult
	}{
		{
			search: "audio",
			scope:  "/new/",
			expectedResult: []searchResult{
				{
					Path: "test/audio.wav",
					Type: "audio",
					Size: 0,
				},
			},
		},
		{
			search: "test",
			scope:  "/",
			expectedResult: []searchResult{
				{
					Path: "test",
					Type: "directory",
					Size: 0,
				},
				{
					Path: "new/test",
					Type: "directory",
					Size: 0,
				},
			},
		},
		{
			search: "archive",
			scope:  "/",
			expectedResult: []searchResult{
				{
					Path: "firstDir/archive.zip",
					Type: "archive",
					Size: 100,
				},
				{
					Path: "new/test/path/archive.zip",
					Type: "archive",
					Size: 0,
				},
			},
		},
		{
			search: "arch",
			scope:  "/firstDir",
			expectedResult: []searchResult{
				{
					Path: "archive.zip",
					Type: "archive",
					Size: 100,
				},
			},
		},
		{
			search: "isdir",
			scope:  "/",
			expectedResult: []searchResult{
				{
					Path: "firstDir/thisIsDir",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "isdir type:largerThan=1",
			scope:  "/",
			expectedResult: []searchResult{
				{
					Path: "firstDir/thisIsDir",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "video",
			scope:  "/",
			expectedResult: []searchResult{
				{
					Path: "new/test/video.mp4",
					Type: "video",
					Size: 0,
				},
				{
					Path: "new/test/video.MP4",
					Type: "video",
					Size: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			result := index.Search(tt.search, tt.scope, "")
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
