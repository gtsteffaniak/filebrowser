package files

import (
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	sessionInProgress = make(map[string]string)
	sessionMutex      sync.Mutex
	maxSearchResults  = 100
)

func (si *Index) Search(search string, scope string, sourceSession string) ([]string, map[string]map[string]bool) {
	if scope == "" {
		scope = "/"
	}
	runningHash := generateRandomHash(4)

	// Lock sessionMutex before accessing/updating sessionInProgress
	sessionMutex.Lock()
	sessionInProgress[sourceSession] = runningHash
	sessionMutex.Unlock()

	searchOptions := ParseSearch(search)
	fileListTypes := make(map[string]map[string]bool)
	matching := []string{}
	count := 0

	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		matching = si.searchEachDirectory(searchTerm, scope, sourceSession, runningHash, *searchOptions, &count, &fileListTypes)
	}

	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matching, func(i, j int) bool {
		parts1 := strings.Split(matching[i], "/")
		parts2 := strings.Split(matching[j], "/")
		return len(parts1) < len(parts2)
	})
	return matching, fileListTypes
}

func (si *Index) searchEachDirectory(searchTerm, scope, sourceSession string, runningHash string, searchOptions SearchOptions, count *int, fileListTypes *map[string]map[string]bool) []string {
	si.mu.Lock()
	defer si.mu.Unlock()
	matching := []string{}
	for dirName, dir := range si.Directories {
		isDir := true
		files := strings.Split(dir.Files, ";")
		pathName := scopedPathNameFilter(dirName, scope, isDir)

		if pathName == "" {
			continue // path not matched
		}

		// Lock sessionMutex before checking sessionInProgress
		sessionMutex.Lock()
		value, found := sessionInProgress[sourceSession]
		sessionMutex.Unlock()

		if !found || value != runningHash {
			return []string{}
		}

		if *count > maxSearchResults {
			break
		}

		fileTypes := map[string]bool{}
		matches, fileType := containsSearchTerm(dirName, searchTerm, searchOptions, isDir, fileTypes)
		if matches {
			updateTypes(*fileListTypes, pathName, fileType)
			matching = append(matching, pathName)
			*count++
		}

		isDir = false

		for _, file := range files {
			if file == "" {
				continue
			}
			sessionMutex.Lock()
			value, found := sessionInProgress[sourceSession]
			sessionMutex.Unlock()

			if !found || value != runningHash {
				return []string{}
			}

			if *count > maxSearchResults {
				break
			}
			fullName := pathName + file
			fileTypes := map[string]bool{}
			matches, fileType := containsSearchTerm(fullName, searchTerm, searchOptions, isDir, fileTypes)
			if matches {
				updateTypes(*fileListTypes, fullName, fileType)
				matching = append(matching, fullName)
			}
		}
	}
	return matching
}

func scopedPathNameFilter(pathName string, scope string, isDir bool) string {
	scope = strings.TrimPrefix(scope, "/")
	pathName = strings.TrimPrefix(pathName, "/")
	pathName = strings.TrimSuffix(pathName, "/")
	if strings.HasPrefix(pathName, scope) {
		pathName = strings.TrimPrefix(pathName, scope)
		if isDir {
			pathName = pathName + "/"
		}
	} else {
		pathName = "" // return not matched
	}
	return pathName
}

func containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool, fileTypes map[string]bool) (bool, map[string]bool) {
	conditions := options.Conditions
	path := getLastPathComponent(pathName)
	// Convert to lowercase once
	if !conditions["exact"] {
		path = strings.ToLower(path)
		searchTerm = strings.ToLower(searchTerm)
	}
	if strings.Contains(path, searchTerm) {
		// Calculate fileSize only if needed
		var fileSize int64
		matchesAllConditions := true
		extension := filepath.Ext(path)
		for _, k := range AllFiletypeOptions {
			if IsMatchingType(extension, k) {
				fileTypes[k] = true
			}
		}
		fileTypes["dir"] = isDir
		for t, v := range conditions {
			if t == "exact" {
				continue
			}
			var matchesCondition bool
			switch t {
			case "larger":
				if fileSize == 0 {
					fileSize = getFileSize(pathName)
				}
				matchesCondition = fileSize > int64(options.LargerThan)*bytesInMegabyte
			case "smaller":
				if fileSize == 0 {
					fileSize = getFileSize(pathName)
				}
				matchesCondition = fileSize < int64(options.SmallerThan)*bytesInMegabyte
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

func getFileSize(filepath string) int64 {
	fileInfo, err := os.Stat(rootPath + "/" + filepath)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func getLastPathComponent(path string) string {
	// Use filepath.Base to extract the last component of the path
	return filepath.Base(path)
}

func generateRandomHash(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
