package index

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func createMockData(node *TrieNode, numLevels, numDirs, numFilesPerDir int) {
	if numLevels == 0 {
		return
	}
	rootPath := "srv"
	for i := 0; i < numDirs; i++ {
		dirName := getRandomTerm()
		addToIndex(node, rootPath, dirName)

		for j := 0; j < numFilesPerDir; j++ {
			fileName := "file-" + getRandomTerm() + getRandomExtension()
			addToIndex(node, dirName, fileName)
		}

		// Recursively create data for subdirectories
		createMockData(node.Children[dirName], numLevels-1, numDirs, numFilesPerDir)
	}
}

// Usage in the benchmark function:
func BenchmarkFillIndex(b *testing.B) {
	indexes = Index{
		Root: &TrieNode{
			Children: make(map[string]*TrieNode),
			IsDir:    true,
		},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		createMockData(indexes.Root, 5, 2, 2) // 5 levels * 5 dirs per level * 3 files per dir
	}
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
			if got := GetIndex(); !reflect.DeepEqual(got, tt.want) {
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
func PrintIndexPaths(node *TrieNode, currentPath string, count int) {
	if node == nil {
		return
	}
	count += 1
	// Print the current path
	if currentPath != "" {
		fmt.Println(strconv.Itoa(count) + " " + currentPath)
	}

	// Iterate over the children
	for name, child := range node.Children {
		count += 1
		// If the child is a directory, continue to traverse
		if child.IsDir {
			PrintIndexPaths(child, currentPath+"/"+name, count)
		} else {
			// If it's a file, print the path
			fmt.Println(currentPath + "/" + name)
		}
	}
}
