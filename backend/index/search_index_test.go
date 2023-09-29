package index

import (
	"reflect"
	"testing"
)

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

func TestSearchIndexes(t *testing.T) {
	type args struct {
		search        string
		scope         string
		sourceSession string
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 map[string]map[string]bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := indexes.Search(tt.args.search, tt.args.scope, tt.args.sourceSession)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchAllIndexes() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("SearchAllIndexes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_scopedPathNameFilter(t *testing.T) {
	type args struct {
		pathName string
		scope    string
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
			if got := scopedPathNameFilter(tt.args.pathName, tt.args.scope); got != tt.want {
				t.Errorf("scopedPathNameFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_containsSearchTerm(t *testing.T) {
	type args struct {
		pathName   string
		searchTerm string
		options    SearchOptions
		isDir      bool
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		want1 map[string]bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := containsSearchTerm(tt.args.pathName, tt.args.searchTerm, tt.args.options, tt.args.isDir)
			if got != tt.want {
				t.Errorf("containsSearchTerm() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("containsSearchTerm() got1 = %v, want %v", got1, tt.want1)
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
