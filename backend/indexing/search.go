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

// Search parses query and delegates to SearchParsed (legacy / tests).
func (idx *Index) Search(search string, scope string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	return idx.SearchParsed(iteminfo.ParseSearch(search), scope, sourceSession, largest, limit, olderThanUnix, newerThanUnix, useWildcard)
}

// SearchParsed runs indexing search using pre-built options (e.g. repeated HTTP term parameters plus filter prefix).
func (idx *Index) SearchParsed(baseOpts iteminfo.SearchOptions, scope string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	scope = utils.AddTrailingSlashIfNotExists(scope)

	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash)

	searchOptions := baseOpts
	if olderThanUnix > 0 {
		searchOptions.ModifiedOlderThan = olderThanUnix
	}
	if newerThanUnix > 0 {
		searchOptions.ModifiedNewerThan = newerThanUnix
	}

	results := make(map[string]*SearchResult)
	count := 0

	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	nameGlobPatterns := nameGlobPatternsForSearch(searchOptions, useWildcard, largest)
	globAnd := useWildcard && searchOptions.MatchAllTerms

	rows, err := idx.db.SearchItems(idx.Name, scope, largest, nameGlobPatterns, globAnd)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
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

		item := iteminfo.ItemInfo{
			Name:       name,
			Size:       size,
			ModTime:    time.Unix(modTime, 0),
			Type:       mimeType,
			HasPreview: hasPreview,
		}

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, useWildcard, nameGlobPatterns)

		if matches {
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
		}
	}

	sortedKeys := make([]*SearchResult, 0, len(results))
	for _, v := range results {
		sortedKeys = append(sortedKeys, v)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		parts1 := strings.Split(sortedKeys[i].Path, "/")
		parts2 := strings.Split(sortedKeys[j].Path, "/")
		return len(parts1) < len(parts2)
	})
	return sortedKeys
}

func itemMatchesSearchFilters(item iteminfo.ItemInfo, isDir bool, searchOptions iteminfo.SearchOptions, largest, useWildcard bool, nameGlobPatterns []string) bool {
	if largest {
		largerThan := int64(searchOptions.LargerThan) * 1024 * 1024
		sizeMatches := largerThan == 0 || item.Size > largerThan
		dirCondition, hasDirCondition := searchOptions.Conditions["dir"]
		var typeMatches bool
		if isDir {
			typeMatches = hasDirCondition && dirCondition
		} else {
			typeMatches = !hasDirCondition || (hasDirCondition && !dirCondition)
		}
		dateMatches := searchDateMatches(item.ModTime.Unix(), searchOptions)
		return sizeMatches && typeMatches && dateMatches
	}
	if useWildcard && len(nameGlobPatterns) > 0 {
		return item.MatchesSearchAuxiliaryFilters(searchOptions)
	}
	if searchOptions.MatchAllTerms {
		sawNonempty := false
		for _, searchTerm := range searchOptions.Terms {
			if searchTerm == "" {
				continue
			}
			sawNonempty = true
			if !item.ContainsSearchTerm(searchTerm, searchOptions) {
				return false
			}
		}
		return sawNonempty
	}
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		if item.ContainsSearchTerm(searchTerm, searchOptions) {
			return true
		}
	}
	return false
}

// nameGlobPatternsForSearch builds non-empty parsed terms for SQLite name GLOB OR clauses.
func nameGlobPatternsForSearch(opts iteminfo.SearchOptions, useWildcard, largest bool) []string {
	if largest || !useWildcard || len(opts.Terms) == 0 {
		return nil
	}
	var patterns []string
	for _, t := range opts.Terms {
		if t != "" {
			patterns = append(patterns, t)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	return patterns
}

func searchDateMatches(modUnix int64, opts iteminfo.SearchOptions) bool {
	if opts.ModifiedNewerThan > 0 && modUnix < opts.ModifiedNewerThan {
		return false
	}
	if opts.ModifiedOlderThan > 0 && modUnix >= opts.ModifiedOlderThan {
		return false
	}
	return true
}

// SearchMultiSources parses query and delegates to SearchMultiSourcesParsed.
func SearchMultiSources(search string, sources []string, sourceScopes map[string]string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	return SearchMultiSourcesParsed(iteminfo.ParseSearch(search), sources, sourceScopes, sourceSession, largest, limit, olderThanUnix, newerThanUnix, useWildcard)
}

// SearchMultiSourcesParsed searches multiple sources using pre-built options.
func SearchMultiSourcesParsed(baseOpts iteminfo.SearchOptions, sources []string, sourceScopes map[string]string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	if len(sources) == 0 {
		return []*SearchResult{}
	}

	db := GetIndexDB()
	if db == nil {
		return []*SearchResult{}
	}

	normalizedScopes := make(map[string]string)
	for source, scope := range sourceScopes {
		normalizedScopes[source] = utils.AddTrailingSlashIfNotExists(scope)
	}

	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash)

	searchOptions := baseOpts
	if olderThanUnix > 0 {
		searchOptions.ModifiedOlderThan = olderThanUnix
	}
	if newerThanUnix > 0 {
		searchOptions.ModifiedNewerThan = newerThanUnix
	}

	results := make(map[string]*SearchResult)
	count := 0

	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	nameGlobPatterns := nameGlobPatternsForSearch(searchOptions, useWildcard, largest)
	globAnd := useWildcard && searchOptions.MatchAllTerms

	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest, nameGlobPatterns, globAnd)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
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

		item := iteminfo.ItemInfo{
			Name:       name,
			Size:       size,
			ModTime:    time.Unix(modTime, 0),
			Type:       mimeType,
			HasPreview: hasPreview,
		}

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, useWildcard, nameGlobPatterns)

		if matches {
			resType := mimeType
			if isDir {
				resType = "directory"
			}

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
		}
	}

	sortedKeys := make([]*SearchResult, 0, len(results))
	for _, v := range results {
		sortedKeys = append(sortedKeys, v)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		parts1 := strings.Split(sortedKeys[i].Path, "/")
		parts2 := strings.Split(sortedKeys[j].Path, "/")
		return len(parts1) < len(parts2)
	})
	return sortedKeys
}
