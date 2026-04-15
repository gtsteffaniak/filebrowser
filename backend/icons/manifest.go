package icons

import (
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
	if name == "FileBrowser Quantum" {
		shortName = "FBQ"
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

// InitializePWAManifest caches the PWA manifest at startup. Icon URLs always point at
// PNGs under public/static/icons/, which GeneratePWAIcons produces from the configured
// favicon (custom SVG uses a raster sidecar as the raster source).
func InitializePWAManifest() {
	config := &settings.Config
	staticURL := config.Server.BaseURL + "public/static"
	title := config.Frontend.Name
	description := config.Frontend.Description
	defaultThemeColor := "#455a64"

	pwaIcon192 := staticURL + "/icons/pwa-icon-192.png"
	pwaIcon256 := staticURL + "/icons/pwa-icon-256.png"
	pwaIcon512 := staticURL + "/icons/pwa-icon-512.png"

	CachedManifest = generatePWAManifest(title, description, config.Server.BaseURL, defaultThemeColor, pwaIcon192, pwaIcon256, pwaIcon512)
}
