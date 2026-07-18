package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/analytics"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/go-logger/logger"
)

type analyticsStatusResponse struct {
	Enabled          bool `json:"enabled"`
	Ready            bool `json:"ready"`
	PublishSupported bool `json:"publishSupported"`
}

type analyticsPatchRequest struct {
	Enabled bool `json:"enabled"`
}

func settingsAnalyticsGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	return RenderJSON(w, r, analyticsStatusResponse{
		Enabled:          state.IsAnalyticsEnabled(),
		Ready:            analytics.Ready(),
		PublishSupported: analytics.PublishSupported(),
	})
}

func settingsAnalyticsPatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var body analyticsPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	if body.Enabled && !analytics.PublishSupported() {
		return http.StatusForbidden, fmt.Errorf("deployment analytics publishing is not available in this build")
	}

	if err := state.SetAnalyticsEnabled(body.Enabled); err != nil {
		logger.Errorf("failed to update analytics setting: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to update analytics setting")
	}

	if !body.Enabled {
		analytics.NotifyAnalyticsDisabled()
		analytics.InvalidateCache()
	} else {
		analytics.NotifyAnalyticsEnabled()
	}

	return RenderJSON(w, r, analyticsStatusResponse{
		Enabled:          state.IsAnalyticsEnabled(),
		Ready:            analytics.Ready(),
		PublishSupported: analytics.PublishSupported(),
	})
}

func settingsAnalyticsPreviewHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	preview, err := analytics.PreviewSnapshot()
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("failed to build analytics preview: %w", err)
	}

	var payload any
	if err := json.Unmarshal(preview, &payload); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to format analytics preview: %w", err)
	}
	return RenderJSON(w, r, payload)
}
