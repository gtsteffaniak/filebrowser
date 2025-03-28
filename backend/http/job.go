package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func getJobHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sources := settings.GetSources(d.user)
	return renderJSON(w, r, indexing.GetIndexesInfo(sources...))
}
