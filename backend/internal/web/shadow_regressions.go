package web

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/shadowregress"
	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/go-logger/logger"
)

// shadowRegressionStore holds dev-mode shadow regressions. It opens lazily, so
// production never creates the database file unless the viewer is used.
var shadowRegressionStore = &shadowregress.Store{}

// registerShadowRegressionSink routes shadow regressions from gomedia into the
// database. Recording only happens in shadow mode, so this is safe in production.
func registerShadowRegressionSink() {
	gomedia.SetRegressionSink(func(r gomedia.Regression) {
		err := shadowRegressionStore.Record(shadowregress.Entry{
			CreatedAt: r.CreatedAt,
			Operation: r.Operation,
			Label:     r.Label,
			Detail:    r.Detail,
			Format:    r.Format,
			SizeBytes: r.SizeBytes,
			Codecs:    r.Codecs,
		})
		if err != nil {
			logger.Errorf("failed to record shadow regression for %s: %v", r.Operation, err)
		}
	})
}

func settingsShadowRegressionsGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	entries, err := shadowRegressionStore.List()
	if err != nil {
		logger.Errorf("failed to read shadow regressions: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to read shadow regressions")
	}
	return RenderJSON(w, r, entries)
}

func settingsShadowRegressionsClearHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if err := shadowRegressionStore.Clear(); err != nil {
		logger.Errorf("failed to clear shadow regressions: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to clear shadow regressions")
	}
	return RenderJSON(w, r, map[string]bool{"cleared": true})
}
