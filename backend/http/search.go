package http

import (
	"github.com/gtsteffaniak/filebrowser/search"
	"net/http"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")
	indexInfo, fileTypes := search.SearchAllIndexes(query, r.URL.Path)
	for _, path := range indexInfo {
		f := fileTypes[path]
		responseObj := map[string]interface{}{
			"path": path,
		}
		for filterType, _ := range f {
			if f[filterType] {
				responseObj[filterType] = f[filterType]
			}
		}
		response = append(response, responseObj)
	}
	return renderJSON(w, r, response)
})
