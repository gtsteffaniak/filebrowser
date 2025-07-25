package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
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

func handleWithStaticData(w http.ResponseWriter, r *http.Request, d *requestContext, file, contentType string) (int, error) {
	w.Header().Set("Content-Type", contentType)

	auther, err := store.Auth.Get("password")
	if err != nil {
		return http.StatusInternalServerError, err
	}
	customCSS := config.Frontend.Styling.CustomCSS

	if d.user != nil {
		theme, ok := config.Frontend.Styling.CustomThemes[d.user.CustomTheme]
		if ok {
			customCSS = theme.CSS
		}
	}

	data := map[string]interface{}{
		"CustomCSS":       customCSS,
		"LightBackground": config.Frontend.Styling.LightBackground,
		"DarkBackground":  config.Frontend.Styling.DarkBackground,
		"StaticURL":       config.Server.BaseURL + "static",
		"BaseURL":         config.Server.BaseURL,
	}
	// variables consumed by frontend as json
	data["globalVars"] = map[string]interface{}{
		"Name":                 config.Frontend.Name,
		"DisableExternal":      config.Frontend.DisableDefaultLinks,
		"darkMode":             settings.Config.UserDefaults.DarkMode,
		"BaseURL":              config.Server.BaseURL,
		"Version":              version.Version,
		"CommitSHA":            version.CommitSHA,
		"Signup":               settings.Config.Auth.Methods.PasswordAuth.Signup,
		"NoAuth":               config.Auth.Methods.NoAuth,
		"LoginPage":            auther.LoginPage(),
		"EnableThumbs":         !config.Server.DisablePreviews,
		"ExternalLinks":        config.Frontend.ExternalLinks,
		"ExternalUrl":          strings.TrimSuffix(config.Server.ExternalUrl, "/"),
		"OnlyOfficeUrl":        settings.Config.Integrations.OnlyOffice.Url,
		"SourceCount":          len(config.Server.SourceMap),
		"OidcAvailable":        config.Auth.Methods.OidcAuth.Enabled,
		"PasswordAvailable":    config.Auth.Methods.PasswordAuth.Enabled,
		"MediaAvailable":       config.Integrations.Media.FfmpegPath != "",
		"MuPdfAvailable":       config.Server.MuPdfAvailable,
		"UpdateAvailable":      utils.GetUpdateAvailableUrl(),
		"DisableNavButtons":    settings.Config.Frontend.DisableNavButtons,
		"UserSelectableThemes": config.Frontend.Styling.CustomThemeOptions,
	}
	jsonVars, err := json.Marshal(data["globalVars"])
	if err != nil {
		return http.StatusInternalServerError, err
	}
	data["globalVars"] = strings.ReplaceAll(string(jsonVars), `'`, `\'`)

	// Render the template with global variables
	if err := templateRenderer.Render(w, file, data); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
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

func indexHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if r.Method != http.MethodGet {
		return http.StatusNotFound, nil
	}
	return handleWithStaticData(w, r, d, "index.html", "text/html")
}
