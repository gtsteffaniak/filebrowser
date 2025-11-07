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
			shareProps["singleFileShare"] = d.share.IsSingleFileShare()
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
			shareProps["isPasswordProtected"] = d.share.HasPassword()
			shareProps["downloadURL"] = getDownloadURL(r, d.share.Hash)
			shareProps["enforceDarkLightMode"] = d.share.EnforceDarkLightMode
			shareProps["viewMode"] = d.share.ViewMode
			shareProps["enableOnlyOffice"] = d.share.EnableOnlyOffice
			shareProps["shareType"] = utils.Ternary(d.share.ShareType == "", "normal", d.share.ShareType)
			shareProps["perUserDownloadLimit"] = d.share.PerUserDownloadLimit
			shareProps["extractEmbeddedSubtitles"] = d.share.ExtractEmbeddedSubtitles
			shareProps["disableDownload"] = d.share.DisableDownload
			shareProps["allowCreate"] = d.share.AllowCreate
			shareProps["allowModify"] = d.share.AllowModify
			shareProps["allowDelete"] = d.share.AllowDelete
			shareProps["allowReplacements"] = d.share.AllowReplacements
			shareProps["downloadsLimit"] = d.share.DownloadsLimit
			shareProps["shareTheme"] = d.share.ShareTheme
			shareProps["disableAnonymous"] = d.share.DisableAnonymous
			shareProps["maxBandwidth"] = d.share.MaxBandwidth
			shareProps["keepAfterExpiration"] = d.share.KeepAfterExpiration
			shareProps["allowedUsernames"] = d.share.AllowedUsernames
			shareProps["hideNavButtons"] = d.share.HideNavButtons

			// Additional computed properties from extended.go
			shareProps["isPermanent"] = d.share.IsPermanent()
			shareProps["fileExtension"] = d.share.GetFileExtension()
			shareProps["fileName"] = d.share.GetFileName()
			if d.share.Favicon != "" {
				if strings.HasPrefix(d.share.Favicon, "http") {
					data["favicon"] = d.share.Favicon
				} else {
					data["favicon"] = staticURL + "/" + d.share.Favicon
				}
			}
			if d.share.Description != "" {
				data["description"] = d.share.Description
			}
			if d.share.Title != "" {
				data["title"] = d.share.Title
			}
			if d.share.ShareTheme != "" {
				theme, ok := config.Frontend.Styling.CustomThemeOptions[d.share.ShareTheme]
				if ok {
					userSelectedTheme = theme.CssRaw
				}
			}
		}

		// base url could be different for routes behind proxy
		data["staticURL"] = staticURL
		// Use custom favicon for shares too if configured
		if config.Frontend.Favicon != "" {
			data["favicon"] = staticURL + "/favicon"
		} else {
			data["favicon"] = staticURL + "/img/icons/favicon-256x256.png"
		}
		data["winIcon"] = staticURL + "/img/icons/mstile-144x144.png"
		data["appIcon"] = staticURL + "/img/icons/android-chrome-256x256.png"
	}
	// Set login icon URL
	loginIcon := staticURL + "/loginIcon"

	data["htmlVars"] = map[string]interface{}{
		"title":             config.Frontend.Name,
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
		"description":       config.Frontend.Description,
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
		"sourceCount":          len(config.Server.SourceMap),
		"oidcAvailable":        config.Auth.Methods.OidcAuth.Enabled,
		"proxyAvailable":       config.Auth.Methods.ProxyAuth.Enabled,
		"passwordAvailable":    config.Auth.Methods.PasswordAuth.Enabled,
		"mediaAvailable":       config.Integrations.Media.FfmpegPath != "",
		"muPdfAvailable":       settings.Env.MuPdfAvailable,
		"updateAvailable":      utils.GetUpdateAvailableUrl(),
		"disableNavButtons":    disableNavButtons,
		"userSelectableThemes": config.Frontend.Styling.CustomThemeOptions,
		"enableHeicConversion": config.Integrations.Media.Convert.ImagePreview[settings.HEICImagePreview],
		"eventBasedThemes":     !config.Frontend.Styling.DisableEventBasedThemes,
		"loginIcon":            loginIcon,
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

func loginIconHandler(w http.ResponseWriter, r *http.Request) {
	const maxAge = 86400 // 1 day
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))

	// Handle custom login icon if configured
	if config.Frontend.LoginIcon != "" {
		// Check if it's a file path (not embedded)
		if _, err := fs.Stat(assetFs, config.Frontend.LoginIcon); err == nil {
			// Serve from embedded/dist filesystem
			fileContents, err := fs.ReadFile(assetFs, config.Frontend.LoginIcon)
			if err == nil {
				// Set content type based on file extension
				lowerPath := strings.ToLower(config.Frontend.LoginIcon)
				if strings.HasSuffix(lowerPath, ".svg") {
					w.Header().Set("Content-Type", "image/svg+xml")
				} else if strings.HasSuffix(lowerPath, ".png") {
					w.Header().Set("Content-Type", "image/png")
				} else if strings.HasSuffix(lowerPath, ".jpg") || strings.HasSuffix(lowerPath, ".jpeg") {
					w.Header().Set("Content-Type", "image/jpeg")
				} else if strings.HasSuffix(lowerPath, ".gif") {
					w.Header().Set("Content-Type", "image/gif")
				} else if strings.HasSuffix(lowerPath, ".webp") {
					w.Header().Set("Content-Type", "image/webp")
				} else if strings.HasSuffix(lowerPath, ".ico") {
					w.Header().Set("Content-Type", "image/x-icon")
				}
				_, err = w.Write(fileContents)
				if err != nil {
					http.NotFound(w, r)
				}
				return
			}
		}
	}

	// Fallback to default favicon icon if custom login icon not found
	// Try both paths to support embedded and dev modes
	defaultIconPaths := []string{
		"img/icons/favicon-256x256.png",        // Dev mode path
		"public/img/icons/favicon-256x256.png", // Embedded mode path
	}

	for _, defaultIconPath := range defaultIconPaths {
		fileContents, err := fs.ReadFile(assetFs, defaultIconPath)
		if err == nil {
			w.Header().Set("Content-Type", "image/png")
			_, err = w.Write(fileContents)
			if err != nil {
				http.NotFound(w, r)
			}
			return
		}
	}

	// If even default icon fails, return 404
	http.NotFound(w, r)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	const maxAge = 86400 // 1 day
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))

	// Try to serve custom favicon if configured
	if config.Frontend.Favicon != "" {
		http.ServeFile(w, r, config.Frontend.Favicon)
		return
	}

	// Serve default favicon.ico using path set at startup
	iconPath := assetPathPrefix + "favicon.ico"
	fileContents, err := fs.ReadFile(assetFs, iconPath)
	if err == nil {
		w.Header().Set("Content-Type", "image/x-icon")
		_, err = w.Write(fileContents)
		if err != nil {
			http.NotFound(w, r)
		}
		return
	}

	http.NotFound(w, r)
}

func manifestHandler(w http.ResponseWriter, r *http.Request) {
	const maxAge = 86400 // 1 day
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))
	w.Header().Set("Content-Type", "application/manifest+json")

	// Read manifest using path set at startup
	manifestPath := assetPathPrefix + "site.webmanifest"
	fileContents, err := fs.ReadFile(assetFs, manifestPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Replace relative paths with baseURL-aware paths
	manifestStr := string(fileContents)
	baseURL := config.Server.BaseURL
	staticURL := baseURL + "public/static/"

	// Update the manifest to use the correct baseURL
	manifestStr = strings.ReplaceAll(manifestStr, `"start_url": "/"`, fmt.Sprintf(`"start_url": "%s"`, baseURL))
	manifestStr = strings.ReplaceAll(manifestStr, `"src": "img/icons/`, fmt.Sprintf(`"src": "%simg/icons/`, staticURL))

	_, err = w.Write([]byte(manifestStr))
	if err != nil {
		http.NotFound(w, r)
	}
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

	// Handle site.webmanifest with proper content type
	if strings.HasSuffix(r.URL.Path, "site.webmanifest") || strings.HasSuffix(r.URL.Path, "manifest.json") {
		w.Header().Set("Content-Type", "application/manifest+json")
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
