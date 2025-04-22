package http

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func getJobsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sources := settings.GetSources(d.user)
	reducedIndexes := map[string]indexing.ReducedIndex{}
	for _, source := range sources {
		reducedIndex, err := indexing.GetIndexInfo(source)
		if err != nil {
			logger.Debug(fmt.Sprintf("error getting index info: %v", err))
			continue
		}
		reducedIndexes[source] = reducedIndex
	}
	return renderJSON(w, r, reducedIndexes)
}
