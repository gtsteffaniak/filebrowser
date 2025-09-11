package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

// settingsGetHandler retrieves the current system settings.
// @Summary Get system settings
// @Description Returns the current configuration settings for signup, user directories, rules, frontend.
// @Tags Settings
// @Accept json
// @Produce json
// @Param property query string false "Property to retrieve: `userDefaults`, `frontend`, `auth`, `server`, `sources`"
// @Success 200 {object} settings.Settings "System settings data"
// @Router /api/settings [get]
func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	property := r.URL.Query().Get("property")
	if property != "" {
		// get property by name
		switch property {
		case "userDefaults":
			return renderJSON(w, r, config.UserDefaults)
		case "frontend":
			return renderJSON(w, r, config.Frontend)
		case "auth":
			return renderJSON(w, r, config.Auth)
		case "server":
			return renderJSON(w, r, config.Server)
		case "sources":
			return renderJSON(w, r, config.Server.Sources)
		default:
			return http.StatusNotFound, nil
		}
	}
	return renderJSON(w, r, config)
}

// settingsConfigHandler returns the current system settings as YAML.
// @Summary Get system settings as YAML
// @Description Returns the current running configuration settings as YAML format with optional comments and filtering.
// @Tags Settings
// @Accept json
// @Produce text/plain
// @Param full query boolean false "Show all values (true) or only non-default values (false, default)"
// @Param comments query boolean false "Include comments in YAML (true) or plain YAML (false, default)"
// @Success 200 {string} string "System settings in YAML format"
// @Router /api/settings/config [get]
func settingsConfigHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Parse query parameters
	fullParam := r.URL.Query().Get("full")
	commentsParam := r.URL.Query().Get("comments")

	showFull := fullParam == "true"
	showComments := commentsParam == "true"

	// First, try to generate YAML using the existing generator function (don't filter deprecated fields in API)
	yamlOutput, err := settings.GenerateConfigYaml(config, showComments, showFull, false)
	if err != nil {
		// If the primary method fails, try to use embedded YAML as fallback
		if showComments {
			embeddedYaml, readErr := assets.ReadFile("embed/config.generated.yaml")
			if readErr == nil {
				yamlOutput, err = settings.GenerateConfigYamlWithEmbedded(config, showComments, showFull, false, string(embeddedYaml))
			}
		}

		// If still failing, return the error
		if err != nil {
			return http.StatusInternalServerError, err
		}
	}

	// Set content type and write response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(yamlOutput)); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
