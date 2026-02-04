package icons

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/kovidgoyal/imaging"
)

// sourceType represents the type of favicon source
type sourceType int

const (
	sourceCustomSVG sourceType = iota
	sourceCustomRaster
	sourceDefault
)

// iconSize defines an icon size to generate
type iconSize struct {
	size int
	name string
	env  *string
}

// determineSourceType determines which type of favicon source we have
func determineSourceType(faviconPath string) sourceType {
	if !settings.Env.FaviconIsCustom {
		return sourceDefault
	}

	ext := strings.ToLower(filepath.Ext(faviconPath))
	switch ext {
	case ".svg":
		return sourceCustomSVG
	default:
		return sourceCustomRaster
	}
}

// loadDefaultFavicon loads the embedded default favicon PNG
func loadDefaultFavicon() ([]byte, error) {
	assetFs := fileutils.GetAssetFS()
	if assetFs == nil {
		return nil, errors.New("asset filesystem not initialized")
	}

	pngPath := "img/icons/favicon.png"
	sourceData, err := fs.ReadFile(assetFs, pngPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read default favicon PNG: %w", err)
	}

	logger.Debug("Loaded default embedded favicon")
	return sourceData, nil
}

// findRasterCompanion looks for a raster companion file for an SVG favicon
// Returns the companion data and path, or error if not found
func findRasterCompanion(svgPath string) ([]byte, string, error) {
	basePath := svgPath[:len(svgPath)-len(filepath.Ext(svgPath))]
	rasterExts := []string{".png", ".jpg", ".jpeg", ".webp", ".gif"}

	for _, ext := range rasterExts {
		companionPath := basePath + ext
		if _, err := os.Stat(companionPath); err == nil {
			data, readErr := os.ReadFile(companionPath)
			if readErr != nil {
				continue
			}
			return data, companionPath, nil
		}
	}

	return nil, "", fmt.Errorf("no raster companion found at %s.{png,jpg,webp,gif}", basePath)
}

// convertRasterToPNG decodes any raster format and re-encodes as PNG
func convertRasterToPNG(rasterData []byte, sourceExt string) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(rasterData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
		return nil, fmt.Errorf("failed to convert to PNG: %w", err)
	}

	logger.Debugf("Converted %s to PNG for icon generation", sourceExt)
	return buf.Bytes(), nil
}

// loadFaviconSource loads and prepares the favicon source data as PNG
// Returns PNG data ready for resizing into icon sizes
func loadFaviconSource() ([]byte, error) {
	sourceType := determineSourceType(settings.Env.FaviconPath)

	switch sourceType {
	case sourceCustomSVG:
		logger.Debugf("Processing custom SVG favicon: %s", settings.Env.FaviconPath)
		return handleSVGFavicon(settings.Env.FaviconPath)

	case sourceCustomRaster:
		logger.Debugf("Processing custom raster favicon: %s", settings.Env.FaviconPath)
		return handleRasterFavicon(settings.Env.FaviconPath)

	case sourceDefault:
		logger.Debug("Using default embedded favicon")
		return loadDefaultFavicon()

	default:
		return nil, errors.New("unknown icon source type")
	}
}

// handleSVGFavicon processes an SVG favicon by finding its raster companion
func handleSVGFavicon(svgPath string) ([]byte, error) {
	logger.Debug("Looking for raster companion for SVG favicon")

	companionData, companionPath, err := findRasterCompanion(svgPath)
	if err != nil {
		logger.Warningf("SVG favicon has no raster companion: %v", err)
		logger.Warning("Falling back to default embedded favicon")
		logger.Warning("For best Apple/iOS compatibility, provide a PNG/JPG companion alongside your SVG")
		return loadDefaultFavicon()
	}

	logger.Debugf("Found raster companion: %s", companionPath)
	return convertRasterToPNG(companionData, filepath.Ext(companionPath))
}

// handleRasterFavicon processes a raster favicon (PNG, JPEG, GIF, WebP, etc.)
func handleRasterFavicon(faviconPath string) ([]byte, error) {
	sourceData, err := os.ReadFile(faviconPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read favicon: %w", err)
	}

	return convertRasterToPNG(sourceData, filepath.Ext(faviconPath))
}

// getIconSizesToGenerate returns all icon sizes that need to be generated
func getIconSizesToGenerate() []iconSize {
	return []iconSize{
		// Browser favicon (32x32 PNG for Safari/older browsers)
		{32, "favicon-32x32.png", nil},
		// PWA icons (PNG required for Apple PWA compatibility)
		{192, "pwa-icon-192.png", &settings.Env.PWAIcon192},
		{256, "pwa-icon-256.png", &settings.Env.PWAIcon256},
		{512, "pwa-icon-512.png", &settings.Env.PWAIcon512},
		// Platform-specific icons
		{180, "apple-touch-icon.png", nil}, // iOS home screen (required for Apple)
		{256, "mstile-256x256.png", nil},   // Windows tile
	}
}

// generateIconSizes generates all required icon sizes from source PNG data
func generateIconSizes(sourceData []byte) error {
	previewService := preview.GetService()
	if previewService == nil {
		return errors.New("preview service not initialized")
	}

	iconSizes := getIconSizesToGenerate()
	allSuccess := true
	generatedCount := 0

	for _, icon := range iconSizes {
		if err := generateSingleIcon(previewService, sourceData, icon); err != nil {
			logger.Warningf("Failed to generate %s: %v", icon.name, err)
			allSuccess = false
			continue
		}

		// Update environment variable if specified
		if icon.env != nil {
			*icon.env = filepath.Join("icons", icon.name)
		}
		generatedCount++
	}

	if allSuccess {
		logger.Debugf("Successfully generated %d icon sizes", generatedCount)
	} else {
		logger.Warningf("Generated %d/%d icon sizes (some failed)", generatedCount, len(iconSizes))
	}

	return nil
}

// generateSingleIcon generates a single icon file at the specified size
func generateSingleIcon(previewService *preview.Service, sourceData []byte, icon iconSize) error {
	outputPath := filepath.Join(settings.Env.PWAIconsDir, icon.name)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Resize image using the preview service's Resize method
	err = previewService.Resize(
		bytes.NewReader(sourceData),
		outFile,
		preview.ResizeOptions{
			Width:      icon.size,
			Height:     icon.size,
			ResizeMode: preview.ResizeModeFill,
			Quality:    preview.QualityHigh,
			Format:     preview.FormatPng,
		},
	)

	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("resize failed: %w", err)
	}

	return nil
}

// GeneratePWAIcons generates all PWA and platform icon sizes from favicon source
// All icons are generated as PNG for maximum compatibility (Apple devices, PWA, older browsers)
func GeneratePWAIcons() error {
	logger.Info("Generating PWA icons...")

	// Load and prepare source data as PNG
	sourceData, err := loadFaviconSource()
	if err != nil {
		return fmt.Errorf("failed to load favicon source: %w", err)
	}

	// Generate all icon sizes
	if err := generateIconSizes(sourceData); err != nil {
		return fmt.Errorf("failed to generate icon sizes: %w", err)
	}

	return nil
}
