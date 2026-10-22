package indexing

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var SearchResultsCache = cache.NewCache[[]string](15 * time.Second)

var DefaultSearchResults = 100

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
func (idx *Index) Search(ctx context.Context, search string, scope string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	return idx.SearchParsed(ctx, iteminfo.ParseSearch(search), scope, largest, limit, olderThanUnix, newerThanUnix, useWildcard)
}

// SearchParsed runs indexing search using pre-built options (e.g. repeated HTTP term parameters plus filter prefix).
func (idx *Index) SearchParsed(ctx context.Context, baseOpts iteminfo.SearchOptions, scope string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	scope = utils.AddTrailingSlashIfNotExists(scope)

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

	nameFilter := iteminfo.NameFilterForSearch(searchOptions, useWildcard, largest)
	nameFiltered := nameFilter != nil && len(nameFilter.Exact) > 0

	rows, err := idx.db.SearchItems(ctx, idx.Name, scope, largest, nameFilter)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
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

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameFiltered)

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
			if createdTime > 0 {
				results[path].Created = time.Unix(createdTime, 0).Format(time.RFC3339)
			}
			count++
		}
	}

	return sortSearchResults(results)
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

// itemMatchesSearchFilters applies the remaining filters in Go. nameFiltered is true
// when the SQL already applied the exact name predicate, so only the auxiliary
// (type/size/date) filters are re-checked here.
func itemMatchesSearchFilters(item iteminfo.ItemInfo, isDir bool, searchOptions iteminfo.SearchOptions, largest bool, nameFiltered bool) bool {
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
	if nameFiltered {
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
func SearchMultiSources(ctx context.Context, search string, sources []string, sourceScopes map[string]string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
	return SearchMultiSourcesParsed(ctx, iteminfo.ParseSearch(search), sources, sourceScopes, largest, limit, olderThanUnix, newerThanUnix, useWildcard)
}

// SearchMultiSourcesParsed searches multiple sources using pre-built options.
func SearchMultiSourcesParsed(ctx context.Context, baseOpts iteminfo.SearchOptions, sources []string, sourceScopes map[string]string, largest bool, limit int, olderThanUnix, newerThanUnix int64, useWildcard bool) []*SearchResult {
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

	nameFilter := iteminfo.NameFilterForSearch(searchOptions, useWildcard, largest)
	nameFiltered := nameFilter != nil && len(nameFilter.Exact) > 0

	rows, err := db.SearchItemsMultiSource(ctx, sources, normalizedScopes, largest, nameFilter)
	if err != nil {
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
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

		matches := itemMatchesSearchFilters(item, isDir, searchOptions, largest, nameFiltered)

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

	return sortSearchResults(results)
}
