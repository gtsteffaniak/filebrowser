package search

import (
	"encoding/json"
	"fmt"
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
}

func BenchmarkSearchAllIndexes(b *testing.B) {
	indexes = make(map[string][]string)

	// Create mock data
	createMockData(500, 3) // 1000 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := generateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			SearchAllIndexes(term, "/")
		}
	}
	printBenchmarkResults(b)
}

func BenchmarkFillIndex(b *testing.B) {
	indexes = make(map[string][]string)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		createMockData(10000, 10) // 1000 dirs, 3 files per dir
	}
	for a, _ := range indexes {
		b.Logf(a)
	}
	printBenchmarkResults(b)
}

func createMockData(numDirs, numFilesPerDir int) {
	for i := 0; i < numDirs; i++ {
		dirName := getRandomTerm()
		addToIndex("/", dirName, true)
		for j := 0; j < numFilesPerDir; j++ {
			fileName := "file-" + getRandomTerm() + getRandomExtension()
			addToIndex("/"+dirName, fileName, false)
		}
	}
}

func getRandomTerm() string {
	wordbank := []string{
		"hi", "test", "other", "name",
		"cool", "things", "more", "items",
	}
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(wordbank))
	return wordbank[index]
}

func getRandomExtension() string {
	wordbank := []string{
		".txt", ".mp3", ".mov", ".doc",
		".mp4", ".bak", ".zip", ".jpg",
	}
	rand.Seed(time.Now().UnixNano())
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

func passedFunc(t *testing.T) {
	t.Logf("%s passed!", t.Name())
}

func formatDuration(duration time.Duration) string {
	if duration >= time.Second {
		return fmt.Sprintf("%.2f seconds", duration.Seconds())
	} else if duration >= time.Millisecond {
		return fmt.Sprintf("%.2f ms", float64(duration.Milliseconds()))
	}
	return fmt.Sprintf("%.2f ns", float64(duration.Nanoseconds()))
}

func formatMemory(bytes int64) string {
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for bytes >= 1024 && i < len(sizes)-1 {
		bytes /= 1024
		i++
	}
	return fmt.Sprintf("%d %s", bytes, sizes[i])
}

// Output the benchmark results with human-readable units
func printBenchmarkResults(b *testing.B) {
	averageTimePerIteration := b.Elapsed() / time.Duration(b.N)
	fmt.Printf("\nIterations            : %d\n", b.N)
	fmt.Printf("Total time            : %s\n", formatDuration(b.Elapsed()))
	fmt.Printf("Avg time per op       : %s\n", formatDuration(averageTimePerIteration))
}
