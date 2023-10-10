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
	Shell            []string              `json:"shell"`
	Commands         map[string][]string   `json:"commands"`
}

var settingsGetHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	data := &settingsData{
		Signup:           settings.GlobalConfiguration.Auth.Signup,
		CreateUserDir:    settings.GlobalConfiguration.Server.CreateUserDir,
		UserHomeBasePath: settings.GlobalConfiguration.Server.UserHomeBasePath,
		Defaults:         d.settings.UserDefaults,
		Rules:            d.settings.Rules,
		Frontend:         d.settings.Frontend,
		Shell:            d.settings.Shell,
		Commands:         d.settings.Commands,
	}

	return renderJSON(w, r, data)
})

var settingsPutHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
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
	d.settings.Shell = req.Shell
	d.settings.Commands = req.Commands
	err = d.store.Settings.Save(d.settings)
	return errToStatus(err), err
})
