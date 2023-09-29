package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/index"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")

	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	index := *index.GetIndex()
	results, fileTypes := index.Search(query, r.URL.Path, sessionId)
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
