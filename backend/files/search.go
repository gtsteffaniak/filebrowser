package files

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/cache"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

var (
	sessionInProgress sync.Map
	maxSearchResults  = 100
)

type SearchResult struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func (idx *Index) Search(search string, scope string, sourceSession string) []SearchResult {
	// Remove slashes
	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	results := make(map[string]SearchResult, 0)
	count := 0
	var directories []string
	cachedDirs, ok := cache.SearchResults.Get(idx.Source.Path + scope).([]string)
	if ok {
		directories = cachedDirs
	} else {
		directories = idx.getDirsInScope(scope)
		cache.SearchResults.Set(idx.Source.Path+scope, directories)
	}
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		if count > maxSearchResults {
			break
		}
		idx.mu.Lock()
		for _, dirName := range directories {
			scopedPath := strings.TrimPrefix(strings.TrimPrefix(dirName, scope), "/") + "/"
			idx.mu.Unlock()
			dir, found := idx.GetReducedMetadata(dirName, true)
			idx.mu.Lock()
			if !found {
				continue
			}
			if count > maxSearchResults {
				break
			}
			reducedDir := ItemInfo{
				Name: filepath.Base(dirName),
				Type: "directory",
				Size: dir.Size,
			}
			matches := reducedDir.containsSearchTerm(searchTerm, searchOptions)
			if matches {
				results[scopedPath] = SearchResult{Path: scopedPath, Type: "directory", Size: dir.Size}
				count++
			}
			// search files first
			for _, item := range dir.Files {
				fullPath := dirName + "/" + item.Name
				scopedPath := strings.TrimPrefix(strings.TrimPrefix(fullPath, scope), "/")
				if item.Type == "directory" {
					scopedPath += "/"
				}
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					idx.mu.Unlock()
					return []SearchResult{}
				}
				if count > maxSearchResults {
					break
				}
				matches := item.containsSearchTerm(searchTerm, searchOptions)
				if matches {
					results[scopedPath] = SearchResult{Path: scopedPath, Type: item.Type, Size: item.Size}
					count++
				}
			}
		}
		idx.mu.Unlock()
	}

	// Sort keys based on the number of elements in the path after splitting by "/"
	sortedKeys := make([]SearchResult, 0, len(results))
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
func (fi ItemInfo) containsSearchTerm(searchTerm string, options SearchOptions) bool {

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

func (idx *Index) getDirsInScope(scope string) []string {
	newList := []string{}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for k := range idx.Directories {
		if strings.HasPrefix(k, scope) || scope == "" {
			newList = append(newList, k)
		}
	}
	return newList
}
