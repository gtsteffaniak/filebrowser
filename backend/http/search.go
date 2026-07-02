package http

import (
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

type searchOptions struct {
	parsed        iteminfo.SearchOptions
	sources       []string
	searchScope   string
	combinedPath  map[string]string // source -> combinedPath
	sessionId     string
	largest       bool
	useWildcard   bool
	olderThanUnix int64 // optional; 0 = unset. Modified time must be strictly before this Unix second.
	newerThanUnix int64 // optional; 0 = unset. Modified time must be >= this Unix second.
}

// scopedSourcePath is one repeated "scope" query value using "sourceName:relativePath"
// (split on the first ':'). Path is relative within the user's scope for that source; an
// empty path after ':' means "/" . Example: scope=mydisk:/photos&scope=backup:/archive
type scopedSourcePath struct {
	source  string
	relPath string
}

// searchHandler handles search requests for files based on the provided query.
//
// Per-source search scope:
//
//	Repeated query parameter "scope" with the value "sourceName:relativePath",
//	split on the first colon. Encode the whole value with normal query string rules (e.g. %3A for ':').
//	Examples:
//	  ?scope=mydisk:/&scope=backup:/Photos
//	Duplicate source names: the last repeated scope for that source wins.
//
// When scope is omitted, each listed source is searched from the root of the user's scope.
//
// Query parameters:
// - query: Structured filter prefix, or full search string when "terms" parameters are not used
// - terms: Repeated query parameter; each value is one literal search term. OR-combined by default; use termJoin=and for AND.
// - termJoin: Optional; "and" requires every term to match; any other value keeps OR semantics (default).
// - sources: Comma-separated list of source names when not using repeated scope=source:path params
// - scope: Repeated "sourceName:relativePath" per source
// - olderThan: Optional Unix time in seconds; only items modified strictly before this instant
// - newerThan: Optional Unix time in seconds; only items modified on or after this instant
// - useWildcard: Optional; when true, file names are matched with SQLite GLOB (wildcards) instead of substring search
//
// @Summary Search Files
// @Description Searches for files matching the provided query. Returns file paths and metadata based on the user's session and scope. Supports searching across multiple sources when using the 'sources' parameter.
// @Tags Tools
// @Accept json
// @Produce json
// @Param query query string false "Filter prefix or full search text (required when no terms are supplied)"
// @Param terms query []string false "Repeated: one literal search term per parameter; combined with OR unless termJoin=and"
// @Param sources query string false "Comma-separated source names when not using repeated scope=source:path"
// @Param scope query []string false "Repeated: 'sourceName:relativePath' per source"
// @Param olderThan query int false "Unix seconds; only results modified strictly before this time"
// @Param newerThan query int false "Unix seconds; only results modified on or after this time"
// @Param useWildcard query bool false "When true, match indexed file names with SQLite GLOB (wildcard patterns)"
// @Param termJoin query string false "Optional: 'and' to require all repeated 'terms' match; default is OR"
// @Success 200 {array} indexing.SearchResult "List of search results with source field populated"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/tools/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	searchOptions, err := prepSearchOptions(r, d)
	if err != nil {
		return http.StatusBadRequest, err
	}

	searchSize := indexing.DefaultSearchResults
	if searchOptions.largest {
		searchSize = 200
	}

	var response []*indexing.SearchResult
	if len(searchOptions.sources) == 1 {
		index := indexing.GetIndex(searchOptions.sources[0])
		if index == nil {
			return http.StatusBadRequest, fmt.Errorf("index not found for source %s", searchOptions.sources[0])
		}
		combinedPath := searchOptions.combinedPath[searchOptions.sources[0]]
		response = index.SearchParsed(searchOptions.parsed, combinedPath, searchOptions.sessionId, searchOptions.largest, searchSize, searchOptions.olderThanUnix, searchOptions.newerThanUnix, searchOptions.useWildcard)
	} else {
		response = indexing.SearchMultiSourcesParsed(searchOptions.parsed, searchOptions.sources, searchOptions.combinedPath, searchOptions.sessionId, searchOptions.largest, searchSize, searchOptions.olderThanUnix, searchOptions.newerThanUnix, searchOptions.useWildcard)
	}

	// Filter out items that are not permitted according to access rules and trim user scope from paths
	filteredResponse := make([]*indexing.SearchResult, 0, len(response))
	for _, result := range response {
		index := indexing.GetIndex(result.Source)
		combinedPath := searchOptions.combinedPath[result.Source]
		indexPath := utils.JoinPathAsUnix(combinedPath, result.Path)
		if accessStore != nil && !accessStore.Permitted(index.Path, utils.IndexPathFromNormalized(indexPath, true), d.user.Username) {
			continue // Silently skip this file/folder
		}
		// Remove the user scope from the path (modifying in place is safe - these are fresh allocations)
		result.Path = strings.TrimPrefix(result.Path, combinedPath)
		if result.Path == "" {
			result.Path = "/"
		}
		filteredResponse = append(filteredResponse, result)
		// This is to filter the ext-hidden files from search results, like the ones with the hidden property
		if d.user.HideFileExt != "" {
			filtered := filteredResponse[:0]
			for _, res := range filteredResponse {
				if res.Type == "directory" {
					filtered = append(filtered, res)
					continue
				}
				baseName := filepath.Base(res.Path)
				if !utils.HideFileByExt(baseName, d.user.HideFileExt) {
					filtered = append(filtered, res)
				}
			}
			filteredResponse = filtered
		}
	}
	return renderJSON(w, r, filteredResponse)
}

// parseRepeatedScopeParams interprets repeated "scope" query values as "sourceName:relativePath".
func parseRepeatedScopeParams(scopeQueryValues []string) ([]scopedSourcePath, error) {
	var clauses []scopedSourcePath
	for _, raw := range scopeQueryValues {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		idx := strings.IndexByte(raw, ':')
		if idx <= 0 {
			return nil, fmt.Errorf("invalid scope parameter %q: use scope=sourceName:relativePath", raw)
		}
		src := strings.TrimSpace(raw[:idx])
		if src == "" {
			return nil, fmt.Errorf("invalid scope parameter %q: empty source name before ':'", raw)
		}
		pathPart := strings.TrimSpace(raw[idx+1:])
		if pathPart == "" {
			pathPart = "/"
		}
		cleanPath, err := utils.SanitizePath(pathPart)
		if err != nil {
			return nil, fmt.Errorf("invalid path in scope parameter %q: %v", raw, err)
		}
		clauses = append(clauses, scopedSourcePath{source: src, relPath: cleanPath})
	}
	return clauses, nil
}

func prepSearchOptions(r *http.Request, d *requestContext) (*searchOptions, error) {
	query := r.URL.Query().Get("query")
	rawTerms := r.URL.Query()["terms"]
	termJoin := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("termJoin")))
	matchAllTerms := termJoin == "and"
	sourcesParam := r.URL.Query().Get("sources")
	scopeValues := r.URL.Query()["scope"]
	largest := r.URL.Query().Get("largest") == "true"
	wildRaw := strings.TrimSpace(r.URL.Query().Get("useWildcard"))
	useWildcard := strings.EqualFold(wildRaw, "true") || wildRaw == "1"
	olderThanUnix, err := parseOptionalUnixQueryParam("olderThan", r.URL.Query().Get("olderThan"))
	if err != nil {
		return nil, err
	}
	newerThanUnix, err := parseOptionalUnixQueryParam("newerThan", r.URL.Query().Get("newerThan"))
	if err != nil {
		return nil, err
	}

	scopedClauses, err := parseRepeatedScopeParams(scopeValues)
	if err != nil {
		return nil, err
	}

	normalizedTerms := make([]string, 0, len(rawTerms))
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		if t != "" {
			normalizedTerms = append(normalizedTerms, t)
		}
	}

	parsed := iteminfo.BuildSearchOptionsFromQuery(query, normalizedTerms, matchAllTerms)

	minLen := config.Server.MinSearchLength
	if !largest {
		if len(normalizedTerms) > 0 {
			for _, t := range normalizedTerms {
				if len(t) < minLen {
					return nil, fmt.Errorf("each term is too short, minimum length is %d", minLen)
				}
			}
		} else if len(strings.TrimSpace(query)) < minLen {
			return nil, fmt.Errorf("query is too short, minimum length is %d", minLen)
		}
	}

	sessionId := r.Header.Get("SessionId")

	var sources []string
	combinedPathMap := make(map[string]string)
	var searchScopeOut string

	if len(scopedClauses) > 0 {
		pathBySource := make(map[string]string)
		for _, c := range scopedClauses {
			if _, ok := pathBySource[c.source]; !ok {
				sources = append(sources, c.source)
			}
			pathBySource[c.source] = c.relPath
		}
		for _, source := range sources {
			index := indexing.GetIndex(source)
			if index == nil {
				return nil, fmt.Errorf("index not found for source %s", source)
			}
			userscope, err := d.user.GetScopeForSourceName(source)
			if err != nil {
				return nil, err
			}
			rel := strings.TrimPrefix(pathBySource[source], ".")
			combinedPathMap[source] = index.MakeIndexPath(filepath.Join(userscope, rel), true).String()
		}
	} else {
		if sourcesParam == "" {
			return nil, fmt.Errorf("'sources' or repeated scope=sourceName:path query parameters are required")
		}
		sources = strings.Split(sourcesParam, ",")
		for i := range sources {
			sources[i] = strings.TrimSpace(sources[i])
		}

		for _, source := range sources {
			index := indexing.GetIndex(source)
			if index == nil {
				return nil, fmt.Errorf("index not found for source %s", source)
			}
		}

		searchScopeOut = "/"

		if len(sources) > 1 {
			searchScopeOut = ""
		}

		for _, source := range sources {
			index := indexing.GetIndex(source)
			userscope, err := d.user.GetScopeForSourceName(source)
			if err != nil {
				return nil, err
			}
			combinedPathMap[source] = index.MakeIndexPath(filepath.Join(userscope, searchScopeOut), true).String()
		}
	}

	return &searchOptions{
		parsed:        parsed,
		sources:       sources,
		searchScope:   searchScopeOut,
		combinedPath:  combinedPathMap,
		sessionId:     sessionId,
		largest:       largest,
		useWildcard:   useWildcard,
		olderThanUnix: olderThanUnix,
		newerThanUnix: newerThanUnix,
	}, nil
}

func parseOptionalUnixQueryParam(name, raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be a non-negative Unix timestamp in seconds", name)
	}
	if v > math.MaxInt64 {
		return 0, fmt.Errorf("invalid %s: value too large", name)
	}
	return int64(v), nil
}
