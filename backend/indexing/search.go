package indexing

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var SearchResultsCache = cache.NewCache[[]string](15 * time.Second)

var (
	sessionInProgress    sync.Map
	DefaultSearchResults = 100
)

type SearchResult struct {
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Modified   string `json:"modified,omitempty"`
	HasPreview bool   `json:"hasPreview"`
	Source     string `json:"source"`
}

func (idx *Index) Search(search string, scope string, sourceSession string, largest bool, limit int) []*SearchResult {
	// Ensure scope has consistent trailing slash for directory matching
	scope = utils.AddTrailingSlashIfNotExists(scope)

	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := iteminfo.ParseSearch(search)
	results := make(map[string]*SearchResult, 0)
	count := 0

	// When largest=true and no search terms, ensure we run at least once
	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	rows, err := idx.db.SearchItems(idx.Name, scope, largest)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()
	for rows.Next() {
		// Check for cancellation
		value, found := sessionInProgress.Load(sourceSession)
		if !found || value != runningHash {
			return []*SearchResult{}
		}

		if limit > 0 && count >= limit {
			break
		}

		var path string
		var name string
		var size int64
		var modTime int64
		var mimeType string
		var isDir bool
		var hasPreview bool

		if err := rows.Scan(&path, &name, &size, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
			logger.Errorf("Failed to scan search result row: %v", err)
			continue
		}

		// Create ItemInfo for matching
		item := iteminfo.ItemInfo{
			Name:       name,
			Size:       size,
			ModTime:    time.Unix(modTime, 0),
			Type:       mimeType,
			HasPreview: hasPreview,
		}

		// Check against all search terms
		for _, searchTerm := range searchOptions.Terms {
			if searchTerm == "" && !largest {
				continue
			}

			var matches bool
			if largest {
				// When largest=true, check size and type conditions, skip name matching
				largerThan := int64(searchOptions.LargerThan) * 1024 * 1024
				sizeMatches := largerThan == 0 || item.Size > largerThan
				// Check if directories should be excluded (when type:file is specified)
				dirCondition, hasDirCondition := searchOptions.Conditions["dir"]
				// For directories: match if dir condition is explicitly true
				// For files: match if no dir condition, or if dir condition is false
				var typeMatches bool
				if isDir {
					typeMatches = hasDirCondition && dirCondition
				} else {
					typeMatches = !hasDirCondition || (hasDirCondition && !dirCondition)
				}
				matches = sizeMatches && typeMatches
			} else {
				matches = item.ContainsSearchTerm(searchTerm, searchOptions)
			}

			if matches {
				// Determine type string for response
				resType := mimeType
				if isDir {
					resType = "directory"
				}

				results[path] = &SearchResult{
					Path:       path,
					Type:       resType,
					Size:       size,
					Modified:   item.ModTime.Format(time.RFC3339),
					HasPreview: hasPreview,
					Source:     idx.Name,
				}
				count++
				// Break inner loop (terms) if matched, move to next row
				break
			}
		}
	}

	// Sort keys based on the number of elements in the path after splitting by "/"
	sortedKeys := make([]*SearchResult, 0, len(results))
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

// SearchMultiSources searches across multiple sources in a single database query.
// sources is a list of source names to search.
// sourceScopes is a map from source name to the scope path for that source.
// sourceSession is used for cancellation tracking.
// largest and limit work the same as in Search.
func SearchMultiSources(search string, sources []string, sourceScopes map[string]string, sourceSession string, largest bool, limit int) []*SearchResult {
	if len(sources) == 0 {
		return []*SearchResult{}
	}

	// Get the shared database
	db := GetIndexDB()
	if db == nil {
		return []*SearchResult{}
	}

	// Ensure all scopes have consistent trailing slash
	normalizedScopes := make(map[string]string)
	for source, scope := range sourceScopes {
		normalizedScopes[source] = utils.AddTrailingSlashIfNotExists(scope)
	}

	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash)
	searchOptions := iteminfo.ParseSearch(search)
	results := make(map[string]*SearchResult, 0)
	count := 0

	// When largest=true and no search terms, ensure we run at least once
	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
		// Check for cancellation
		value, found := sessionInProgress.Load(sourceSession)
		if !found || value != runningHash {
			return []*SearchResult{}
		}

		if limit > 0 && count >= limit {
			break
		}

		var source string
		var path string
		var name string
		var size int64
		var modTime int64
		var mimeType string
		var isDir bool
		var hasPreview bool

		if err := rows.Scan(&source, &path, &name, &size, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
			logger.Errorf("Failed to scan search result row: %v", err)
			continue
		}

		// Create ItemInfo for matching
		item := iteminfo.ItemInfo{
			Name:       name,
			Size:       size,
			ModTime:    time.Unix(modTime, 0),
			Type:       mimeType,
			HasPreview: hasPreview,
		}

		// Check against all search terms
		for _, searchTerm := range searchOptions.Terms {
			if searchTerm == "" && !largest {
				continue
			}

			var matches bool
			if largest {
				// When largest=true, check size and type conditions, skip name matching
				largerThan := int64(searchOptions.LargerThan) * 1024 * 1024
				sizeMatches := largerThan == 0 || item.Size > largerThan
				// Check if directories should be excluded (when type:file is specified)
				dirCondition, hasDirCondition := searchOptions.Conditions["dir"]
				// For directories: match if dir condition is explicitly true
				// For files: match if no dir condition, or if dir condition is false
				var typeMatches bool
				if isDir {
					typeMatches = hasDirCondition && dirCondition
				} else {
					typeMatches = !hasDirCondition || (hasDirCondition && !dirCondition)
				}
				matches = sizeMatches && typeMatches
			} else {
				matches = item.ContainsSearchTerm(searchTerm, searchOptions)
			}

			if matches {
				// Determine type string for response
				resType := mimeType
				if isDir {
					resType = "directory"
				}

				// Use source+path as key to handle same path in different sources
				key := source + ":" + path
				results[key] = &SearchResult{
					Path:       path,
					Type:       resType,
					Size:       size,
					Modified:   item.ModTime.Format(time.RFC3339),
					HasPreview: hasPreview,
					Source:     source,
				}
				count++
				// Break inner loop (terms) if matched, move to next row
				break
			}
		}
	}

	// Sort keys based on the number of elements in the path after splitting by "/"
	sortedKeys := make([]*SearchResult, 0, len(results))
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
