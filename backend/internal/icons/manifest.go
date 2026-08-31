package icons

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// PWAManifest represents the web app manifest structure
type PWAManifest struct {
	Name            string    `json:"name"`
	ID              string    `json:"id"`
	Scope           string    `json:"scope"`
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

const pwaManifestNameMaxLen = 30

// pwaManifestName returns the app title for the web app manifest.
// Names longer than 30 characters are truncated to fit common mobile launcher limits.
func pwaManifestName(name string) string {
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) > pwaManifestNameMaxLen {
		return string(runes[:pwaManifestNameMaxLen])
	}
	return name
}

// generatePWAManifest creates the PWA manifest structure
func generatePWAManifest(name, description, baseURL, themeColor, backgroundColor, pwaIcon192, pwaIcon256, pwaIcon512, pwaIconMaskable192, pwaIconMaskable512 string) PWAManifest {
	return PWAManifest{
		Name:            pwaManifestName(name),
		ID:              baseURL,
		Scope:           baseURL,
		StartURL:        baseURL,
		Display:         "standalone",
		BackgroundColor: backgroundColor,
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
				Purpose: "any",
			},
			{
				Src:     pwaIcon512,
				Sizes:   "512x512",
				Type:    "image/png",
				Purpose: "any",
			},
			{
				Src:     pwaIconMaskable192,
				Sizes:   "192x192",
				Type:    "image/png",
				Purpose: "maskable",
			},
			{
				Src:     pwaIconMaskable512,
				Sizes:   "512x512",
				Type:    "image/png",
				Purpose: "maskable",
			},
		},
	}
}

// InitializePWAManifest caches the PWA manifest at startup. Icon URLs always point at
// PNGs under public/static/icons/, which GeneratePWAIcons produces from the configured
// favicon (custom SVG uses a raster sidecar as the raster source).
func InitializePWAManifest() {
	config := &settings.Config
	staticURL := config.Http.BaseURL + "public/static"
	title := config.Frontend.Name
	description := config.Frontend.Description
	pwaIcon192 := staticURL + "/icons/pwa-icon-192.png"
	pwaIcon256 := staticURL + "/icons/pwa-icon-256.png"
	pwaIcon512 := staticURL + "/icons/pwa-icon-512.png"
	pwaIconMaskable192 := staticURL + "/icons/pwa-icon-maskable-192.png"
	pwaIconMaskable512 := staticURL + "/icons/pwa-icon-maskable-512.png"

	backgroundColor := settings.DefaultBackgroundColor()

	CachedManifest = generatePWAManifest(title, description, config.Http.BaseURL, backgroundColor, backgroundColor, pwaIcon192, pwaIcon256, pwaIcon512, pwaIconMaskable192, pwaIconMaskable512)
}
