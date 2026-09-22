package utils

import (
	"time"

	"math/rand"
)

func GenerateRandomPath(levels int) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	dirName := "srv"
	for i := 0; i < levels; i++ {
		dirName += "/" + GetRandomTerm()
	}
	return dirName
}

func GetRandomTerm() string {
	wordbank := []string{
		"hi", "test", "other", "name",
		"cool", "things", "more", "items",
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))

	index := rand.Intn(len(wordbank))
	return wordbank[index]
}

func GetRandomExtension() string {
	wordbank := []string{
		".txt", ".mp3", ".mov", ".doc",
		".mp4", ".bak", ".zip", ".jpg",
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	index := rand.Intn(len(wordbank))
	return wordbank[index]
}

func GenerateRandomSearchTerms(numTerms int) []string {
	// Generate random search terms
	searchTerms := make([]string, numTerms)
	for i := 0; i < numTerms; i++ {
		searchTerms[i] = GetRandomTerm()
	}
	return searchTerms
}

// Seeded variants used by mock listing generation so performance runs are
// byte-for-byte reproducible. Adding a seeded random source here rather than
// mutating the global rand keeps existing callers untouched.

var mockTerms = []string{
	"hi", "test", "other", "name",
	"cool", "things", "more", "items",
}

var mockExtensions = []string{
	".txt", ".mp3", ".mov", ".doc",
	".mp4", ".bak", ".zip", ".jpg",
}

// GetRandomTermSeeded returns a term from an explicit random source.
func GetRandomTermSeeded(r *rand.Rand) string {
	return mockTerms[r.Intn(len(mockTerms))]
}

// GetRandomExtensionSeeded returns an extension from an explicit random source.
func GetRandomExtensionSeeded(r *rand.Rand) string {
	return mockExtensions[r.Intn(len(mockExtensions))]
}

// GenerateRandomPathSeeded builds a path from an explicit random source.
func GenerateRandomPathSeeded(r *rand.Rand, levels int) string {
	dirName := "srv"
	for i := 0; i < levels; i++ {
		dirName += "/" + GetRandomTermSeeded(r)
	}
	return dirName
}
