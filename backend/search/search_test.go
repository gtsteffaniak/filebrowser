package search

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"
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

func BenchmarkSearchAllIndexes(b *testing.B) {
	indexes = make(map[string][]string)

	// Create mock data
	createMockData(50, 3) // 1000 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := generateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			SearchAllIndexes(term, "/", "test")
		}
	}
}

func BenchmarkFillIndex(b *testing.B) {
	indexes = make(map[string][]string)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		createMockData(50, 3) // 1000 dirs, 3 files per dir
	}
}

func createMockData(numDirs, numFilesPerDir int) {
	for i := 0; i < numDirs; i++ {
		dirName := generateRandomPath(rand.Intn(3) + 1)
		addToIndex("/", dirName, true)
		for j := 0; j < numFilesPerDir; j++ {
			fileName := "file-" + getRandomTerm() + getRandomExtension()
			addToIndex("/"+dirName, fileName, false)
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
