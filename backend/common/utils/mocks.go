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
