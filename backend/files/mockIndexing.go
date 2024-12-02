package files

import (
	"time"

	"math/rand"
)

func CreateMockData(numDirs, numFilesPerDir int) FileInfo {
	dir := FileInfo{}
	dir.Path = "/here/is/your/mock/dir"
	for i := 0; i < numDirs; i++ {
		newFile := ItemInfo{
			Name:    "file-" + getRandomTerm() + getRandomExtension(),
			Size:    rand.Int63n(1000),                                          // Random size
			ModTime: time.Now().Add(-time.Duration(rand.Intn(100)) * time.Hour), // Random mod time
			Type:    "blob",
		}
		dir.Folders = append(dir.Folders, newFile)
	}
	// Simulating files and directories with FileInfo
	for j := 0; j < numFilesPerDir; j++ {
		newFile := ItemInfo{
			Name:    "file-" + getRandomTerm() + getRandomExtension(),
			Size:    rand.Int63n(1000),                                          // Random size
			ModTime: time.Now().Add(-time.Duration(rand.Intn(100)) * time.Hour), // Random mod time
			Type:    "blob",
		}
		dir.Files = append(dir.Files, newFile)
	}
	return dir
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
