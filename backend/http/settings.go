package http

import (
	"encoding/json"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

type settingsData struct {
	Signup           bool                  `json:"signup"`
	CreateUserDir    bool                  `json:"createUserDir"`
	UserHomeBasePath string                `json:"userHomeBasePath"`
	Defaults         settings.UserDefaults `json:"defaults"`
	Rules            []users.Rule          `json:"rules"`
	Frontend         settings.Frontend     `json:"frontend"`
	Commands         map[string][]string   `json:"commands"`
}

// settingsGetHandler retrieves the current system settings.
// @Summary Get system settings
// @Description Returns the current configuration settings for signup, user directories, rules, frontend, and commands.
// @Tags Settings
// @Accept json
// @Produce json
// @Success 200 {object} settingsData "System settings data"
// @Router /api/settings [get]
func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	data := &settingsData{
		Signup:           config.Auth.Signup,
		CreateUserDir:    config.Server.CreateUserDir,
		UserHomeBasePath: config.Server.UserHomeBasePath,
		Defaults:         config.UserDefaults,
		Rules:            config.Rules,
		Frontend:         config.Frontend,
	}

	return renderJSON(w, r, data)
}

// settingsPutHandler updates the system settings.
// @Summary Update system settings
// @Description Updates the system configuration settings for signup, user directories, rules, frontend, and commands.
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

	config.Server.CreateUserDir = req.CreateUserDir
	config.Server.UserHomeBasePath = req.UserHomeBasePath
	config.UserDefaults = req.Defaults
	config.Rules = req.Rules
	config.Frontend = req.Frontend
	config.Auth.Signup = req.Signup
	err = store.Settings.Save(config)
	return errToStatus(err), err
}
