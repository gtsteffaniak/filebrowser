package index

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

func BenchmarkFillIndex(b *testing.B) {
	index = Index{
		Root:              strings.TrimSuffix(settings.GlobalConfiguration.Server.Root, "/"),
		Directories:       []Directory{},
		NumDirs:           0,
		NumFiles:          0,
		currentlyIndexing: false,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		createMockData(50, 3) // 1000 dirs, 3 files per dir
	}
}
func createMockData(numDirs, numFilesPerDir int) {
	for i := 0; i < numDirs; i++ {
		dirName := generateRandomPath(rand.Intn(3) + 1)
		// Append a new Directory to the slice
		index.Directories = append(index.Directories, Directory{Name: dirName})
		for j := 0; j < numFilesPerDir; j++ {
			fileName := "file-" + getRandomTerm() + getRandomExtension()
			// Append the fileName to the Files slice within the Directory struct
			index.Directories[i].Files += fileName + ";"
		}
	}
}

func generateRandomPath(levels int) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	dirName := "srv"
	for i := 0; i < levels; i++ {
		dirName += "/" + getRandomTerm()
	}
	return dirName
}

func getRandomTerm() string {
	wordbank := []string{
		"hi", "test", "other", "name",
		"cool", "things", "more", "items",
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))

	index := rand.Intn(len(wordbank))
	return wordbank[index]
}

func getRandomExtension() string {
	wordbank := []string{
		".txt", ".mp3", ".mov", ".doc",
		".mp4", ".bak", ".zip", ".jpg",
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	index := rand.Intn(len(wordbank))
	return wordbank[index]
}

func generateRandomSearchTerms(numTerms int) []string {
	// Generate random search terms
	searchTerms := make([]string, numTerms)
	for i := 0; i < numTerms; i++ {
		searchTerms[i] = getRandomTerm()
	}
	return searchTerms
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

func TestInitializeIndex(t *testing.T) {
	type args struct {
		intervalMinutes uint32
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Initialize(tt.args.intervalMinutes)
		})
	}
}

func Test_indexingScheduler(t *testing.T) {
	type args struct {
		intervalMinutes uint32
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexingScheduler(tt.args.intervalMinutes)
		})
	}
}
