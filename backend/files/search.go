package files

import (
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	sessionInProgress sync.Map
	maxSearchResults  = 100
)

func (si *Index) Search(search string, scope string, sourceSession string) ([]string, map[string]map[string]bool) {
	// Remove slashes
	scope = strings.TrimLeft(scope, "/")
	scope = strings.TrimRight(scope, "/")
	runningHash := generateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	fileListTypes := make(map[string]map[string]bool)
	matching := []string{}
	count := 0
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		si.mu.Lock()
		for dirName, dir := range si.Directories {
			isDir := true
			files := []string{}
			for _, item := range dir.Items {
				if !item.IsDir {
					files = append(files, item.Name)
				}
			}
			value, found := sessionInProgress.Load(sourceSession)
			if !found || value != runningHash {
				si.mu.Unlock()
				return []string{}, map[string]map[string]bool{}
			}
			if count > maxSearchResults {
				break
			}
			pathName := scopedPathNameFilter(dirName, scope, isDir)
			if pathName == "" {
				continue // path not matched
			}
			fileTypes := map[string]bool{}
			si.mu.Unlock()
			matches, fileType := si.containsSearchTerm(dirName, searchTerm, *searchOptions, isDir, fileTypes)
			si.mu.Lock()
			if matches {
				fileListTypes[pathName] = fileType
				matching = append(matching, pathName)
				count++
			}
			isDir = false
			for _, file := range files {
				if file == "" {
					continue
				}
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					return []string{}, map[string]map[string]bool{}
				}

				if count > maxSearchResults {
					break
				}
				fullName := strings.TrimLeft(pathName+file, "/")
				fileTypes := map[string]bool{}
				si.mu.Unlock()
				matches, fileType := si.containsSearchTerm(fullName, searchTerm, *searchOptions, isDir, fileTypes)
				si.mu.Lock()
				if !matches {
					continue
				}
				fileListTypes[fullName] = fileType
				matching = append(matching, fullName)
				count++
			}
		}
		si.mu.Unlock()
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matching, func(i, j int) bool {
		parts1 := strings.Split(matching[i], "/")
		parts2 := strings.Split(matching[j], "/")
		return len(parts1) < len(parts2)
	})
	return matching, fileListTypes
}

func scopedPathNameFilter(pathName string, scope string, isDir bool) string {
	pathName = strings.TrimLeft(pathName, "/")
	pathName = strings.TrimRight(pathName, "/")
	if strings.HasPrefix(pathName, scope) || scope == "" {
		pathName = strings.TrimPrefix(pathName, scope)
		pathName = strings.TrimLeft(pathName, "/")
		if isDir {
			pathName = pathName + "/"
		}
	} else {
		pathName = "" // return not matched
	}
	return pathName
}

func (si *Index) containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool, fileTypes map[string]bool) (bool, map[string]bool) {
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	fileName := filepath.Base(pathName)
	adjustedPath := si.makeIndexPath(pathName, isDir)

	// Convert to lowercase if not exact match
	if !conditions["exact"] {
		fileName = strings.ToLower(fileName)
		searchTerm = strings.ToLower(searchTerm)
	}

	// Check if the file name contains the search term
	if !strings.Contains(fileName, searchTerm) {
		return false, map[string]bool{}
	}

	// Initialize file size and fileTypes map
	var fileSize int64
	extension := filepath.Ext(fileName)

	// Collect file types
	for _, k := range AllFiletypeOptions {
		if IsMatchingType(extension, k) {
			fileTypes[k] = true
		}
	}
	fileTypes["dir"] = isDir
	// Get file info if needed for size-related conditions
	if largerThan > 0 || smallerThan > 0 {
		fileInfo, exists := si.GetMetadataInfo(adjustedPath)
		if !exists {
			return false, fileTypes
		} else if !isDir {
			// Look for specific file in ReducedItems
			for _, item := range fileInfo.ReducedItems {
				lower := strings.ToLower(item.Name)
				if strings.Contains(lower, searchTerm) {
					if item.Size == 0 {
						return false, fileTypes
					}
					fileSize = item.Size
					break
				}
			}
		} else {
			fileSize = fileInfo.Size
		}
		if fileSize == 0 {
			return false, fileTypes
		}
	}

	// Evaluate all conditions
	for t, v := range conditions {
		if t == "exact" {
			continue
		}
		switch t {
		case "larger":
			if largerThan > 0 {
				if fileSize <= largerThan {
					return false, fileTypes
				}
			}
		case "smaller":
			if smallerThan > 0 {
				if fileSize >= smallerThan {
					return false, fileTypes
				}
			}
		default:
			// Handle other file type conditions
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false, fileTypes
			}
		}
	}

	return true, fileTypes
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
