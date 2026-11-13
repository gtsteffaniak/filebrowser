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
	// Content hits still obey type/size/date filters; folders-only must not return files.
	var content *contentSearch
	if searchContent {
		content = newContentSearch(idx.db, idx.Name, scope, searchOptions, largest, useWildcard, limit)
		if content != nil {
			content.nextPage()
		}
	}

	results := make(map[string]*SearchResult)
	count := 0
	nameGlobPatterns := iteminfo.NameGlobPatternsForSearch(searchOptions, useWildcard, largest)
	caseExact := searchOptions.Conditions["exact"]
	globAnd := len(nameGlobPatterns) > 0 && searchOptions.MatchAllTerms

	// Largest mode is ordered by size in Go, so it cannot resume from a path cursor.
	cursorPath := searchOptions.CursorPath
	if largest {
		cursorPath = ""
	}
	rows, err := idx.db.SearchItems(idx.Name, scope, largest, nameGlobPatterns, globAnd, caseExact, cursorPath)
	if err != nil && !content.hasPending() {
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
		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) ||
			content.claim(path)
		if !matches {
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

	// Content-only hits are appended, in depth order, until the page is full. They are
	// still returned when the name query failed.
	if limit <= 0 || count < limit {
		appendContentResults([]*contentSearch{content}, limit, results, &count)
	}
	return sortSearchResults(results)
}

// contentSearch resolves paths whose content matches the terms when full-text search
// applies; nil otherwise (wildcard and term-less searches never use content).
//
// The FTS query itself is limited, so candidates are read page by page and filtered
// (auxiliary filters, folders still indexed) before anything is kept: filtered-out
// hits never consume the caller's limit.
type contentSearch struct {
	db        *dbsql.IndexDB
	source    string
	opts      iteminfo.SearchOptions
	scope     string
	pageSize  int
	set       map[string]bool // pending hits, for O(1) membership
	order     []string        // pending hits in match order
	last      string          // (length, path) cursor of the last page read
	exhausted bool
	pagesRead int
	multi     bool // results of these searches are keyed source:path
}

// contentLookupPages bounds how many FTS pages are read for one search.
const contentLookupPages = 5

// contentFillPage bounds content-only results appended when the caller passes no limit.
const contentFillPage = 1000

// newContentSearch returns nil when content search does not apply to this search.
func newContentSearch(db *dbsql.IndexDB, source, scope string, opts iteminfo.SearchOptions, largest, useWildcard bool, limit int) *contentSearch {
	if largest || useWildcard || len(opts.Terms) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = contentFillPage
	}
	return &contentSearch{db: db, source: source, opts: opts, scope: scope, pageSize: limit, set: make(map[string]bool)}
}

// nextPage reads another FTS page and queues the candidates that pass the filters.
// It reports whether anything was queued.
func (cs *contentSearch) nextPage() bool {
	for !cs.exhausted && cs.pagesRead < contentLookupPages {
		paths, err := cs.db.SearchFullText(cs.source, cs.scope, cs.opts.Terms, cs.opts.MatchAllTerms, cs.pageSize, cs.last)
		if err != nil {
			logger.Errorf("full-text search failed for source %s: %v", cs.source, err)
			cs.exhausted = true
			return false
		}
		cs.pagesRead++
		kept := filterContentPaths(cs.db, cs.source, cs.scope, paths, cs.opts)
		for _, p := range kept {
			cs.set[p] = true
			cs.order = append(cs.order, p)
		}
		if len(paths) < cs.pageSize {
			cs.exhausted = true
			return len(kept) > 0
		}
		cs.last = paths[len(paths)-1]
		if len(kept) > 0 {
			return true
		}
	}
	return false
}

// claim reports whether path is a pending content hit and stops it from being returned again.
func (cs *contentSearch) claim(path string) bool {
	if cs == nil || !cs.set[path] {
		return false
	}
	delete(cs.set, path)
	for i, p := range cs.order {
		if p == path {
			cs.order = append(cs.order[:i], cs.order[i+1:]...)
			break
		}
	}
	return true
}

// hasPending reports whether content hits may still remain.
func (cs *contentSearch) hasPending() bool {
	return cs != nil && (len(cs.order) > 0 || !cs.exhausted)
}

// filterContentPaths keeps content hits whose item row exists, is a file, passes the
// type/size/date filters, and still has a parent folder that is indexed.
func filterContentPaths(db *dbsql.IndexDB, source, scope string, paths []string, opts iteminfo.SearchOptions) []string {
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
			info, ok := items[p]
			if !ok || info.IsDir || info.Type == "directory" || !info.MatchesSearchAuxiliaryFilters(opts) {
				continue
			}
			kept = append(kept, p)
		}
	}
	return dropMissingParents(db, source, scope, kept)
}

// dropMissingParents keeps hits whose parent folder is still indexed, so content from a
// directory removed from the index is not returned. The scope root is always in scope.
func dropMissingParents(db *dbsql.IndexDB, source, scope string, paths []string) []string {
	scopeKey := strings.TrimSuffix(scope, "/")
	if scopeKey == "" {
		scopeKey = "/"
	}
	parents := make(map[string]bool)
	var check []string
	for _, p := range paths {
		parent := parentIndexPath(p)
		if _, seen := parents[parent]; !seen && strings.TrimSuffix(parent, "/") != scopeKey {
			parents[parent] = false
			check = append(check, parent)
		}
	}
	if len(check) > 0 {
		if items, err := db.GetItemsByPaths(source, check); err == nil {
			for parent := range items {
				parents[parent] = true
			}
		}
	}
	kept := paths[:0]
	for _, p := range paths {
		// A top-level file ("/a.txt") has no parent folder row.
		if parent := parentIndexPath(p); parent == "" || strings.TrimSuffix(parent, "/") == scopeKey || parents[parent] {
			kept = append(kept, p)
		}
	}
	return kept
}

// parentIndexPath returns p's parent index path (with trailing slash), or "" at the root.
func parentIndexPath(p string) string {
	i := strings.LastIndex(strings.TrimSuffix(p, "/"), "/")
	if i <= 0 {
		return ""
	}
	return p[:i+1]
}

// appendContentResults adds content-only hits in depth order until the limit is reached.
// Several searches (multi-source) are merged by path depth.
func appendContentResults(searches []*contentSearch, limit int, results map[string]*SearchResult, count *int) {
	var pending []*SearchResult
	for _, cs := range searches {
		for {
			path, ok := cs.pop(limit - len(pending))
			if !ok {
				break
			}
			// Single-source results are keyed by path; multi-source by source:path.
			key := path
			if _, ok := results[key]; !ok {
				key = cs.source + ":" + path
				if _, ok := results[key]; !ok {
					key = path
				}
			}
			if _, ok := results[key]; ok {
				continue
			}
			pending = append(pending, contentResult(cs, path))
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return strings.Count(pending[i].Path, "/") < strings.Count(pending[j].Path, "/")
	})
	for _, res := range pending {
		if limit > 0 && *count >= limit {
			return
		}
		key := res.Path
		if _, ok := results[key]; !ok && searches[0].multi {
			key = res.Source + ":" + res.Path
		}
		results[key] = res
		*count++
	}
}

// pop returns the next pending content hit, loading more pages while room remains.
func (cs *contentSearch) pop(room int) (string, bool) {
	if cs == nil {
		return "", false
	}
	for len(cs.order) == 0 {
		if room <= 0 || !cs.nextPage() {
			return "", false
		}
	}
	path := cs.order[0]
	cs.order = cs.order[1:]
	delete(cs.set, path)
	return path, true
}

// contentResult builds a result for a content-only hit from its index row.
func contentResult(cs *contentSearch, path string) *SearchResult {
	res := &SearchResult{Path: path, Source: cs.source}
	if items, err := cs.db.GetItemsByPaths(cs.source, []string{path}); err == nil {
		if info := items[path]; info != nil {
			res.Type = info.Type
			res.Size = info.Size
			res.Modified = info.ModTime.Format(time.RFC3339)
			res.HasPreview = info.HasPreview
			if info.Created != nil && !info.Created.IsZero() {
				res.Created = info.Created.Format(time.RFC3339)
			}
		}
	}
	return res
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

	// Content hits are resolved per source with the same guards as single-source.
	var content []*contentSearch
	if searchContent {
		for _, source := range sources {
			cs := newContentSearch(db, source, normalizedScopes[source], searchOptions, largest, useWildcard, limit)
			if cs != nil {
				cs.multi = true
				cs.nextPage()
				content = append(content, cs)
			}
		}
	}
	hasContent := false
	for _, cs := range content {
		hasContent = hasContent || cs.hasPending()
	}

	// Largest mode is ordered by size in Go, so it cannot resume from a path cursor.
	cursorPath := searchOptions.CursorPath
	if largest {
		cursorPath = ""
	}
	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest, nameGlobPatterns, globAnd, caseExact, cursorPath)
	if err != nil && !hasContent {
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
			claimFrom(content, source, path)

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

	// Content-only hits are appended, in depth order, until the page is full.
	if limit <= 0 || count < limit {
		appendContentResults(content, limit, results, &count)
	}

	return sortSearchResults(results)
}

// claimFrom marks path as returned by the content search of source, if any.
func claimFrom(searches []*contentSearch, source, path string) bool {
	for _, cs := range searches {
		if cs.source == source {
			return cs.claim(path)
		}
	}
	return false
}
