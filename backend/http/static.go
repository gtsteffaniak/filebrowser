package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
)

var templateRenderer *TemplateRenderer
var cachedManifest PWAManifest // generated at startup

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
	banner := staticURL + "/pwa-icon-512.png" // Use largest generated icon for best quality
	disableSidebar := false

	// Use custom favicon if configured and validated, otherwise fall back to default
	// Determine the correct favicon extension based on type
	faviconExt := ".svg" // Default to SVG
	if settings.Env.FaviconIsCustom && strings.ToLower(filepath.Ext(settings.Env.FaviconPath)) != ".svg" {
		faviconExt = ".png"
	}
	favicon := staticURL + "/favicon" + faviconExt
	shareHash := ""
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
		if d.share.DisableSidebar {
			disableSidebar = true
		}
		if strings.HasPrefix(d.share.Banner, "http") {
			banner = d.share.Banner
		} else {
			_, _, err := getShareImagePartsHelper(d.share, true)
			if err == nil {
				banner = fmt.Sprintf("%s%spublic/api/share/image?banner=true&hash=%s", config.Server.ExternalUrl, config.Server.BaseURL, d.share.Hash)
			}
		}
		if strings.HasPrefix(d.share.Favicon, "http") {
			favicon = d.share.Favicon
		} else {
			_, _, err := getShareImagePartsHelper(d.share, false)
			if err == nil {
				favicon = fmt.Sprintf("%s%spublic/api/share/image?favicon=true&hash=%s", config.Server.ExternalUrl, config.Server.BaseURL, d.share.Hash)
			}
		}
		shareHash = d.share.Hash
	}
	// Set login icon URL
	loginIcon := staticURL + "/loginIcon"

	// Load loading spinners CSS from static files
	loadingSpinnersCSS := ""
	cssPath := "css/loadingSpinners.css"
	cssContent, err := fs.ReadFile(assetFs, cssPath)
	if err == nil {
		loadingSpinnersCSS = string(cssContent)
	}

	// Determine OpenGraph image: use banner if set, otherwise use largest available icon (512x512)
	ogImage := banner
	if banner == staticURL+"/pwa-icon-512.png" {
		// Note: 512x512 is square; OpenGraph prefers 1200x630 (1.91:1 ratio) but square works fine
		ogImage = staticURL + "/pwa-icon-512.png"
	}

	// Construct the full URL for the current request
	var fullURL string
	if config.Server.ExternalUrl != "" {
		// ExternalUrl already includes schema (e.g., http://mydomain.com)
		fullURL = strings.TrimSuffix(config.Server.ExternalUrl, "/") + r.URL.Path
	} else {
		// Build URL from request
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		fullURL = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.Path)
	}

	// Determine PWA icon URLs based on custom favicon settings
	pwaIcon192 := staticURL + "/" + settings.Env.PWAIcon192
	pwaIcon256 := staticURL + "/" + settings.Env.PWAIcon256
	pwaIcon512 := staticURL + "/" + settings.Env.PWAIcon512

	// If custom favicon is set and it's SVG, use the favicon directly
	if settings.Env.FaviconIsCustom && strings.ToLower(filepath.Ext(settings.Env.FaviconPath)) == ".svg" {
		pwaIcon192 = favicon
		pwaIcon256 = favicon
		pwaIcon512 = favicon
	} else if settings.Env.FaviconIsCustom {
		// For custom PNG/ICO favicons, use generated PWA icons
		pwaIcon192 = staticURL + "/pwa-icon-192.png"
		pwaIcon256 = staticURL + "/pwa-icon-256.png"
		pwaIcon512 = staticURL + "/pwa-icon-512.png"
	}

	data["htmlVars"] = map[string]interface{}{
		"title":              title,
		"customCSS":          config.Frontend.Styling.CustomCSSRaw,
		"userSelectedTheme":  userSelectedTheme,
		"lightBackground":    config.Frontend.Styling.LightBackground,
		"darkBackground":     config.Frontend.Styling.DarkBackground,
		"staticURL":          staticURL,
		"baseURL":            config.Server.BaseURL,
		"favicon":            favicon,
		"loginIcon":          loginIcon,
		"color":              defaultThemeColor,
		"winIcon":            staticURL + "/mstile-256x256.png",
		"appIcon":            staticURL + "/apple-touch-icon.png",
		"description":        description,
		"loadingSpinnersCSS": loadingSpinnersCSS,
		"banner":             banner,
		"image":              ogImage,
		"url":                fullURL,
		"pwaIcon192":         pwaIcon192,
		"pwaIcon256":         pwaIcon256,
		"pwaIcon512":         pwaIcon512,
		"manifestURL":        staticURL + "/site.webmanifest",
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
		"enableHeicConversion": settings.CanConvertImage("heic"),
		"eventBasedThemes":     !config.Frontend.Styling.DisableEventBasedThemes,
		"loginIcon":            loginIcon,
		"disableSidebar":       disableSidebar,
		"shareHash":            shareHash,
		"oidcLoginButtonText":  config.Frontend.OIDCLoginButtonText,
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

// manifestHandler serves the cached PWA manifest
func manifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	if err := json.NewEncoder(w).Encode(cachedManifest); err != nil {
		http.Error(w, "Failed to serve manifest", http.StatusInternalServerError)
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
	fmt.Println("asset path", path)
	switch path {
	case "site.webmanifest":
		manifestHandler(w, r)
		return
	case "favicon.svg":
		// For custom SVG favicons, serve the SVG directly
		if settings.Env.FaviconIsCustom && strings.ToLower(filepath.Ext(settings.Env.FaviconPath)) == ".svg" {
			http.ServeFile(w, r, settings.Env.FaviconPath)
			return
		}
		// Use embedded default SVG
		assetPath = settings.Env.FaviconEmbeddedPath
	case "favicon.png":
		if settings.Env.FaviconIsCustom && strings.ToLower(filepath.Ext(settings.Env.FaviconPath)) != ".svg" {
			iconPath := filepath.Join(settings.Env.PWAIconsDir, "pwa-icon-512.png")
			if _, err := os.Stat(iconPath); err == nil {
				http.ServeFile(w, r, iconPath)
				return
			}
		}
		// Fall back to embedded default favicon.png
		assetPath = "img/icons/favicon.png"
	case "favicon-32x32.png",
		"pwa-icon-192.png", "pwa-icon-256.png", "pwa-icon-512.png",
		"apple-touch-icon.png":
		// Serve generated icons from cache directory
		iconPath := filepath.Join(settings.Env.PWAIconsDir, path)
		if _, err := os.Stat(iconPath); err == nil {
			fmt.Println(iconPath)
			http.ServeFile(w, r, iconPath)
			return
		}
		// Fall back to embedded favicon.png if generation failed
		assetPath = "img/icons/favicon.png"
	case "mstile-256x256.png":
		// Windows tile - redirect to pwa-icon-256.png (they're identical)
		iconPath := filepath.Join(settings.Env.PWAIconsDir, "pwa-icon-256.png")
		if _, err := os.Stat(iconPath); err == nil {
			http.ServeFile(w, r, iconPath)
			return
		}
		// Fall back to embedded favicon.png if generation failed
		assetPath = "img/icons/favicon.png"
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

// PWAManifest represents the web app manifest structure
type PWAManifest struct {
	Name            string    `json:"name"`
	ShortName       string    `json:"short_name"`
	Icons           []PWAIcon `json:"icons"`
	StartURL        string    `json:"start_url"`
	Display         string    `json:"display"`
	BackgroundColor string    `json:"background_color"`
	ThemeColor      string    `json:"theme_color"`
	Description     string    `json:"description"`
}

// PWAIcon represents an icon in the web app manifest
type PWAIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

// generatePWAManifest creates the PWA manifest structure
func generatePWAManifest(name, description, baseURL, themeColor, pwaIcon192, pwaIcon256, pwaIcon512 string) PWAManifest {
	shortName := name
	if len(name) > 12 {
		shortName = name[:12]
	}

	return PWAManifest{
		Name:            name,
		ShortName:       shortName,
		StartURL:        baseURL,
		Display:         "standalone",
		BackgroundColor: "#ffffff",
		ThemeColor:      themeColor,
		Description:     description,
		Icons: []PWAIcon{
			{
				Src:     pwaIcon192,
				Sizes:   "192x192",
				Type:    "image/png",
				Purpose: "any",
			},
			{
				Src:     pwaIcon256,
				Sizes:   "256x256",
				Type:    "image/png",
				Purpose: "any maskable",
			},
			{
				Src:     pwaIcon512,
				Sizes:   "512x512",
				Type:    "image/png",
				Purpose: "any",
			},
		},
	}
}

// InitializePWAManifest generates and caches the PWA manifest at startup
func InitializePWAManifest() {
	staticURL := config.Server.BaseURL + "public/static"
	title := config.Frontend.Name
	description := config.Frontend.Description
	defaultThemeColor := "#455a64"

	// Determine PWA icon URLs based on custom favicon settings
	pwaIcon192 := staticURL + "/" + settings.Env.PWAIcon192
	pwaIcon256 := staticURL + "/" + settings.Env.PWAIcon256
	pwaIcon512 := staticURL + "/" + settings.Env.PWAIcon512

	if settings.Env.FaviconIsCustom && strings.ToLower(filepath.Ext(settings.Env.FaviconPath)) == ".svg" {
		favicon := staticURL + "/favicon"
		pwaIcon192 = favicon
		pwaIcon256 = favicon
		pwaIcon512 = favicon
	} else if settings.Env.FaviconIsCustom {
		pwaIcon192 = staticURL + "/pwa-icon-192.png"
		pwaIcon256 = staticURL + "/pwa-icon-256.png"
		pwaIcon512 = staticURL + "/pwa-icon-512.png"
	}

	cachedManifest = generatePWAManifest(title, description, config.Server.BaseURL, defaultThemeColor, pwaIcon192, pwaIcon256, pwaIcon512)
}
