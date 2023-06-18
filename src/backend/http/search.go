package http

import (
	"net/http"
	"github.com/gtsteffaniak/filebrowser/search"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")
	files, dirs := search.SearchAllIndexes(query, r.URL.Path)
	for _,v := range(files){
		response = append(response, map[string]interface{}{
			"dir":  false,
			"path": v,
		})
	}
	for _,v := range(dirs){
		response = append(response, map[string]interface{}{
			"dir":  true,
			"path": v,
		})
	}
	return renderJSON(w, r, response)
})