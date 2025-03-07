package http

import (
	"encoding/json"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

type settingsData struct {
	Signup           bool                  `json:"signup"`
	CreateUserDir    bool                  `json:"createUserDir"`
	UserHomeBasePath string                `json:"userHomeBasePath"`
	Defaults         settings.UserDefaults `json:"defaults"`
	Frontend         settings.Frontend     `json:"frontend"`
}

// settingsGetHandler retrieves the current system settings.
// @Summary Get system settings
// @Description Returns the current configuration settings for signup, user directories, rules, frontend.
// @Tags Settings
// @Accept json
// @Produce json
// @Param property query string false "Property to retrieve"
// @Success 200 {object} settingsData "System settings data"
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

// settingsPutHandler updates the system settings.
// @Summary Update system settings
// @Description Updates the system configuration settings for signup, user directories, rules, frontend.
// @Tags Settings
// @Accept json
// @Produce json
// @Param body body settingsData true "Settings data to update"
// @Success 200 "Settings updated successfully"
// @Failure 400 {object} map[string]string "Bad request - failed to decode body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/settings [put]
func settingsPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	req := &settingsData{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		return http.StatusBadRequest, err
	}

	config.Server.UserHomeBasePath = req.UserHomeBasePath
	config.UserDefaults = req.Defaults
	config.Frontend = req.Frontend
	config.Auth.Signup = req.Signup
	err = store.Settings.Save(config)
	return errToStatus(err), err
}
