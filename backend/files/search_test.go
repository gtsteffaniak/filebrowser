package files

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkSearchAllIndexes(b *testing.B) {
	InitializeIndex(5, false)
	si := GetIndex(rootPath)

	si.createMockData(50, 3) // 1000 dirs, 3 files per dir

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

// loop over test files and compare output
func TestParseSearch(t *testing.T) {
	value := ParseSearch("my test search")
	want := &SearchOptions{
		Conditions: map[string]bool{
			"exact": false,
		},
		Terms: []string{"my test search"},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("\n got:  %+v\n want: %+v", value, want)
	}
	value = ParseSearch("case:exact my|test|search")
	want = &SearchOptions{
		Conditions: map[string]bool{
			"exact": true,
		},
		Terms: []string{"my", "test", "search"},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("\n got:  %+v\n want: %+v", value, want)
	}
	value = ParseSearch("type:largerThan=100 type:smallerThan=1000 test")
	want = &SearchOptions{
		Conditions: map[string]bool{
			"exact":  false,
			"larger": true,
		},
		Terms:       []string{"test"},
		LargerThan:  100,
		SmallerThan: 1000,
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("\n got:  %+v\n want: %+v", value, want)
	}
	value = ParseSearch("type:audio thisfile")
	want = &SearchOptions{
		Conditions: map[string]bool{
			"exact": false,
			"audio": true,
		},
		Terms: []string{"thisfile"},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("\n got:  %+v\n want: %+v", value, want)
	}
}

func TestSearchWhileIndexing(t *testing.T) {
	InitializeIndex(5, false)
	si := GetIndex(rootPath)
	// Generate 100 random search terms
	// Generate 100 random search terms
	searchTerms := generateRandomSearchTerms(10)
	for i := 0; i < 5; i++ {
		// Execute the SearchAllIndexes function
		go si.createMockData(100, 100) // 1000 dirs, 3 files per dir
		for _, term := range searchTerms {
			go si.Search(term, "/", "test")
		}
	}
}

func TestSearchIndexes(t *testing.T) {
	index := Index{
		Directories: map[string]Directory{
			"test": {
				Files: "audio1.wav;",
			},
			"test/path": {
				Files: "file.txt;",
			},
			"new": {},
			"new/test": {
				Files: "audio.wav;video.mp4;video.MP4;",
			},
			"new/test/path": {
				Files: "archive.zip;",
			},
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
				"test/audio.wav": map[string]bool{"audio": true, "dir": false},
			},
		},
		{
			search:         "test",
			scope:          "/",
			expectedResult: []string{"test/", "new/test/"},
			expectedTypes: map[string]map[string]bool{
				"test/":     map[string]bool{"dir": true},
				"new/test/": map[string]bool{"dir": true},
			},
		},
		{
			search:         "archive",
			scope:          "/",
			expectedResult: []string{"new/test/path/archive.zip"},
			expectedTypes: map[string]map[string]bool{
				"new/test/path/archive.zip": map[string]bool{"archive": true, "dir": false},
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
				"new/test/video.MP4": map[string]bool{"video": true, "dir": false},
				"new/test/video.mp4": map[string]bool{"video": true, "dir": false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			actualResult, actualTypes := index.Search(tt.search, tt.scope, "")
			assert.Equal(t, tt.expectedResult, actualResult)
			if !reflect.DeepEqual(tt.expectedTypes, actualTypes) {
				t.Fatalf("\n got:  %+v\n want: %+v", actualTypes, tt.expectedTypes)
			}
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

func Test_isDoc(t *testing.T) {
	type args struct {
		extension string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDoc(tt.args.extension); got != tt.want {
				t.Errorf("isDoc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getFileSize(t *testing.T) {
	type args struct {
		filepath string
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFileSize(tt.args.filepath); got != tt.want {
				t.Errorf("getFileSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isArchive(t *testing.T) {
	type args struct {
		extension string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isArchive(tt.args.extension); got != tt.want {
				t.Errorf("isArchive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLastPathComponent(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLastPathComponent(tt.args.path); got != tt.want {
				t.Errorf("getLastPathComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateRandomHash(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateRandomHash(tt.args.length); got != tt.want {
				t.Errorf("generateRandomHash() = %v, want %v", got, tt.want)
			}
		})
	}
}
