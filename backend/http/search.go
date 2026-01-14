package http

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

type searchOptions struct {
	query        string
	sources      []string
	searchScope  string
	combinedPath map[string]string // source -> combinedPath
	sessionId    string
	largest      bool
}

// searchHandler handles search requests for files based on the provided query.
//
// This endpoint processes a search query, retrieves relevant file paths, and
// returns a JSON response with the search results. The search is performed
// against the file index, which is built from the root directory specified in
// the server's configuration. The results are filtered based on the user's scope.
//
// The handler supports searching a single source (using the 'source' parameter)
// or multiple sources (using the 'sources' parameter). When multiple sources
// are specified, the scope is always set to the user's scope for each source.
//
// The handler expects the following headers in the request:
// - SessionId: A unique identifier for the user's session.
//
// The request URL should include query parameters:
// - query: The search terms to use (required)
// - source: Source name (deprecated, use 'sources' instead)
// - sources: Comma-separated list of source names (e.g., "source1,source2")
// - scope: Optional path within user scope to search
//
// Example request (single source):
//
//	GET api/search?query=myfile&source=mysource
//
// Example request (multiple sources):
//
//	GET api/search?query=myfile&sources=source1,source2
//
// Example response:
// [
//
//	{
//	    "path": "/path/to/myfile.txt",
//	    "type": "text",
//	    "source": "mysource"
//	},
//	{
//	    "path": "/path/to/mydir/",
//	    "type": "directory",
//	    "source": "mysource"
//	}
//
// ]
//
// @Summary Search Files
// @Description Searches for files matching the provided query. Returns file paths and metadata based on the user's session and scope. Supports searching across multiple sources when using the 'sources' parameter.
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param source query string false "Source name for the desired source (deprecated, use 'sources' instead)"
// @Param sources query string false "Comma-separated list of source names to search across multiple sources. When multiple sources are specified, scope is always the user's scope for each source."
// @Param scope query string false "path within user scope to search, for example '/first/second' to search within the second directory only. Ignored when multiple sources are specified."
// @Param SessionId header string false "User session ID, add unique value to prevent collisions"
// @Success 200 {array} indexing.SearchResult "List of search results with source field populated"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	searchOptions, err := prepSearchOptions(r, d)
	if err != nil {
		return http.StatusBadRequest, err
	}

	var response []*indexing.SearchResult
	if len(searchOptions.sources) == 1 {
		// Single source - use the existing Search method for backward compatibility
		index := indexing.GetIndex(searchOptions.sources[0])
		if index == nil {
			return http.StatusBadRequest, fmt.Errorf("index not found for source %s", searchOptions.sources[0])
		}
		combinedPath := searchOptions.combinedPath[searchOptions.sources[0]]
		response = index.Search(searchOptions.query, combinedPath, searchOptions.sessionId, searchOptions.largest, indexing.DefaultSearchResults)
	} else {
		// Multiple sources - use the new SearchMultiSources function
		response = indexing.SearchMultiSources(searchOptions.query, searchOptions.sources, searchOptions.combinedPath, searchOptions.sessionId, searchOptions.largest, indexing.DefaultSearchResults)
	}

	// Filter out items that are not permitted according to access rules and trim user scope from paths
	filteredResponse := make([]*indexing.SearchResult, 0, len(response))
	for _, result := range response {
		index := indexing.GetIndex(result.Source)
		combinedPath := searchOptions.combinedPath[result.Source]
		indexPath := utils.JoinPathAsUnix(combinedPath, result.Path)
		if store.Access != nil && !store.Access.Permitted(index.Path, indexPath, d.user.Username) {
			continue // Silently skip this file/folder
		}
		// Remove the user scope from the path (modifying in place is safe - these are fresh allocations)
		result.Path = strings.TrimPrefix(result.Path, combinedPath)
		if result.Path == "" {
			result.Path = "/"
		}
		filteredResponse = append(filteredResponse, result)
	}
	return renderJSON(w, r, filteredResponse)
}

func prepSearchOptions(r *http.Request, d *requestContext) (*searchOptions, error) {
	query := r.URL.Query().Get("query")
	sourcesParam := r.URL.Query().Get("sources")
	sourceParam := r.URL.Query().Get("source") // deprecated, but still supported
	scope := r.URL.Query().Get("scope")
	largest := r.URL.Query().Get("largest") == "true"

	var sources []string
	if sourcesParam != "" {
		sources = strings.Split(sourcesParam, ",")
	} else if sourceParam != "" {
		sources = []string{sourceParam}
	} else {
		return nil, fmt.Errorf("either 'source' or 'sources' query parameter is required")
	}

	// Validate all sources exist
	for _, source := range sources {
		index := indexing.GetIndex(source)
		if index == nil {
			return nil, fmt.Errorf("index not found for source %s", source)
		}
	}

	unencodedScope, err := url.PathUnescape(scope)
	if err != nil {
		return nil, fmt.Errorf("invalid path encoding: %v", err)
	}
	if len(query) < settings.Config.Server.MinSearchLength && !largest {
		return nil, fmt.Errorf("query is too short, minimum length is %d", settings.Config.Server.MinSearchLength)
	}
	searchScope := strings.TrimPrefix(unencodedScope, ".")

	// If multiple sources, always use user scope (ignore searchScope)
	if len(sources) > 1 {
		searchScope = ""
	}

	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")

	// Build combinedPath map for each source
	combinedPathMap := make(map[string]string)
	for _, source := range sources {
		index := indexing.GetIndex(source)
		userscope, err := d.user.GetScopeForSourceName(source)
		if err != nil {
			return nil, err
		}
		combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope), true) // searchScope is a directory
		combinedPathMap[source] = combinedPath
	}

	return &searchOptions{
		query:        query,
		sources:      sources,
		searchScope:  searchScope,
		combinedPath: combinedPathMap,
		sessionId:    sessionId,
		largest:      largest,
	}, nil
}
