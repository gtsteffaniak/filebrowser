package icons

import (
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

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

// CachedManifest holds the generated PWA manifest at startup
var CachedManifest PWAManifest

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
	config := &settings.Config
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
		pwaIcon192 = staticURL + "/icons/pwa-icon-192.png"
		pwaIcon256 = staticURL + "/icons/pwa-icon-256.png"
		pwaIcon512 = staticURL + "/icons/pwa-icon-512.png"
	}

	CachedManifest = generatePWAManifest(title, description, config.Server.BaseURL, defaultThemeColor, pwaIcon192, pwaIcon256, pwaIcon512)
}
