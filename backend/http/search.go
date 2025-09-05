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
	query := r.URL.Query().Get("query")
	source := r.URL.Query().Get("source")
	scope := r.URL.Query().Get("scope")
	unencodedScope, err := url.QueryUnescape(scope)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if len(query) < settings.Config.Server.MinSearchLength {
		return http.StatusBadRequest, fmt.Errorf("query is too short, minimum length is %d", settings.Config.Server.MinSearchLength)
	}
	searchScope := strings.TrimPrefix(unencodedScope, ".")
	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	index := indexing.GetIndex(source)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", source)
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope))
	combinedPath = strings.TrimSuffix(combinedPath, "/") + "/" // Ensure trailing slash
	// Perform the search using the provided query and user scope
	response := index.Search(query, combinedPath, sessionId)
	for i := range response {
		// Remove the user scope from the path
		response[i].Path = strings.TrimPrefix(response[i].Path, combinedPath)
		if response[i].Path == "" {
			response[i].Path = "/"
		}
	}
	// trim user scope from each result path
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	return renderJSON(w, r, response)
}
