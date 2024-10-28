package http

import (
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/settings"
)

// swagger reference
type searchResult struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
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
// @Tags search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param SessionId header string false "User session ID, add unique value to prevent collisions"
// @Param UserScope header string true "User scope for the search"
// @Success 200 {array} searchResult "List of search results"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	query := r.URL.Query().Get("query")

	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	userScope := r.Header.Get("UserScope")
	index := files.GetIndex(settings.Config.Server.Root)
	adjustedRestPath := strings.TrimPrefix(r.URL.Path, "/search")
	combinedScope := strings.TrimPrefix(userScope+adjustedRestPath, ".")

	// Perform the search using the provided query and user scope
	response := index.Search(query, combinedScope, sessionId)
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	return renderJSON(w, r, response)
}
