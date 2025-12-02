package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
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
	userSelectedTheme := ""
	versionString := ""
	commitSHAString := ""
	externalLinks := config.Frontend.ExternalLinks
	if settings.Env.IsPlaywright {
		versionString = version.Version
		commitSHAString = version.CommitSHA
	}
	if d.user != nil && d.user.Username != "anonymous" {
		theme, ok := config.Frontend.Styling.CustomThemeOptions[d.user.CustomTheme]
		if ok {
			userSelectedTheme = theme.CssRaw
		}
		versionString = version.Version
		commitSHAString = version.CommitSHA
	} else if !settings.Env.IsPlaywright {
		newExternalLinks := []settings.ExternalLink{}
		// remove version and commit SHA from external links
		for _, link := range externalLinks {
			if link.Title == version.CommitSHA {
				continue
			}
			newExternalLinks = append(newExternalLinks, link)
		}
		externalLinks = newExternalLinks
	}

	defaultThemeColor := "#455a64"
	staticURL := config.Server.BaseURL + "public/static"
	description := config.Frontend.Description
	title := config.Frontend.Name

	// Use custom favicon if configured and validated, otherwise fall back to default
	favicon := staticURL + "/favicon"
	data := make(map[string]interface{})
	disableNavButtons := settings.Config.Frontend.DisableNavButtons
	if d.share != nil && d.shareValid {
		if d.share.Favicon != "" {
			if strings.HasPrefix(d.share.Favicon, "http") {
				favicon = d.share.Favicon
			} else {
				favicon = staticURL + "/" + d.share.Favicon
			}
		}
		if d.share.Description != "" {
			description = d.share.Description
		}
		if d.share.Title != "" {
			title = d.share.Title
		}
		if d.share.ShareTheme != "" {
			theme, ok := config.Frontend.Styling.CustomThemeOptions[d.share.ShareTheme]
			if ok {
				userSelectedTheme = theme.CssRaw
			}
		}
		if d.share.ShareTheme != "" {
			theme, ok := config.Frontend.Styling.CustomThemeOptions[d.share.ShareTheme]
			if ok {
				userSelectedTheme = theme.CssRaw
			}
		}
	}
	// Set login icon URL
	loginIcon := staticURL + "/loginIcon"
	data["htmlVars"] = map[string]interface{}{
		"title":             title,
		"customCSS":         config.Frontend.Styling.CustomCSSRaw,
		"userSelectedTheme": userSelectedTheme,
		"lightBackground":   config.Frontend.Styling.LightBackground,
		"darkBackground":    config.Frontend.Styling.DarkBackground,
		"staticURL":         staticURL,
		"baseURL":           config.Server.BaseURL,
		"favicon":           favicon,
		"loginIcon":         loginIcon,
		"color":             defaultThemeColor,
		"winIcon":           staticURL + "/img/icons/mstile-144x144.png",
		"appIcon":           staticURL + "/img/icons/android-chrome-256x256.png",
		"description":       description,
	}
	// variables consumed by frontend as json
	data["globalVars"] = map[string]interface{}{
		"name":                 config.Frontend.Name,
		"minSearchLength":      config.Server.MinSearchLength,
		"disableExternal":      config.Frontend.DisableDefaultLinks,
		"darkMode":             settings.Config.UserDefaults.DarkMode,
		"baseURL":              config.Server.BaseURL,
		"version":              versionString,
		"commitSHA":            commitSHAString,
		"signup":               settings.Config.Auth.Methods.PasswordAuth.Signup,
		"noAuth":               config.Auth.Methods.NoAuth,
		"enableThumbs":         !config.Server.DisablePreviews,
		"externalLinks":        externalLinks,
		"externalUrl":          strings.TrimSuffix(config.Server.ExternalUrl, "/"),
		"onlyOfficeUrl":        settings.Config.Integrations.OnlyOffice.Url,
		"oidcAvailable":        config.Auth.Methods.OidcAuth.Enabled,
		"proxyAvailable":       config.Auth.Methods.ProxyAuth.Enabled,
		"passwordAvailable":    config.Auth.Methods.PasswordAuth.Enabled,
		"mediaAvailable":       settings.MediaEnabled(),
		"muPdfAvailable":       settings.Env.MuPdfAvailable,
		"updateAvailable":      utils.GetUpdateAvailableUrl(),
		"disableNavButtons":    disableNavButtons,
		"userSelectableThemes": config.Frontend.Styling.CustomThemeOptions,
		"enableHeicConversion": config.Integrations.Media.Convert.ImagePreview[settings.HEICImagePreview] && settings.MediaEnabled(),
		"eventBasedThemes":     !config.Frontend.Styling.DisableEventBasedThemes,
		"loginIcon":            loginIcon,
	}

	// Marshal each variable to JSON strings for direct template usage
	globalVarsJSON, err := json.Marshal(data["globalVars"])
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error marshaling globalVars: %w", err)
	}

	// Replace with JSON strings for direct template usage
	data["globalVars"] = string(globalVarsJSON)

	// Render the template with global variables
	if err := templateRenderer.Render(w, file, data); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// setContentType sets the appropriate Content-Type header based on file extension
func setContentType(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".webmanifest":
		w.Header().Set("Content-Type", "application/manifest+json")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	}
}

// staticAssetHandler serves static assets exactly as the frontend build produces them
func staticAssetHandler(w http.ResponseWriter, r *http.Request) {
	const maxAge = 86400 // 1 day
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))
	w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)

	// Strip baseURL and /static/ prefix to get clean asset path
	path := r.URL.Path
	path = strings.TrimPrefix(path, config.Server.BaseURL) // Remove /testing/ or other baseURL
	path = strings.TrimPrefix(path, "/")                   // Remove leading slash FIRST
	path = strings.TrimPrefix(path, "static/")             // Then remove static/ prefix

	// Handle special routes that need path mapping
	var assetPath string
	switch path {
	case "favicon.svg", "favicon":
		// Handle custom favicon from filesystem
		if settings.Env.FaviconIsCustom {
			http.ServeFile(w, r, settings.Env.FaviconPath)
			return
		}
		// Use embedded default
		assetPath = settings.Env.FaviconEmbeddedPath
	case "manifest.json":
		assetPath = "img/icons/manifest.json"
	case "site.webmanifest":
		assetPath = "img/icons/site.webmanifest"
	case "loginIcon":
		// Handle custom login icon from filesystem
		if settings.Env.LoginIconIsCustom {
			http.ServeFile(w, r, settings.Env.LoginIconPath)
			return
		}
		// Use embedded default
		assetPath = settings.Env.LoginIconEmbeddedPath
	default:
		assetPath = path
	}

	// Try gzipped version first for files that may be compressed
	var fileContents []byte
	var err error

	// Check if gzipped version exists
	if strings.HasSuffix(assetPath, ".js") || strings.HasSuffix(assetPath, ".woff2") {
		gzPath := assetPath + ".gz"
		fileContents, err = fs.ReadFile(assetFs, gzPath)
		if err == nil {
			// Gzipped version exists, serve it
			setContentType(w, assetPath)
			w.Header().Set("Content-Encoding", "gzip")
			_, err = w.Write(fileContents)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}

	// Serve uncompressed version
	fileContents, err = fs.ReadFile(assetFs, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	setContentType(w, assetPath)
	_, err = w.Write(fileContents)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if r.Method != http.MethodGet {
		return http.StatusNotFound, nil
	}
	return handleWithStaticData(w, r, d, "index.html", "text/html")
}
