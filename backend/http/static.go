package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/version"
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
		"DisableExternal":       config.Frontend.DisableDefaultLinks,
		"DisableUsedPercentage": config.Frontend.DisableUsedPercentage,
		"darkMode":              settings.Config.UserDefaults.DarkMode,
		"Color":                 config.Frontend.Color,
		"BaseURL":               config.Server.BaseURL,
		"Version":               version.Version,
		"CommitSHA":             version.CommitSHA,
		"StaticURL":             config.Server.BaseURL + "static",
		"Signup":                settings.Config.Auth.Signup,
		"NoAuth":                config.Auth.Method == "noauth",
		"AuthMethod":            config.Auth.Method,
		"LoginPage":             auther.LoginPage(),
		"CSS":                   false,
		"ReCaptcha":             false,
		"EnableThumbs":          config.Server.EnableThumbnails,
		"ResizePreview":         config.Server.ResizePreview,
		"EnableExec":            config.Server.EnableExec,
		"ReCaptchaHost":         config.Auth.Recaptcha.Host,
		"ExternalLinks":         config.Frontend.ExternalLinks,
		"ExternalUrl":           strings.TrimSuffix(config.Server.ExternalUrl, "/"),
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func staticFilesHandler(w http.ResponseWriter, r *http.Request) {
	const maxAge = 86400 // 1 day
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))
	w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
	// Remove "/static/" from the request path
	adjustedPath := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%vstatic/", config.Server.BaseURL))
	adjustedCompressed := adjustedPath + ".gz"
	if strings.HasSuffix(adjustedPath, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8") // Set the correct MIME type for JavaScript files
	}
	// Check if the gzipped version of the file exists
	fileContents, err := fs.ReadFile(assetFs, adjustedCompressed)
	if err == nil {
		w.Header().Set("Content-Encoding", "gzip") // Let the browser know the file is compressed
		status, err := w.Write(fileContents)       // Write the gzipped file content to the response
		if err != nil {
			http.Error(w, http.StatusText(status), status)
		}
	} else {
		// Otherwise, serve the regular file
		http.StripPrefix(fmt.Sprintf("%vstatic/", config.Server.BaseURL), http.FileServer(http.FS(assetFs))).ServeHTTP(w, r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	handleWithStaticData(w, r, "index.html", "text/html")

}
