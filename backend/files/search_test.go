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
				Conditions:  map[string]bool{"exact": false, "larger": true},
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
			"test":      {Items: []*FileInfo{{Name: "audio1.wav"}}},
			"test/path": {Items: []*FileInfo{{Name: "file.txt"}}},
			"new/test": {Items: []*FileInfo{
				{Name: "audio.wav"},
				{Name: "video.mp4"},
				{Name: "video.MP4"},
			}},
			"new/test/path": {Items: []*FileInfo{{Name: "archive.zip"}}},
		},
	}

	tests := []struct {
		search         string
		scope          string
		expectedResult []string
		expectedTypes  map[string]map[string]bool
	}{
		{
			search:         "audio",
			scope:          "/new/",
			expectedResult: []string{"test/audio.wav"},
			expectedTypes: map[string]map[string]bool{
				"test/audio.wav": {"audio": true, "dir": false},
			},
		},
		{
			search:         "test",
			scope:          "/",
			expectedResult: []string{"test/", "new/test/"},
			expectedTypes: map[string]map[string]bool{
				"test/":     {"dir": true},
				"new/test/": {"dir": true},
			},
		},
		{
			search:         "archive",
			scope:          "/",
			expectedResult: []string{"new/test/path/archive.zip"},
			expectedTypes: map[string]map[string]bool{
				"new/test/path/archive.zip": {"archive": true, "dir": false},
			},
		},
		{
			search: "video",
			scope:  "/",
			expectedResult: []string{
				"new/test/video.mp4",
				"new/test/video.MP4",
			},
			expectedTypes: map[string]map[string]bool{
				"new/test/video.MP4": {"video": true, "dir": false},
				"new/test/video.mp4": {"video": true, "dir": false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			actualResult, actualTypes := index.Search(tt.search, tt.scope, "")
			assert.Equal(t, tt.expectedResult, actualResult)
			assert.True(t, reflect.DeepEqual(tt.expectedTypes, actualTypes))
		})
	}
}

func Test_scopedPathNameFilter(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			pathName string
			scope    string
			isDir    bool // Assuming isDir should be included in args
		}
		want string
	}{
		{
			name: "scope test",
			args: struct {
				pathName string
				scope    string
				isDir    bool
			}{
				pathName: "/",
				scope:    "/",
				isDir:    false,
			},
			want: "", // Update this with the expected result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scopedPathNameFilter(tt.args.pathName, tt.args.scope, tt.args.isDir); got != tt.want {
				t.Errorf("scopedPathNameFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
