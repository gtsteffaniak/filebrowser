package search

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PathInfo struct {
	DirPathNames   []string
	FilePathNames   []string
	LastIndexed time.Time
}
var rootPath = "/srv/"
var indexes map[string]PathInfo

func InitializeIndex(intervalMinutes uint32) {
	// Initialize the indexes map
	indexes = make(map[string]PathInfo)
	log.Println("Indexing files...")
	// Call the function to index files and directories
	err := indexFiles(rootPath, 1)
	if err != nil {
		log.Fatal(err)
	}
	go indexingScheduler(intervalMinutes)
	log.Println("Successfully indexed files.")
}

func indexingScheduler(intervalMinutes uint32) {
	for {
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		err := indexFiles(rootPath, 1)
		if err != nil {
			log.Fatal(err)
		}
	}
}


// Define a function to recursively index files and directories
func indexFiles(path string, depth int) error {
	// Check if the current directory has been modified since last indexing
	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		delete(indexes, path)
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(indexes[path].LastIndexed) {
		return nil
	}
	// Check if the directory path is more than 3 levels deep
	if depth > 3 {
		// Index the directory and its subdirectories
		err = indexEverythingFlattened(path)
		if err != nil {
			return err
		}
		return err
	}
	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}
	// Iterate over the files and directories
	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		if file.IsDir() {
			// Recursively index subdirectories
			err = indexFiles(filePath, depth+1)
		} else {
			addToIndex(path, filePath, file.ModTime(),file.IsDir())
		}
	}
	return nil
}

func indexEverythingFlattened(path string) error {
	// Index the directory and its subdirectories
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		addToIndex(path, filePath, info.ModTime(),info.IsDir())
		return nil
	})
	return err
}

func addToIndex(path string, filePath string, lastModified time.Time, isDir bool) {
	filePath = strings.TrimPrefix(filePath, rootPath)
	currentTime := time.Now()
	info, exists := indexes[path]
	if !exists {
		info = PathInfo{}
	}
	if isDir {
		info.DirPathNames = append(info.DirPathNames, filePath)
	}else{
		info.FilePathNames = append(info.FilePathNames, filePath)
	}
	info.LastIndexed = currentTime
	indexes[path] = info
}

func searchAllIndexes(searchTerm string,isDir bool,scope string) []string {
	var matchingResults []string
	// Iterate over the indexes
	for _, subFiles := range indexes {
		searchItems := subFiles.FilePathNames
		if isDir {
			searchItems = subFiles.DirPathNames
		}
		// Iterate over the path names
		for _, pathName := range searchItems {
			// Check if the path name contains the search term
			if !containsSearchTerm(pathName, searchTerm) {
				continue
			}
			pathName = scopedPathNameFilter(pathName,scope)
			if pathName == "" {
				continue
			}
			matchingResults = append(matchingResults, pathName)
		}
	}
	return matchingResults
}

func scopedPathNameFilter(pathName string, scope string) string {
	scope = strings.TrimPrefix(scope, "/")
	if strings.HasPrefix(pathName, scope) {
		pathName = strings.TrimPrefix(pathName, scope)
	} else {
		pathName = ""
	}
	return pathName
}

func containsSearchTerm(pathName string, searchTerm string) bool {
	path := getLastPathComponent(pathName)
	// Perform case-insensitive search
	pathNameLower := strings.ToLower(path)
	searchTermLower := strings.ToLower(searchTerm)

	return strings.Contains(pathNameLower, searchTermLower)
}
func getLastPathComponent(path string) string {
	// Use filepath.Base to extract the last component of the path
	return filepath.Base(path)
}