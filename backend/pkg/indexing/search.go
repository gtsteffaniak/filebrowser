package indexing

import (
	"slices"
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
	// Content hits still obey type/size/date filters; folders-only must not return files.
	if searchContent {
		contentPaths = filterContentPaths(idx.db, idx.Name, contentPaths, searchOptions)
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

	rows, err := idx.db.SearchItems(idx.Name, scope, largest, nameGlobPatterns, globAnd, caseExact, searchOptions.ExcludeThroughPath)
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
		// The name query returns every row of the scope; a content-matched path is a
		// match even when its name does not contain the term.
		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns)
		if !matches && !contentSet[path] {
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

	// Content hits are appended even if the name query failed.
	if searchContent {
		appendContentResults(idx.db, idx.Name, contentPaths, limit, searchOptions, results, &count)
	}
	return sortSearchResults(results)
}

// contentSearchPaths returns index paths whose content matches the terms when
// full-text search is requested; otherwise nil. Name-only matches are left to the name query.
func contentSearchPaths(db *dbsql.IndexDB, source, scope string, opts iteminfo.SearchOptions, largest, useWildcard bool, limit int) ([]string, error) {
	if largest || useWildcard || len(opts.Terms) == 0 {
		return nil, nil
	}
	return db.SearchFullText(source, scope, opts.Terms, opts.MatchAllTerms, limit, opts.ExcludeThroughPath)
}

// filterContentPaths drops content hits whose item fails the type/size/date filters.
// It loads rows in chunks so a large match set does not need one huge IN clause.
func filterContentPaths(db *dbsql.IndexDB, source string, paths []string, opts iteminfo.SearchOptions) []string {
	const chunk = 500
	kept := make([]string, 0, len(paths))
	for start := 0; start < len(paths); start += chunk {
		end := min(start+chunk, len(paths))
		items, err := db.GetItemsByPaths(source, paths[start:end])
		if err != nil {
			logger.Errorf("full-text search: load items for source %s: %v", source, err)
			return nil
		}
		for _, p := range paths[start:end] {
			if info := items[p]; info != nil && !info.IsDir && info.Type != "directory" &&
				info.MatchesSearchAuxiliaryFilters(opts) {
				kept = append(kept, p)
			}
		}
	}
	return kept
}

func hasContentPaths(bySource map[string][]string) bool {
	for _, paths := range bySource {
		if len(paths) > 0 {
			return true
		}
	}
	return false
}

// appendContentResults loads index rows for content-matched paths and adds the files
// that were not already matched by name (call after the name-based loop).
func appendContentResults(db *dbsql.IndexDB, source string, paths []string, limit int, opts iteminfo.SearchOptions, results map[string]*SearchResult, count *int) {
	const chunk = 500
	for start := 0; start < len(paths); start += chunk {
		end := min(start+chunk, len(paths))
		items, err := db.GetItemsByPaths(source, paths[start:end])
		if err != nil {
			logger.Errorf("full-text search: load items for source %s: %v", source, err)
			return
		}
		appendContentChunk(paths[start:end], items, source, limit, opts, results, count)
		if limit > 0 && *count >= limit {
			return
		}
	}
}

func appendContentChunk(paths []string, items map[string]*iteminfo.FileInfo, source string, limit int, opts iteminfo.SearchOptions, results map[string]*SearchResult, count *int) {
	for _, path := range paths {
		if limit > 0 && *count >= limit {
			return
		}
		info := items[path]
		if info == nil || info.IsDir || info.Type == "directory" || !info.MatchesSearchAuxiliaryFilters(opts) {
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

	// Content hits are resolved per source first (same guards as single-source);
	// their index rows are fetched by path below.
	contentPathsBySource := make(map[string][]string)
	if searchContent {
		for _, source := range sources {
			paths, err := contentSearchPaths(db, source, normalizedScopes[source], searchOptions, largest, useWildcard, limit)
			if err != nil {
				logger.Errorf("full-text search failed for source %s: %v", source, err)
				continue
			}
			contentPathsBySource[source] = filterContentPaths(db, source, paths, searchOptions)
		}
	}

	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest, nameGlobPatterns, globAnd, caseExact, searchOptions.ExcludeThroughPath)
	if err != nil && !hasContentPaths(contentPathsBySource) {
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

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) ||
			slices.Contains(contentPathsBySource[source], path)

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

	// Content hits are appended even if the name query failed.
	if searchContent {
		for _, source := range sources {
			appendContentResults(db, source, contentPathsBySource[source], limit, searchOptions, results, &count)
		}
	}

	return sortSearchResults(results)
}
