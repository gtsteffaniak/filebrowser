package indexing

import (
	"sort"
	"strings"
	"sync"
	"time"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
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
	Created    string `json:"created,omitempty"`
	HasPreview bool   `json:"hasPreview"`
	Source     string `json:"source"`
}

// Search parses query and delegates to SearchParsed (legacy / tests).
func (idx *Index) Search(search string, scope string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	return idx.SearchParsed(iteminfo.ParseSearch(search), scope, sourceSession, largest, limit, olderThanUnix, newerThanUnix, useWildcard, false)
}

// SearchParsed runs indexing search using pre-built options (e.g. repeated HTTP term parameters plus filter prefix).
// searchContent additionally matches terms against indexed file contents (requires full-text search enabled).
func (idx *Index) SearchParsed(baseOpts iteminfo.SearchOptions, scope string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool, searchContent bool) []*SearchResult {
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
	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	// Content hits are resolved to paths first; they participate in the name scan
	// below so ordering and limits behave exactly like name matches.
	var contentPaths []string
	if searchContent {
		var err error
		contentPaths, err = contentSearchPaths(idx.db, idx.Name, scope, searchOptions, largest, useWildcard, limit)
		if err != nil {
			logger.Errorf("full-text search failed for source %s: %v", idx.Name, err)
			contentPaths = nil
		}
	}
	contentSet := make(map[string]bool, len(contentPaths))
	for _, p := range contentPaths {
		contentSet[p] = true
	}

	results := make(map[string]*SearchResult)
	count := 0
	nameGlobPatterns := iteminfo.NameGlobPatternsForSearch(searchOptions, useWildcard, largest)
	caseExact := searchOptions.Conditions["exact"]
	globAnd := len(nameGlobPatterns) > 0 && searchOptions.MatchAllTerms

	rows, err := idx.db.SearchItems(idx.Name, scope, largest, nameGlobPatterns, globAnd, caseExact)
	if err != nil && len(contentPaths) == 0 {
		return []*SearchResult{}
	}
	if err == nil {
		defer rows.Close()
	}

	for err == nil && rows.Next() {
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
		var createdTime int64
		var modTime int64
		var mimeType string
		var isDir bool
		var hasPreview bool
		if err := rows.Scan(&path, &name, &size, &createdTime, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
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
		// Content-matched paths match by definition; the name query returns every row of
		// the scope, so they must not be filtered by name here.
		if !itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) && !contentSet[path] {
			continue
		}

		resType := mimeType
		if isDir {
			resType = "directory"
		}
		res := &SearchResult{
			Path:       path,
			Type:       resType,
			Size:       size,
			Modified:   item.ModTime.Format(time.RFC3339),
			HasPreview: hasPreview,
			Source:     idx.Name,
		}
		if createdTime > 0 {
			res.Created = time.Unix(createdTime, 0).Format(time.RFC3339)
		}
		results[path] = res
		count++
	}

	if err == nil && searchContent {
		appendContentResults(idx.db, idx.Name, contentPaths, limit, results, &count)
	}
	return sortSearchResults(results)
}

// contentSearchPaths returns index paths whose content matches every search term when
// full-text search is enabled; otherwise nil. Name-only matches are left to the name query.
func contentSearchPaths(db *dbsql.IndexDB, source, scope string, opts iteminfo.SearchOptions, largest, useWildcard bool, limit int) ([]string, error) {
	if largest || useWildcard || len(opts.Terms) == 0 {
		return nil, nil
	}
	return db.SearchFullText(source, scope, opts.Terms, limit)
}

// appendContentResults loads index rows for content-matched paths and adds the files
// that were not already matched by name (call after the name-based loop).
func appendContentResults(db *dbsql.IndexDB, source string, paths []string, limit int, results map[string]*SearchResult, count *int) {
	if len(paths) == 0 {
		return
	}
	items, err := db.GetItemsByPaths(source, paths)
	if err != nil {
		logger.Errorf("full-text search: load items for source %s: %v", source, err)
		return
	}
	for _, path := range paths {
		if limit > 0 && *count >= limit {
			return
		}
		info := items[path]
		if info == nil || info.IsDir || info.Type == "directory" {
			continue
		}
		// Single-source results are keyed by path; multi-source by source:path.
		key := path
		if _, ok := results[key]; !ok {
			key = source + ":" + path // multi-source callers key by source:path
		}
		if _, ok := results[key]; ok {
			continue
		}
		res := &SearchResult{
			Path:       path,
			Type:       info.Type,
			Size:       info.Size,
			Modified:   info.ModTime.Format(time.RFC3339),
			HasPreview: info.HasPreview,
			Source:     source,
		}
		if info.Created != nil && !info.Created.IsZero() {
			res.Created = info.Created.Format(time.RFC3339)
		}
		results[key] = res
		*count++
	}
}

func sortSearchResults(results map[string]*SearchResult) []*SearchResult {
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

func itemMatchesSearchFilters(item iteminfo.ItemInfo, isDir bool, searchOptions iteminfo.SearchOptions, largest bool, nameGlobPatterns []string) bool {
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
	if len(nameGlobPatterns) > 0 {
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
	return SearchMultiSourcesParsed(iteminfo.ParseSearch(search), sources, sourceScopes, sourceSession, largest, limit, olderThanUnix, newerThanUnix, useWildcard, false)
}

// SearchMultiSourcesParsed searches multiple sources using pre-built options.
func SearchMultiSourcesParsed(baseOpts iteminfo.SearchOptions, sources []string, sourceScopes map[string]string, sourceSession string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool, searchContent bool) []*SearchResult {
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

	nameGlobPatterns := iteminfo.NameGlobPatternsForSearch(searchOptions, useWildcard, largest)
	caseExact := searchOptions.Conditions["exact"]
	globAnd := len(nameGlobPatterns) > 0 && searchOptions.MatchAllTerms

	// Content hits are resolved per source first; their index rows are fetched by path below.
	contentPathsBySource := make(map[string]map[string]bool)
	if searchContent {
		for _, source := range sources {
			paths, err := db.SearchFullText(source, normalizedScopes[source], searchOptions.Terms, limit)
			if err != nil {
				logger.Errorf("full-text search failed for source %s: %v", source, err)
				continue
			}
			set := make(map[string]bool, len(paths))
			for _, p := range paths {
				set[p] = true
			}
			contentPathsBySource[source] = set
		}
	}

	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest, nameGlobPatterns, globAnd, caseExact)
	if err != nil && len(contentPathsBySource) == 0 {
		return []*SearchResult{}
	}
	if err == nil {
		defer rows.Close()
	}

	for err == nil && rows.Next() {
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
		var createdTime int64
		var modTime int64
		var mimeType string
		var isDir bool
		var hasPreview bool

		if err := rows.Scan(&source, &path, &name, &size, &createdTime, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
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

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) || contentPathsBySource[source][path]

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
			if createdTime > 0 {
				results[key].Created = time.Unix(createdTime, 0).Format(time.RFC3339)
			}
			count++
		}
	}

	if err == nil && searchContent {
		for _, source := range sources {
			paths := make([]string, 0, len(contentPathsBySource[source]))
			for p := range contentPathsBySource[source] {
				paths = append(paths, p)
			}
			appendContentResults(db, source, paths, limit, results, &count)
		}
	}

	return sortSearchResults(results)
}
