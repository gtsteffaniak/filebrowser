package files

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/utils"
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
	runningHash := utils.GenerateRandomHash(4)
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
			adjustedDir := strings.TrimPrefix(dirName, "/")
			if dirName == "/" {
				adjustedDir = "/"
			}
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
			fileTypes := map[string]bool{}
			si.mu.Unlock()
			matches, fileType, fileSize := si.containsSearchTerm(dirName, searchTerm, *searchOptions, isDir, fileTypes)
			si.mu.Lock()
			if matches {
				scopedPath := strings.TrimPrefix(strings.TrimPrefix(adjustedDir, scope), "/")
				results = append(results, searchResult{Path: scopedPath, Type: fileType, Size: fileSize})
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
				if dirName == "/" {
					fullName = file
				}
				fileTypes := map[string]bool{}
				si.mu.Unlock()
				matches, fileType, fileSize := si.containsSearchTerm(fullName, searchTerm, *searchOptions, isDir, fileTypes)
				si.mu.Lock()
				if matches {
					scopedPath := strings.TrimPrefix(strings.TrimPrefix(fullName, "/"), scope)
					results = append(results, searchResult{Path: strings.TrimPrefix(scopedPath, "/"), Type: fileType, Size: fileSize})
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
	return strings.HasPrefix(pathName, scope)
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

	if !isDir {
		// correct for root path issue
		adjustedPath = filepath.Dir(adjustedPath)
		if adjustedPath == "." {
			adjustedPath = "/"
		}
	}

	fileInfo, exists := si.GetMetadataInfo(adjustedPath)
	// Get file info if needed for size-related conditions
	if !exists {
		return false, "", 0
	}

	if !isDir {
		// Look for specific file in ReducedItems
		for _, item := range fileInfo.ReducedItems {
			if item.Name == fileName {
				fileSize = item.Size
				break
			}
		}
	} else {
		fileSize = fileInfo.Size
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

	return true, fileType, fileSize
}
