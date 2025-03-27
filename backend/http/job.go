package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func getJobHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	return renderJSON(w, r, indexing.GetIndexesInfo())
}
