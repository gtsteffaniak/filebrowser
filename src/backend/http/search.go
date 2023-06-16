package http

import (
	"net/http"
	"github.com/gtsteffaniak/filebrowser/search"
)

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := []map[string]interface{}{}
	query := r.URL.Query().Get("query")
	var files []string
	var dirs []string
	files, dirs = search.IndexedSearch(query,r.URL.Path,&files,&dirs)
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
	files = files[:0]
	dirs = dirs[:0]
//	err := search.Search(d.user.Fs, r.URL.Path, query, d, func(path string, f os.FileInfo) error {
//		response = append(response, map[string]interface{}{
//			"dir":  f.IsDir(),
//			"path": path,
//		})
//
//		return nil
//	})

	return renderJSON(w, r, response)
})
