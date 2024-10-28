package files

import (
	"fmt"
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

type searchResult struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func (si *Index) Search(search string, scope string, sourceSession string) []searchResult {
	// Remove slashes
	scope = strings.TrimLeft(scope, "/")
	scope = strings.TrimRight(scope, "/")
	runningHash := generateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	results := make([]searchResult, 0)
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
				return []searchResult{}
			}
			if count > maxSearchResults {
				break
			}
			hasScope := scopedPathNameFilter(dirName, scope, isDir)
			if !hasScope {
				continue // path not matched
			}
			fmt.Println("this is pathName", dirName, scope, isDir)
			fileTypes := map[string]bool{}
			si.mu.Unlock()
			matches, fileType, fileSize := si.containsSearchTerm(dirName, searchTerm, *searchOptions, isDir, fileTypes)
			si.mu.Lock()
			if matches {
				results = append(results, searchResult{Path: dirName, Type: fileType, Size: fileSize})
				count++
			}
			isDir = false
			for _, file := range files {
				if file == "" {
					continue
				}
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					return []searchResult{}
				}

				if count > maxSearchResults {
					break
				}
				fullName := dirName + "/" + file
				fileTypes := map[string]bool{}
				si.mu.Unlock()
				matches, fileType, fileSize := si.containsSearchTerm(fullName, searchTerm, *searchOptions, isDir, fileTypes)
				si.mu.Lock()
				if matches {
					results = append(results, searchResult{Path: fullName, Type: fileType, Size: fileSize})
					count++
				}
			}
		}
		si.mu.Unlock()
	}

	// Sort by the number of elements in Path after splitting by "/"
	sort.Slice(results, func(i, j int) bool {
		parts1 := strings.Split(results[i].Path, "/")
		parts2 := strings.Split(results[j].Path, "/")
		return len(parts1) < len(parts2)
	})

	return results
}

func scopedPathNameFilter(pathName string, scope string, isDir bool) bool {
	pathName = strings.TrimLeft(pathName, "/")
	pathName = strings.TrimRight(pathName, "/")
	if strings.HasPrefix(pathName, scope) {
		return true // has scope
	}
	return false // does not skip
}

// returns true if the file name contains the search term
// returns file type if the file name contains the search term
// returns size of file/dir if the file name contains the search term
func (si *Index) containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool, fileTypes map[string]bool) (bool, string, int64) {
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	fileName := filepath.Base(pathName)
	lowerFileName := strings.ToLower(fileName)

	adjustedPath := si.makeIndexPath(pathName, isDir)

	// Convert to lowercase if not exact match
	if !conditions["exact"] {
		searchTerm = strings.ToLower(searchTerm)
	}

	// Check if the file name contains the search term
	if !strings.Contains(lowerFileName, searchTerm) {
		return false, "", 0
	}

	// Initialize file size and fileTypes map
	var fileSize int64
	extension := filepath.Ext(lowerFileName)

	fileType := "directory"
	// Collect file types
	for _, k := range AllFiletypeOptions {
		if IsMatchingType(extension, k) {
			fileTypes[k] = true
			fileType = k
		}
	}

	fileTypes["dir"] = isDir

	// Evaluate all conditions
	for t, v := range conditions {
		if t == "exact" {
			continue
		}
		switch t {
		case "larger":
			if largerThan > 0 {
				if fileSize <= largerThan {
					return false, "", 0
				}
			}
		case "smaller":
			if smallerThan > 0 {
				if fileSize >= smallerThan {
					return false, "", 0
				}
			}
		default:
			// Handle other file type conditions
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false, "", 0
			}
		}
	}

	if !isDir {
		// correct for root path issue
		adjustedPath = filepath.Dir(adjustedPath)
		if adjustedPath == "." {
			adjustedPath = "/"
		}
	}
	fmt.Println(pathName, adjustedPath, isDir)

	fileInfo, exists := si.GetMetadataInfo(adjustedPath)
	// Get file info if needed for size-related conditions
	if !exists {
		fmt.Println("not exists", adjustedPath, pathName)
		return false, "", 0
	}
	fmt.Println("looking... ", fileInfo.Name, fileInfo.Size, adjustedPath)

	if !isDir {
		// Look for specific file in ReducedItems
		for _, item := range fileInfo.ReducedItems {
			fmt.Printf("these are files: \"%v\" \"%v\" %v \n", item.Name, fileName, item.Size)
			if item.Name == fileName {
				fileSize = item.Size
				break
			}
		}
	} else {
		fileSize = fileInfo.Size
	}

	return true, fileType, fileSize
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
