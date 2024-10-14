package http

import (
	"encoding/json"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/settings"
)

type settingsData struct {
	Signup           bool                  `json:"signup"`
	CreateUserDir    bool                  `json:"createUserDir"`
	UserHomeBasePath string                `json:"userHomeBasePath"`
	Defaults         settings.UserDefaults `json:"defaults"`
	Rules            []rules.Rule          `json:"rules"`
	Frontend         settings.Frontend     `json:"frontend"`
	Commands         map[string][]string   `json:"commands"`
}

func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	data := &settingsData{
		Signup:           d.settings.Auth.Signup,
		CreateUserDir:    d.settings.Server.CreateUserDir,
		UserHomeBasePath: d.settings.Server.UserHomeBasePath,
		Defaults:         d.settings.UserDefaults,
		Rules:            d.settings.Rules,
		Frontend:         d.settings.Frontend,
	}

	return renderJSON(w, r, data)
}

func settingsPutHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	req := &settingsData{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		return http.StatusBadRequest, err
	}

	d.settings.Server.CreateUserDir = req.CreateUserDir
	d.settings.Server.UserHomeBasePath = req.UserHomeBasePath
	d.settings.UserDefaults = req.Defaults
	d.settings.Rules = req.Rules
	d.settings.Frontend = req.Frontend
	d.settings.Auth.Signup = req.Signup
	err = d.store.Settings.Save(d.settings)
	return errToStatus(err), err
}
