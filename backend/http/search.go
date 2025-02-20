package http

import (
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/files"
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
// @Param scope query string false "path within user scope to search, for example '/first/second' to search within the second directory only"
// @Param SessionId header string false "User session ID, add unique value to prevent collisions"
// @Success 200 {array} files.SearchResult "List of search results"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	query := r.URL.Query().Get("query")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "default"
	}
	searchScope := strings.TrimPrefix(r.URL.Query().Get("scope"), ".")
	searchScope = strings.TrimPrefix(searchScope, "/")
	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	index := files.GetIndex(source)
	userScope := strings.TrimPrefix(d.user.Scopes["default"], ".")
	combinedScope := strings.TrimPrefix(userScope+"/"+searchScope, "/")

	// Perform the search using the provided query and user scope
	response := index.Search(query, combinedScope, sessionId)
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	return renderJSON(w, r, response)
}
