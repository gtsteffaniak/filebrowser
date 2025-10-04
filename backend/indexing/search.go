package indexing

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
)

var SearchResultsCache = cache.NewCache[[]string](15 * time.Second)

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
	// Ensure scope has consistent trailing slash for directory matching
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope = scope + "/"
	}
	if search == "" {
		scope = ""
	}
	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := iteminfo.ParseSearch(search)
	results := make(map[string]SearchResult, 0)
	count := 0
	var directories []string
	cachedDirs, ok := SearchResultsCache.Get(idx.Path + scope)
	if ok {
		directories = cachedDirs
	} else {
		directories = idx.getDirsInScope(scope)
		SearchResultsCache.Set(idx.Path+scope, directories)
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
			dir, found := idx.Directories[dirName]
			if !found {
				continue
			}
			if count > maxSearchResults {
				break
			}
			reducedDir := iteminfo.ItemInfo{
				Name: filepath.Base(dirName),
				Type: "directory",
				Size: dir.Size,
			}
			matches := reducedDir.ContainsSearchTerm(searchTerm, searchOptions)
			if matches {
				results[dirName] = SearchResult{Path: dirName, Type: "directory", Size: dir.Size}
				count++
			}
			// search files first
			for _, item := range dir.Files {
				fullPath := filepath.Join(dirName, item.Name)
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					idx.mu.Unlock()
					return []SearchResult{}
				}
				if count > maxSearchResults {
					break
				}
				matches := item.ContainsSearchTerm(searchTerm, searchOptions)
				if matches {
					results[fullPath] = SearchResult{Path: fullPath, Type: item.Type, Size: item.Size}
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

func (idx *Index) getDirsInScope(scope string) []string {
	newList := []string{}
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// If scope is empty, return all directories
	if scope == "" {
		for k := range idx.Directories {
			newList = append(newList, k)
		}
		return newList
	}

	// For non-empty scope, include the scope directory itself and all subdirectories
	for k := range idx.Directories {
		// Match the scope directory exactly (k == scope)
		// OR match subdirectories (k starts with scope)
		if k == scope || strings.HasPrefix(k, scope) {
			newList = append(newList, k)
		}
	}
	return newList
}
