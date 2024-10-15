package http

import (
	"net/http"
	"path"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/version"
)

var templateRenderer *TemplateRenderer

type TemplateRenderer struct {
	templates *template.Template
}

// Render renders a template document with headers and data
func (t *TemplateRenderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	// Set headers
	w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Accel-Expires", "0")
	w.Header().Set("Transfer-Encoding", "identity")

	// Execute the template with the provided data
	return t.templates.ExecuteTemplate(w, name, data)
}

func handleWithStaticData(w http.ResponseWriter, r *http.Request, file, contentType string) {
	w.Header().Set("Content-Type", contentType)

	auther, err := store.Auth.Get(config.Auth.Method)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Name":                  config.Frontend.Name,
		"DisableExternal":       config.Frontend.DisableExternal,
		"DisableUsedPercentage": config.Frontend.DisableUsedPercentage,
		"darkMode":              settings.Config.UserDefaults.DarkMode,
		"Color":                 config.Frontend.Color,
		"BaseURL":               config.Server.BaseURL,
		"Version":               version.Version,
		"CommitSHA":             version.CommitSHA,
		"StaticURL":             path.Join(config.Server.BaseURL, "/static"),
		"Signup":                settings.Config.Auth.Signup,
		"NoAuth":                config.Auth.Method == "noauth",
		"AuthMethod":            config.Auth.Method,
		"LoginPage":             auther.LoginPage(),
		"CSS":                   false,
		"ReCaptcha":             false,
		"EnableThumbs":          config.Server.EnableThumbnails,
		"ResizePreview":         config.Server.ResizePreview,
		"EnableExec":            config.Server.EnableExec,
	}
	// Render the template with global variables
	if err := templateRenderer.Render(w, file, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func staticFilesHandler(w http.ResponseWriter, r *http.Request) {
	handleWithStaticData(w, r, "index.html", "text/html")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	handleWithStaticData(w, r, "index.html", "text/html")
}
