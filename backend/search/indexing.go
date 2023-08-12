package search

import (
	"log"
	"math/rand"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	sessionInProgress sync.Map // Track IPs with requests in progress
	rootPath          string   = "/srv"
	indexes           map[string][]string
	mutex             sync.RWMutex
	lastIndexed       time.Time
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
	path = strings.TrimSuffix(path, "/") + "/"
	if path == "" {
		path = "/"
	}
	if isDir {
		indexes[path] = []string{}
	} else {
		indexes[path] = append(indexes[path], fileName)
	}
}

func SearchAllIndexes(search string, scope string) ([]string, map[string]map[string]bool) {
	sourceSession := "0.0.0.0"
	runningHash := generateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map

	searchOptions := ParseSearch(search)
	mutex.RLock()
	defer mutex.RUnlock()
	fileListTypes := make(map[string]map[string]bool)
	var matching []string
	// 250 items total seems like a reasonable limit
	maximum := 250
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		// Create a reused fileType map
		reusedFileType := map[string]bool{}
		// Iterate over the indexes
		count := 0
		for pathName, files := range indexes {
			if count > maximum {
				break
			}
			// this is here to terminate a search if a new one has started
			// currently limited to one search per container, should be session based
			value, found := sessionInProgress.Load(sourceSession)
			if !found || value != runningHash {
				return []string{}, map[string]map[string]bool{}
			}
			pathName = scopedPathNameFilter(pathName, scope)
			if pathName == "" {
				continue
			}
			// check if dir matches
			matches, fileType := containsSearchTerm(pathName, searchTerm, *searchOptions, false)
			if matches {
				matching = append(matching, pathName)
				fileListTypes[pathName] = fileType
				count++
			}
			for _, fileName := range files {
				// check if file matches
				matches, fileType := containsSearchTerm(pathName+fileName, searchTerm, *searchOptions, false)
				if !matches {
					continue
				}
				matching = append(matching, pathName+fileName)
				// Clear and reuse the fileType map
				for key := range reusedFileType {
					delete(reusedFileType, key)
				}
				for key, value := range fileType {
					reusedFileType[key] = value
				}
				fileListTypes[pathName] = copyFileTypeMap(reusedFileType)
				count++
			}
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

func copyFileTypeMap(src map[string]bool) map[string]bool {
	dest := make(map[string]bool, len(src))
	for key, value := range src {
		dest[key] = value
	}
	return dest
}

func scopedPathNameFilter(pathName string, scope string) string {
	if strings.HasPrefix(pathName, scope) {
		return strings.TrimPrefix(pathName, scope)
	}
	return ""
}

func containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool) (bool, map[string]bool) {
	conditions := options.Conditions
	path := getLastPathComponent(pathName)
	// Convert to lowercase once
	lowerPath := path
	lowerSearchTerm := searchTerm
	if !conditions["exact"] {
		lowerPath = strings.ToLower(path)
		lowerSearchTerm = strings.ToLower(searchTerm)
	}
	if strings.Contains(lowerPath, lowerSearchTerm) {
		// Reuse the fileTypes map and clear its values
		fileTypes := map[string]bool{
			"audio":   false,
			"image":   false,
			"video":   false,
			"doc":     false,
			"archive": false,
			"dir":     false,
		}

		// Calculate fileSize only if needed
		var fileSize int64
		if conditions["larger"] || conditions["smaller"] {
			fileSize = getFileSize(pathName)
		}

		matchesAllConditions := true
		extension := filepath.Ext(lowerPath)
		mimetype := mime.TypeByExtension(extension)
		fileTypes["audio"] = strings.HasPrefix(mimetype, "audio")
		fileTypes["image"] = strings.HasPrefix(mimetype, "image")
		fileTypes["video"] = strings.HasPrefix(mimetype, "video")
		fileTypes["doc"] = isDoc(extension)
		fileTypes["archive"] = isArchive(extension)
		fileTypes["dir"] = isDir

		for t, v := range conditions {
			if t == "exact" {
				continue
			}
			var matchesCondition bool
			switch t {
			case "larger":
				matchesCondition = fileSize > int64(options.Size)*1000000
			case "smaller":
				matchesCondition = fileSize < int64(options.Size)*1000000
			default:
				matchesCondition = v == fileTypes[t]
			}
			if !matchesCondition {
				matchesAllConditions = false
			}
		}

		return matchesAllConditions, fileTypes
	}
	// Clear variables and return
	return false, map[string]bool{}
}

func isDoc(extension string) bool {
	for _, typefile := range documentTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func getFileSize(filepath string) int64 {
	fileInfo, err := os.Stat(rootPath + "/" + filepath)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
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

func generateRandomHash(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(rand.Int63()) // Automatically seeded based on current time
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func stringExistsInArray(target string, strings []string) bool {
	for _, s := range strings {
		if s == target {
			return true
		}
	}
	return false
}
