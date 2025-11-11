package http

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

type searchOptions struct {
	query        string
	source       string
	searchScope  string
	combinedPath string
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
// The handler expects the following headers in the request:
// - SessionId: A unique identifier for the user's session.
// - UserScope: The scope of the user, which influences the search context.
//
// The request URL should include a query parameter named `query` that specifies
// the search terms to use. The response will include an array of searchResponse objects
// containing the path, type, and dir status.
//
// Example request:
//
//	GET api/search?query=myfile
//
// Example response:
// [
//
//	{
//	    "path": "/path/to/myfile.txt",
//	    "type": "text"
//	},
//	{
//	    "path": "/path/to/mydir/",
//	    "type": "directory"
//	}
//
// ]
//
// @Summary Search Files
// @Description Searches for files matching the provided query. Returns file paths and metadata based on the user's session and scope.
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param source query string true "Source name for the desired source"
// @Param scope query string false "path within user scope to search, for example '/first/second' to search within the second directory only"
// @Param SessionId header string false "User session ID, add unique value to prevent collisions"
// @Success 200 {array} indexing.SearchResult "List of search results"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	searchOptions, err := prepSearchOptions(r, d)
	if err != nil {
		return http.StatusBadRequest, err
	}
	index := indexing.GetIndex(searchOptions.source)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", searchOptions.source)
	}

	fmt.Printf("searchOptions largest=%v query=%s combinedPath=%s sessionId=%s source=%s searchScope=%s", searchOptions.largest, searchOptions.query, searchOptions.combinedPath, searchOptions.sessionId, searchOptions.source, searchOptions.searchScope)

	// Perform the search using the provided query and user scope
	response := index.Search(searchOptions.query, searchOptions.combinedPath, searchOptions.sessionId, searchOptions.largest)
	// Remove the user scope from the path (modifying in place is safe - these are fresh allocations)
	for _, result := range response {
		result.Path = strings.TrimPrefix(result.Path, searchOptions.combinedPath)
		if result.Path == "" {
			result.Path = "/"
		}
	}
	return renderJSON(w, r, response)
}

func prepSearchOptions(r *http.Request, d *requestContext) (*searchOptions, error) {
	query := r.URL.Query().Get("query")
	source := r.URL.Query().Get("source")
	scope := r.URL.Query().Get("scope")
	largest := r.URL.Query().Get("largest") == "true"
	unencodedScope, err := url.PathUnescape(scope)
	if err != nil {
		return nil, fmt.Errorf("invalid path encoding: %v", err)
	}
	if len(query) < settings.Config.Server.MinSearchLength && !largest {
		return nil, fmt.Errorf("query is too short, minimum length is %d", settings.Config.Server.MinSearchLength)
	}
	searchScope := strings.TrimPrefix(unencodedScope, ".")
	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	index := indexing.GetIndex(source)
	if index == nil {
		return nil, fmt.Errorf("index not found for source %s", source)
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return nil, err
	}
	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope))
	return &searchOptions{
		query:        query,
		source:       source,
		searchScope:  searchScope,
		combinedPath: combinedPath,
		sessionId:    sessionId,
		largest:      largest,
	}, nil
}
