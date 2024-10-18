package http

import (
	"encoding/json"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
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
