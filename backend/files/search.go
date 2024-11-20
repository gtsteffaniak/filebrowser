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
	scope = si.makeIndexPath(scope)
	runningHash := utils.GenerateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	results := make(map[string]searchResult, 0)
	count := 0
	directories := si.getDirsInScope(scope)
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		if count > maxSearchResults {
			break
		}
		si.mu.Lock()
		for _, dirName := range directories {
			dir, found := si.GetReducedMetadata(dirName, true)
			if !found {
				continue
			}
			if count > maxSearchResults {
				break
			}
			reducedDir := ReducedItem{
				Name: filepath.Base(dirName),
				Type: "directory",
				Size: dir.Size,
			}

			matches := reducedDir.containsSearchTerm(searchTerm, searchOptions)
			if matches {
				scopedPath := strings.TrimPrefix(strings.TrimPrefix(dirName, scope), "/") + "/"
				results[scopedPath] = searchResult{Path: scopedPath, Type: "directory", Size: dir.Size}
				count++
			}

			// search files first
			for _, item := range dir.Items {

				fullPath := dirName + "/" + item.Name
				if item.Type == "directory" {
					fullPath += "/"
				}
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					si.mu.Unlock()
					return []searchResult{}
				}
				if count > maxSearchResults {
					break
				}
				matches := item.containsSearchTerm(searchTerm, searchOptions)
				if matches {
					scopedPath := strings.TrimPrefix(strings.TrimPrefix(fullPath, scope), "/")
					results[scopedPath] = searchResult{Path: scopedPath, Type: item.Type, Size: item.Size}
					count++
				}
			}
		}
		si.mu.Unlock()
	}

	// Sort keys based on the number of elements in the path after splitting by "/"
	sortedKeys := make([]searchResult, 0, len(results))
	for _, v := range results {
		sortedKeys = append(sortedKeys, v)
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(sortedKeys, func(i, j int) bool {
		parts1 := strings.Split(sortedKeys[i].Path, "/")
		parts2 := strings.Split(sortedKeys[j].Path, "/")
		return len(parts1) < len(parts2)
	})
	return sortedKeys
}

// returns true if the file name contains the search term
// returns file type if the file name contains the search term
// returns size of file/dir if the file name contains the search term
func (fi ReducedItem) containsSearchTerm(searchTerm string, options *SearchOptions) bool {

	fileTypes := map[string]bool{}
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	lowerFileName := strings.ToLower(fi.Name)

	// Convert to lowercase if not exact match
	if !conditions["exact"] {
		searchTerm = strings.ToLower(searchTerm)
	}

	// Check if the file name contains the search term
	if !strings.Contains(lowerFileName, searchTerm) {
		return false
	}

	// Initialize file size and fileTypes map
	var fileSize int64
	extension := filepath.Ext(lowerFileName)

	// Collect file types
	for _, k := range AllFiletypeOptions {
		if IsMatchingType(extension, k) {
			fileTypes[k] = true
		}
	}
	isDir := fi.Type == "directory"
	fileTypes["dir"] = isDir
	fileSize = fi.Size

	// Evaluate all conditions
	for t, v := range conditions {
		if t == "exact" {
			continue
		}
		switch t {
		case "larger":
			if largerThan > 0 {
				if fileSize <= largerThan {
					return false
				}
			}
		case "smaller":
			if smallerThan > 0 {
				if fileSize >= smallerThan {
					return false
				}
			}
		default:
			// Handle other file type conditions
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false
			}
		}
	}

	return true
}

func (si *Index) getDirsInScope(scope string) []string {
	newList := []string{}
	si.mu.RLock()
	defer si.mu.RUnlock()
	for k := range si.Directories {
		if strings.HasPrefix(k, scope) || scope == "" {
			newList = append(newList, k)
		}
	}
	return newList
}
