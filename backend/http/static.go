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
	devMode   bool
}

// Render renders a template document with headers and data
func (t *TemplateRenderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	// If in dev mode, reload templates on every render.
	if t.devMode {
		var err error
		t.templates, err = t.templates.ParseFS(assetFs, "public/index.html")
		if err != nil {
			return fmt.Errorf("error reloading template: %w", err)
		}
	}
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
	userSelectedTheme := ""
	if d.user != nil {
		theme, ok := config.Frontend.Styling.CustomThemeOptions[d.user.CustomTheme]
		if ok {
			userSelectedTheme = theme.CssRaw
		}
	}

	defaultThemeColor := "#455a64"
	staticURL := config.Server.BaseURL + "static"
	publicStaticURL := config.Server.BaseURL + "public/static"

	// Use custom favicon if configured and validated, otherwise fall back to default
	var favicon string
	if config.Frontend.Favicon != "" {
		favicon = staticURL + "/favicon"
	} else {
		// Default favicon
		favicon = staticURL + "/img/icons/favicon-256x256.png"
	}
	data := make(map[string]interface{})
	shareProps := map[string]interface{}{
		"isShare":             false,
		"isValid":             false,
		"banner":              "",
		"title":               "",
		"quickDownload":       false,
		"description":         "",
		"themeColor":          "",
		"disableThumbnails":   false,
		"viewMode":            "list",
		"disableFileViewer":   false,
		"disableShareCard":    false,
		"disableSidebar":      false,
		"isPasswordProtected": false,
		"hash":                "",
	}
	disableNavButtons := settings.Config.Frontend.DisableNavButtons
	if d.share != nil {
		shareProps["isShare"] = true
		shareProps["isValid"] = d.shareValid
		shareProps["hash"] = d.share.Hash

		if d.shareValid {
			disableNavButtons = disableNavButtons || d.share.HideNavButtons
			shareProps["viewMode"] = d.share.ViewMode
			shareProps["banner"] = d.share.Banner
			shareProps["title"] = d.share.Title
			shareProps["description"] = d.share.Description
			shareProps["themeColor"] = d.share.ThemeColor
			shareProps["quickDownload"] = d.share.QuickDownload
			shareProps["disableThumbnails"] = d.share.DisableThumbnails
			shareProps["disableFileViewer"] = d.share.DisableFileViewer
			shareProps["disableShareCard"] = d.share.DisableShareCard
			shareProps["disableSidebar"] = d.share.DisableSidebar
			shareProps["isPasswordProtected"] = d.share.PasswordHash != ""
			shareProps["downloadURL"] = getDownloadURL(r, d.share.Hash)
			shareProps["enforceDarkLightMode"] = d.share.EnforceDarkLightMode
			shareProps["enableOnlyOffice"] = d.share.EnableOnlyOffice
			shareProps["enableOnlyOfficeEditing"] = d.share.EnableOnlyOfficeEditing
			if d.share.Favicon != "" {
				if strings.HasPrefix(d.share.Favicon, "http") {
					data["favicon"] = d.share.Favicon
				} else {
					data["favicon"] = publicStaticURL + "/" + d.share.Favicon
				}
			}
			if d.share.Description != "" {
				data["description"] = d.share.Description
			}
			if d.share.Title != "" {
				data["title"] = d.share.Title
			}
		}

		// base url could be different for routes behind proxy
		data["staticURL"] = publicStaticURL
		// Use custom favicon for shares too if configured
		if config.Frontend.Favicon != "" {
			data["favicon"] = publicStaticURL + "/favicon"
		} else {
			data["favicon"] = publicStaticURL + "/img/icons/favicon-256x256.png"
		}
		data["winIcon"] = publicStaticURL + "/img/icons/mstile-144x144.png"
		data["appIcon"] = publicStaticURL + "/img/icons/android-chrome-256x256.png"
	}
	data["htmlVars"] = map[string]interface{}{
		"title":             config.Frontend.Name,
		"customCSS":         config.Frontend.Styling.CustomCSSRaw,
		"userSelectedTheme": userSelectedTheme,
		"lightBackground":   config.Frontend.Styling.LightBackground,
		"darkBackground":    config.Frontend.Styling.DarkBackground,
		"staticURL":         staticURL,
		"baseURL":           config.Server.BaseURL,
		"favicon":           favicon,
		"color":             defaultThemeColor,
		"winIcon":           staticURL + "/img/icons/mstile-144x144.png",
		"appIcon":           staticURL + "/img/icons/android-chrome-256x256.png",
		"description":       config.Frontend.Description,
	}
	// variables consumed by frontend as json
	data["globalVars"] = map[string]interface{}{
		"name":                 config.Frontend.Name,
		"minSearchLength":      config.Server.MinSearchLength,
		"disableExternal":      config.Frontend.DisableDefaultLinks,
		"darkMode":             settings.Config.UserDefaults.DarkMode,
		"baseURL":              config.Server.BaseURL,
		"version":              version.Version,
		"commitSHA":            version.CommitSHA,
		"signup":               settings.Config.Auth.Methods.PasswordAuth.Signup,
		"noAuth":               config.Auth.Methods.NoAuth,
		"loginPage":            auther.LoginPage(),
		"enableThumbs":         !config.Server.DisablePreviews,
		"externalLinks":        config.Frontend.ExternalLinks,
		"externalUrl":          strings.TrimSuffix(config.Server.ExternalUrl, "/"),
		"onlyOfficeUrl":        settings.Config.Integrations.OnlyOffice.Url,
		"sourceCount":          len(config.Server.SourceMap),
		"oidcAvailable":        config.Auth.Methods.OidcAuth.Enabled,
		"passwordAvailable":    config.Auth.Methods.PasswordAuth.Enabled,
		"mediaAvailable":       config.Integrations.Media.FfmpegPath != "",
		"muPdfAvailable":       config.Server.MuPdfAvailable,
		"updateAvailable":      utils.GetUpdateAvailableUrl(),
		"disableNavButtons":    disableNavButtons,
		"userSelectableThemes": config.Frontend.Styling.CustomThemeOptions,
		"enableHeicConversion": config.Integrations.Media.Convert.ImagePreview[settings.HEICImagePreview],
	}

	// Marshal each variable to JSON strings for direct template usage
	globalVarsJSON, err := json.Marshal(data["globalVars"])
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error marshaling globalVars: %w", err)
	}
	shareVarsJSON, err := json.Marshal(shareProps)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error marshaling shareVars: %w", err)
	}

	// Replace with JSON strings for direct template usage
	data["globalVars"] = string(globalVarsJSON)
	data["shareVars"] = string(shareVarsJSON)

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

	// Handle custom favicon if configured and requested
	if r.URL.Path == "favicon" && config.Frontend.Favicon != "" {
		http.ServeFile(w, r, config.Frontend.Favicon)
		return
	}

	adjustedCompressed := r.URL.Path + ".gz"
	if strings.HasSuffix(r.URL.Path, ".js") {
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
		http.FileServer(http.FS(assetFs)).ServeHTTP(w, r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if r.Method != http.MethodGet {
		return http.StatusNotFound, nil
	}
	return handleWithStaticData(w, r, d, "index.html", "text/html")
}
