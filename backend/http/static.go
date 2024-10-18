package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/auth"
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

	fmt.Println("executing template", name)
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

	if config.Frontend.Files != "" {
		fPath := filepath.Join(config.Frontend.Files, "custom.css")
		_, err := os.Stat(fPath) //nolint:govet

		if err != nil && !os.IsNotExist(err) {
			log.Printf("couldn't load custom styles: %v", err)
		}

		if err == nil {
			data["CSS"] = true
		}
	}

	if config.Auth.Method == "password" {
		raw, err := store.Auth.Get(config.Auth.Method) //nolint:govet
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		auther, ok := raw.(*auth.JSONAuth)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if auther.ReCaptcha != nil {
			data["ReCaptcha"] = auther.ReCaptcha.Key != "" && auther.ReCaptcha.Secret != ""
			data["ReCaptchaHost"] = auther.ReCaptcha.Host
			data["ReCaptchaKey"] = auther.ReCaptcha.Key
		}
	}

	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	data["globalVars"] = strings.ReplaceAll(string(b), `'`, `\'`)

	// Render the template with global variables
	if err := templateRenderer.Render(w, file, data); err != nil {
		fmt.Println("could not render", file)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func staticFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.URL.Path == "/" {
		fmt.Println("indexHandler", r.URL.Path)
		handleWithStaticData(w, r, "index.html", "text/html")
	} else {
		//staticPath := strings.TrimPrefix(r.URL.Path, "/static/")
		const maxAge = 86400 // 1 day
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))

		http.FileServer(http.FS(assetFs)).ServeHTTP(w, r)

	}
}
