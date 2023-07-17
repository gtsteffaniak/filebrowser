package search

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"mime"
)

var (
	rootPath 	string = "/srv"
	indexes =		map[string][]string{}
	mutex       sync.RWMutex
	lastIndexed time.Time
)

func InitializeIndex(intervalMinutes uint32) {
	// Initialize the indexes map
	indexes = make(map[string][]string)
	indexes["dirs"]  = []string{}
	indexes["files"] = []string{}
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
			addToIndex(path, file.Name(), true)
			indexFiles(path+"/"+file.Name(), numFiles, numDirs) // recursive
		} else {
			*numFiles++
			addToIndex(path, file.Name(), false)
		}
	}
	return *numFiles, *numDirs, nil
}

func addToIndex(path string, fileName string, isDir bool) {
	mutex.Lock()
	defer mutex.Unlock()
	path = strings.TrimPrefix(path, rootPath+"/")
	path = strings.TrimSuffix(path, "/")
	adjustedPath := path + "/" + fileName
	if path == rootPath {
		adjustedPath = fileName
	}
	if isDir {
		indexes["dirs"] = append(indexes["dirs"], adjustedPath)
	}else{
		indexes["files"] = append(indexes["files"], adjustedPath)
	}
}

func SearchAllIndexes(search string, scope string) ([]string, map[string]map[string]bool) {
	searchOptions := ParseSearch(search)
	mutex.RLock()
	defer mutex.RUnlock()
	fileListTypes := make(map[string]map[string]bool)
	var matching []string
	maximum := 125

	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		count := 0
		// Iterate over the indexes
		for _, dirName := range indexes["dirs"] {
			if count > maximum {
				break
			}
			pathName := scopedPathNameFilter(dirName, scope)
			if pathName == "" {
				continue
			}
			matches, fileType := containsSearchTerm(pathName, searchTerm, searchOptions.Conditions, true)
			if !matches {
				continue
			}
			count++
			matching = append(matching, pathName+"/")
			fileListTypes[pathName+"/"] = fileType
		}
		count = 0
		for _, fileName := range indexes["files"] {
			if count > maximum {
				break
			}
			pathName := scopedPathNameFilter(fileName, scope)
			if pathName == "" {
				continue
			}
			// Check if the path name contains the search term
			matches, fileType := containsSearchTerm(pathName, searchTerm, searchOptions.Conditions, false)
			if !matches {
				continue
			}
			matching = append(matching, pathName)
			fileListTypes[pathName] = fileType
			count++
		}
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matching, func(i, j int) bool {
		parts1 := strings.Split(matching[i], "/")
		parts2 := strings.Split(matching[j], "/")
		return len(parts1) < len(parts2)
	})
	return matching, fileListTypes
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

func containsSearchTerm(pathName string, searchTerm string, conditions map[string]bool, isDir bool) (bool, map[string]bool) {
	path 					:= getLastPathComponent(pathName)
	fileTypes 				:= map[string]bool{}
	matchesCondition 		:= false
	extension 				:= filepath.Ext(strings.ToLower(path))
	mimetype 				:= mime.TypeByExtension(extension)
	fileTypes["audio"] 		= strings.HasPrefix(mimetype, "audio")
	fileTypes["image"]		= strings.HasPrefix(mimetype, "image")
	fileTypes["video"] 		= strings.HasPrefix(mimetype, "video")
	fileTypes["doc"] 		= isDoc(extension)
	fileTypes["archive"] 	= isArchive(extension)
	fileTypes["dir"]		= isDir
	anyFilter 				:= false
	for t,v := range conditions {
		if t == "exact" {
			continue
		}
		matchesCondition = v == fileTypes[t]
		anyFilter = true
	}
	if !anyFilter {
		matchesCondition = true
	}
    if !conditions["exact"] {
        path = strings.ToLower(path)
        searchTerm = strings.ToLower(searchTerm)
    }
    return strings.Contains(path, searchTerm) && matchesCondition, fileTypes
}

func isDoc(extension string) bool {
	for _, typefile := range documentTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func isArchive(extension string) bool {
	for _, typefile := range compressedFile {
		if extension == typefile {
			return true
		}
	}
	return false
}

func getLastPathComponent(path string) string {
	// Use filepath.Base to extract the last component of the path
	return filepath.Base(path)
}
