package http

import (
	"net/http"
	"github.com/gtsteffaniak/filebrowser/search"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")
	files, dirs, fileTypes := search.SearchAllIndexes(query, r.URL.Path)
	for _,path := range(files){
		f := fileTypes[path]
		responseObj := map[string]interface{}{
			"dir"		:  	false,
			"path"		: 	path,
		}
		for _,filterType := range(search.FilterableTypes) {
			if f[filterType] { responseObj[filterType] = f[filterType] }
		}
		response = append(response,responseObj)
	}
	for _,v := range(dirs){
		response = append(response, map[string]interface{}{
			"dir":  true,
			"path": v,
		})
	}
	return renderJSON(w, r, response)
})