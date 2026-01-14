package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// getJobsHandler returns job info for the user.
// @Summary Get jobs info
// @Description Returns job info for the user.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param action path string true "Job action"
// @Param target path string true "Job target"
// @Success 200 {object} map[string]indexing.ReducedIndex "Job info"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/jobs/{action}/{target} [get]
func getJobsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sources := d.user.GetSourceNames()
	reducedIndexes := map[string]indexing.ReducedIndex{}
	for _, source := range sources {
		reducedIndex, err := indexing.GetIndexInfo(source, false)
		if err != nil {
			logger.Debugf("error getting index info: %v", err)
			continue
		}
		reducedIndexes[source] = reducedIndex
	}
	return renderJSON(w, r, reducedIndexes)
}
