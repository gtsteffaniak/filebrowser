package search

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	rootPath    = "/srv" // DO NOT include trailing slash
	indexes     map[string][]string
	mutex       sync.RWMutex
	lastIndexed time.Time
)

func InitializeIndex(intervalMinutes uint32) {
	// Initialize the indexes map
	indexes = make(map[string][]string)
	var numFiles, numDirs int
	log.Println("Indexing files...")
	lastIndexedStart := time.Now()
	// Call the function to index files and directories
	totalNumFiles, totalNumDirs, err := indexFiles(rootPath, &numFiles, &numDirs)
	if err != nil {
		log.Fatal(err)
	}
	lastIndexed = lastIndexedStart
	go indexingScheduler(intervalMinutes)
	log.Println("Successfully indexed files.")
	log.Println("Files found       :", totalNumFiles)
	log.Println("Directories found :", totalNumDirs)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing scheduler will run every %v minutes", intervalMinutes)
	for {
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		var numFiles, numDirs int
		lastIndexedStart := time.Now()
		totalNumFiles, totalNumDirs, err := indexFiles(rootPath, &numFiles, &numDirs)
		if err != nil {
			log.Fatal(err)
		}
		lastIndexed = lastIndexedStart
		if totalNumFiles+totalNumDirs > 0 {
			log.Println("re-indexing found changes and updated the index.")
		}
	}
}

// Define a function to recursively index files and directories
func indexFiles(path string, numFiles *int, numDirs *int) (int, int, error) {
	// Check if the current directory has been modified since last indexing
	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		delete(indexes, path)
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return *numFiles, *numDirs, err
	}
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(lastIndexed) {
		return *numFiles, *numDirs, nil
	}
	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return *numFiles, *numDirs, err
	}
	// Iterate over the files and directories
	for _, file := range files {
		if file.IsDir() {
			*numDirs++
			indexFiles(path+"/"+file.Name(), numFiles, numDirs)
		}
		*numFiles++
		addToIndex(path, file.Name())
	}
	return *numFiles, *numDirs, nil
}

func addToIndex(path string, fileName string) {
	mutex.Lock()
	defer mutex.Unlock()
	path = strings.TrimPrefix(path, rootPath+"/")
	path = strings.TrimSuffix(path, "/")
	if path == rootPath {
		path = "/"
	}
	info, exists := indexes[path]
	if !exists {
		info = []string{}
	}
	info = append(info, fileName)
	indexes[path] = info
}

func SearchAllIndexes(search string, scope string) ([]string, []string) {
	searchOptions := ParseSearch(search)
	mutex.RLock()
	defer mutex.RUnlock()
	var matchingFiles []string
	var matchingDirs []string
	maximum := 100
	count := 0
	for _, searchTerm := range searchOptions.Terms {
		// Iterate over the indexes
		for dirName, v := range indexes {
			if count > maximum {
				break
			}
			searchItems := v
			// Iterate over the path names
			for _, pathName := range searchItems {
				if count > maximum {
					break
				}
				if dirName != "/" {
					pathName = dirName + "/" + pathName
				}
				// Check if the path name contains the search term
				if !containsSearchTerm(pathName, searchTerm, searchOptions.conditions) {
					continue
				}
				pathName = scopedPathNameFilter(pathName, scope)
				if pathName == "" {
					continue
				}
				count++
				matchingFiles = append(matchingFiles, pathName)
			}
			// Check if the path name contains the search term
			if !containsSearchTerm(dirName, searchTerm, searchOptions.conditions) {
				continue
			}
			pathName := scopedPathNameFilter(dirName, scope)
			if pathName == "" {
				continue
			}
			count++
			matchingDirs = append(matchingDirs, pathName)
		}
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matchingFiles, func(i, j int) bool {
		parts1 := strings.Split(matchingFiles[i], "/")
		parts2 := strings.Split(matchingFiles[j], "/")
		return len(parts1) < len(parts2)
	})
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matchingDirs, func(i, j int) bool {
		parts1 := strings.Split(matchingDirs[i], "/")
		parts2 := strings.Split(matchingDirs[j], "/")
		return len(parts1) < len(parts2)
	})
	return matchingFiles, matchingDirs
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

func containsSearchTerm(pathName string, searchTerm string, conditions []string) bool {
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
