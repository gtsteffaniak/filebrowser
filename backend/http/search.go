package http

import (
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/index"
	"github.com/gtsteffaniak/filebrowser/settings"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")
	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	userScope := r.Header.Get("UserScope")
	index := *index.GetIndex(settings.Config.Server.Root)
	combinedScope := strings.TrimPrefix(userScope+r.URL.Path, ".")
	combinedScope = strings.TrimPrefix(combinedScope, "/")
	results, fileTypes := index.Search(query, combinedScope, sessionId)
	for _, path := range results {
		responseObj := map[string]interface{}{
			"path": path,
			"dir":  true,
		}
		if _, ok := fileTypes[path]; ok {
			responseObj["dir"] = false
			for filterType, value := range fileTypes[path] {
				if value {
					responseObj[filterType] = value
				}
			}
		}
		response = append(response, responseObj)
	}
	return renderJSON(w, r, response)
})
