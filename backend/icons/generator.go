package icons

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
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
	size     int
	name     string
	maskable bool
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
		logger.Debugf("Processing custom favicon: %s", settings.Env.FaviconPath)
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
	logger.Debug("Looking for companion for SVG favicon")

	companionData, companionPath, err := findRasterCompanion(svgPath)
	if err != nil {
		logger.Warningf("SVG favicon has no companion: %v", err)
		logger.Warning("Falling back to default embedded favicon")
		logger.Warning("For best Apple/iOS compatibility, provide a PNG/JPG companion alongside your SVG")
		return loadDefaultFavicon()
	}

	logger.Debugf("Found companion: %s", companionPath)
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
		{32, "favicon-32x32.png", false},
		// PWA icons (PNG required for Apple PWA compatibility)
		{192, "pwa-icon-192.png", false},
		{256, "pwa-icon-256.png", false}, // Also serves as Windows tile
		{512, "pwa-icon-512.png", false},
		// Maskable PWA icons: art inset into the Android adaptive-icon safe zone
		{192, "pwa-icon-maskable-192.png", true},
		{512, "pwa-icon-maskable-512.png", true},
		// Platform-specific icons
		{180, "apple-touch-icon.png", false}, // iOS home screen (required for Apple)
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

		generatedCount++
	}

	if allSuccess {
		logger.Debugf("Successfully generated %d icon sizes", generatedCount)
	} else {
		logger.Warningf("Generated %d/%d icon sizes (some failed)", generatedCount, len(iconSizes))
	}

	return nil
}

// generateMaskableIcon insets the art into the safe zone on an opaque background, since launchers zoom maskable icons to fill the mask edge-to-edge
func generateMaskableIcon(sourceData []byte, icon iconSize) error {
	img, err := imaging.Decode(bytes.NewReader(sourceData))
	if err != nil {
		return fmt.Errorf("failed to decode source: %w", err)
	}

	// Android adaptive-icon content ratio (72/108 ≈ 2/3) keeps square art inside every mask shape
	inner := icon.size * 2 / 3
	art := imaging.Fit(img, inner, inner, imaging.Lanczos)
	canvas := imaging.PasteCenter(imaging.New(icon.size, icon.size, maskableBackground(img)), art)

	outputPath := filepath.Join(settings.PWAIconsCacheDir(), icon.name)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	if err := imaging.Encode(outFile, canvas, imaging.PNG); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("encode failed: %w", err)
	}
	return nil
}

// maskableBackground samples the art's corners then edge midpoints for an opaque pad color, else white
func maskableBackground(img image.Image) color.Color {
	b := img.Bounds()
	cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	points := []image.Point{
		{b.Min.X, b.Min.Y}, {b.Max.X - 1, b.Min.Y}, {b.Min.X, b.Max.Y - 1}, {b.Max.X - 1, b.Max.Y - 1},
		{cx, b.Min.Y}, {cx, b.Max.Y - 1}, {b.Min.X, cy}, {b.Max.X - 1, cy},
	}
	for _, p := range points {
		r, g, bl, a := img.At(p.X, p.Y).RGBA()
		if a == 0xffff {
			return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: 255}
		}
	}
	return color.White
}

// generateSingleIcon generates a single icon file at the specified size
func generateSingleIcon(previewService *preview.Service, sourceData []byte, icon iconSize) error {
	if icon.maskable {
		return generateMaskableIcon(sourceData, icon)
	}

	outputPath := filepath.Join(settings.PWAIconsCacheDir(), icon.name)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Resize image using the preview service's Resize method
	err = previewService.Resize(
		context.Background(),
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
	cacheDir := settings.PWAIconsCacheDir()
	// Create icons directory with configured permissions
	// Note: Parent cache directory should already exist from testCacheDirSpeed()
	if err := os.MkdirAll(cacheDir, fileutils.PermDir); err != nil {
		logger.Warningf("Failed to create PWA icons directory %s: %v", cacheDir, err)
	}
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
