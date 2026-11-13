package indexing

import (
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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

	// Name rows are read to the end of the page before any content page is fetched:
	// db.Query holds the DB mutex for the life of the rows, so interleaving would
	// deadlock. Both streams are then merged by the cursor key so a shallower
	// content-only hit is never displaced by a deeper name hit filling the page.
	// Rows arrive in the same order the result page keeps, so reading past the limit
	// would only build SearchResults that are dropped later.
	var nameRows []*SearchResult
	for err == nil && rows.Next() {
		if limit > 0 && len(nameRows) >= limit {
			break
		}
		value, found := sessionInProgress.Load(sourceSession)
		if !found || value != runningHash {
			return []*SearchResult{}
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
		if !itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) && !content.has(path) {
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
		nameRows = append(nameRows, res)
	}
	if err == nil {
		rows.Close()
	}
	for _, res := range mergeByNameKey(content, nameRows) {
		if limit > 0 && count >= limit {
			break
		}
		if _, dup := results[res.Path]; dup {
			continue
		}
		results[res.Path] = res
		count++
	}

	// Content-only hits are appended, in cursor order, until the page is full. They are
	// still returned when the name query failed.
	if limit <= 0 || count < limit {
		appendContentResults([]*contentSearch{content}, limit, results, &count)
	}
	return sortSearchResults(results)
}

// mergeMulti interleaves multi-source name hits with content hits by (length, path,
// source). Content hits already returned by the name pass are skipped, and the caller
// applies the result limit.
func mergeMulti(searches []*contentSearch, nameRows []*SearchResult) []*SearchResult {
	seen := make(map[string]bool, len(nameRows))
	merged := make([]*SearchResult, 0, len(nameRows))
	for _, row := range nameRows {
		seen[row.Source+":"+row.Path] = true
	}
	i := 0
	room := len(nameRows) + 1
	for {
		cs, path, ok := smallestCandidate(searches, room)
		if !ok {
			break
		}
		if seen[cs.source+":"+path] {
			// Already returned as a name hit; drop it without spending page budget.
			cs.claim(path)
			room++
			continue
		}
		if i >= len(nameRows) {
			cs.claim(path)
			seen[cs.source+":"+path] = true
			merged = append(merged, contentResult(cs, path))
			continue
		}
		next := nameRows[i]
		if sourceKeyLess(path, cs.source, next.Path, next.Source) {
			cs.claim(path)
			seen[cs.source+":"+path] = true
			merged = append(merged, contentResult(cs, path))
			continue
		}
		merged = append(merged, next)
		i++
		room++
	}
	merged = append(merged, nameRows[i:]...)
	return merged
}

// contentHit reports whether path in source is a pending content hit.
func contentHit(searches []*contentSearch, source, path string) bool {
	for _, cs := range searches {
		if cs.source == source {
			return cs.has(path)
		}
	}
	return false
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
	freed     int  // hits claimed by the name pass, available to appendContentResults
	multi     bool // results of these searches are keyed source:path
	at        bool // resume at last (inclusive) instead of strictly after it
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
	// Resume where the caller's cursor left off, or a later page would re-read hits
	// the name query already excluded. A multi-source cursor also names a source: only
	// sources ordered after it still own a hit at that same path, so they start there.
	inclusive := opts.CursorSource != "" && source > opts.CursorSource
	return &contentSearch{
		db: db, source: source, opts: opts, scope: scope, pageSize: limit,
		set:  make(map[string]bool),
		last: opts.CursorPath,
		at:   inclusive,
	}
}

// nextPage reads another FTS page and queues the candidates that pass the filters.
// It reports whether anything was queued.
func (cs *contentSearch) nextPage() bool {
	for !cs.exhausted && cs.pagesRead < contentLookupPages {
		mode := dbsql.StartingAfter
		if cs.at {
			mode = dbsql.StartingAt
		}
		paths, err := cs.db.SearchFullTextFrom(cs.source, cs.scope, cs.opts.Terms, cs.opts.MatchAllTerms, cs.pageSize, cs.last, mode)
		if err != nil {
			logger.Errorf("full-text search failed for source %s: %v", cs.source, err)
			cs.exhausted = true
			return false
		}
		cs.pagesRead++
		// Only the first lookup may include the cursor path itself; later pages resume
		// strictly past the last path they returned.
		cs.at = false
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
// The freed budget lets a later appendContentResults read another page.
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
	cs.freed++
	return true
}

// len reports how many content hits are queued right now.
func (cs *contentSearch) len() int {
	if cs == nil {
		return 0
	}
	return len(cs.order)
}

// has reports whether path is currently a pending content hit.
func (cs *contentSearch) has(path string) bool {
	return cs != nil && cs.set[path]
}

// peek returns the pending content hit that sorts first under the search cursor
// order, loading more pages while budget remains. Hits read from different sources
// are not globally ordered, so the smallest key is chosen explicitly.
func (cs *contentSearch) peek(budget int) (string, bool) {
	if cs == nil {
		return "", false
	}
	// nextPage can return true without queueing anything (its kept hits were already
	// claimed), so bounded by pages rather than by its return value alone.
	for len(cs.order) == 0 && budget > 0 && !cs.exhausted && cs.pagesRead < contentLookupPages {
		cs.nextPage()
	}
	if len(cs.order) == 0 {
		return "", false
	}
	best := 0
	for i := 1; i < len(cs.order); i++ {
		if pathKeyLess(cs.order[i], cs.order[best]) {
			best = i
		}
	}
	return cs.order[best], true
}

// sourceKeyLess orders multi-source results by (length(path), path, source).
func sourceKeyLess(pathA, sourceA, pathB, sourceB string) bool {
	if pathKeyLess(pathA, pathB) {
		return true
	}
	return pathA == pathB && sourceA < sourceB
}

// pathKeyLess orders by the search cursor key: SQLite length(path) (code points), then path.
func pathKeyLess(a, b string) bool {
	if la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b); la != lb {
		return la < lb
	}
	return a < b
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

// appendContentResults adds content-only hits in cursor order until the limit is reached.
// Content hits from several sources are merged here because the name query saw only the
// paths it returned; the remaining hits are ordered by the same (length, path) key.
func appendContentResults(searches []*contentSearch, limit int, results map[string]*SearchResult, count *int) {
	for budget := limit; budget > 0 || limit <= 0; {
		cs, path, ok := smallestCandidate(searches, budget)
		if !ok {
			return
		}
		cs.claim(path)
		key := resultKey(cs, path, results)
		if _, exists := results[key]; exists {
			continue
		}
		// The candidate is already the smallest remaining key, so appending keeps order.
		results[key] = contentResult(cs, path)
		*count++
		budget--
	}
}

// mergeByNameKey interleaves the name hits with content hits already loaded for this
// page, ordered by (length(path), path) so the merged page keeps the cursor order.
func mergeByNameKey(cs *contentSearch, nameRows []*SearchResult) []*SearchResult {
	if cs == nil || len(cs.order) == 0 {
		return nameRows
	}
	// Merge may load further content pages; the page limit is applied by the caller.
	room := len(nameRows) + 1
	merged := make([]*SearchResult, 0, len(nameRows)+cs.len())
	i := 0
	for {
		path, ok := cs.peek(room)
		if !ok || i >= len(nameRows) {
			break
		}
		if pathKeyLess(path, nameRows[i].Path) {
			cs.claim(path)
			merged = append(merged, contentResult(cs, path))
			continue
		}
		merged = append(merged, nameRows[i])
		i++
		room++
	}
	for i < len(nameRows) {
		merged = append(merged, nameRows[i])
		i++
	}
	for {
		path, ok := cs.peek(room)
		if !ok {
			break
		}
		cs.claim(path)
		merged = append(merged, contentResult(cs, path))
	}
	return merged
}

// smallestCandidate returns the content hit with the smallest cursor key across all
// searches, reading more pages only while a search still has budget.
func smallestCandidate(searches []*contentSearch, budget int) (*contentSearch, string, bool) {
	var (
		bestCS   *contentSearch
		bestPath string
	)
	for _, cs := range searches {
		path, ok := cs.peek(budget)
		if !ok {
			continue
		}
		if bestCS == nil || sourceKeyLess(path, cs.source, bestPath, bestCS.source) {
			bestCS, bestPath = cs, path
		}
	}
	return bestCS, bestPath, bestCS != nil
}

// resultKey matches how each search keys its result map: single-source by path,
// multi-source by source:path.
func resultKey(cs *contentSearch, path string, results map[string]*SearchResult) string {
	if !cs.multi {
		return path
	}
	return cs.source + ":" + path
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
			// The cursor advanced past (path, source): the cursor's own source resumes
			// strictly after it, every other source still owns a hit at that same path.
			csOpts := searchOptions
			if csOpts.CursorSource == "" {
				csOpts.CursorSource = source
			}
			cs := newContentSearch(db, source, normalizedScopes[source], csOpts, largest, useWildcard, limit)
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
	rows, err := db.SearchItemsMultiSource(sources, normalizedScopes, largest, nameGlobPatterns, globAnd, caseExact, cursorPath, searchOptions.CursorSource)
	if err != nil && !hasContent {
		return []*SearchResult{}
	}
	if err == nil {
		defer rows.Close()
	}

	// Name and content candidates are merged by (length, path, source), so a shallower
	// content-only hit is never displaced by a deeper name hit filling the page.
	var (
		rowSource   string
		rowPath     string
		name        string
		size        int64
		createdTime int64
		modTime     int64
		mimeType    string
		isDir       bool
		hasPreview  bool
	)
	// Rows arrive in the same order the result page keeps, so reading past the limit
	// would only build SearchResults that are dropped later.
	var nameRows []*SearchResult
	for err == nil && rows.Next() {
		if limit > 0 && len(nameRows) >= limit {
			break
		}
		value, found := sessionInProgress.Load(sourceSession)
		if !found || value != runningHash {
			return []*SearchResult{}
		}

		if err := rows.Scan(&rowSource, &rowPath, &name, &size, &createdTime, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
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
		matchedContent := contentHit(content, rowSource, rowPath)
		if !itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameGlobPatterns) && !matchedContent {
			continue
		}
		resType := mimeType
		if isDir {
			resType = "directory"
		}
		res := &SearchResult{
			Path:       rowPath,
			Type:       resType,
			Size:       size,
			Modified:   item.ModTime.Format(time.RFC3339),
			HasPreview: hasPreview,
			Source:     rowSource,
		}
		if createdTime > 0 {
			res.Created = time.Unix(createdTime, 0).Format(time.RFC3339)
		}
		nameRows = append(nameRows, res)
	}
	if err == nil {
		rows.Close()
	}
	for _, res := range mergeMulti(content, nameRows) {
		if limit > 0 && count >= limit {
			break
		}
		key := res.Source + ":" + res.Path
		if _, dup := results[key]; dup {
			continue
		}
		results[key] = res
		count++
	}

	// Content-only hits are appended, in cursor order, until the page is full.
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
