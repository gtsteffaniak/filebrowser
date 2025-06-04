package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

func getJobsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sources := settings.GetSources(d.user)
	reducedIndexes := map[string]indexing.ReducedIndex{}
	for _, source := range sources {
		reducedIndex, err := indexing.GetIndexInfo(source)
		if err != nil {
			logger.Debugf("error getting index info: %v", err)
			continue
		}
		reducedIndexes[source] = reducedIndex
	}
	return renderJSON(w, r, reducedIndexes)
}
