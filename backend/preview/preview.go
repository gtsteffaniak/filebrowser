package preview

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/kovidgoyal/imaging"
)

var (
	ErrUnsupportedFormat = errors.New("preview is not available for provided file format")
	ErrUnsupportedMedia  = errors.New("unsupported media type")
	service              *Service
)

type Service struct {
	fileCache     diskcache.Interface
	cacheDir      string // Cache directory used for thumbnails and temp files
	debug         bool
	docGenMutex   sync.Mutex            // Mutex to serialize access to doc generation (required for CGO thread safety with go-fitz)
	ffmpegService *ffmpeg.FFmpegService // Shared FFmpeg service for video and HEIC/JPEG fallback
	imageSem      chan struct{}         // Semaphore for small image decode/encode (<8MB)
	imageLargeSem chan struct{}         // Semaphore for large image decode/encode (>=8MB), nil if only 1 processor
}

// Calculate split between small and large imaging library processors
// Distribution formula:
//
//	1 processor:  No split - imageSem handles all, imageLargeSem is nil
//	2 processors: 1 large, 1 small
//	3-9 processors: 2 large, rest small
//	10+ processors: 3 large, rest small
func NewPreviewGenerator(concurrencyLimit int, cacheDir string) *Service {
	if concurrencyLimit < 1 {
		concurrencyLimit = 1
	}
	// get round up half value of concurrencyLimit for FFmpeg operations
	ffmpegConcurrencyLimit := (concurrencyLimit + 1) / 2

	actualCacheDir := cacheDir
	if actualCacheDir == "" {
		actualCacheDir = os.TempDir()
	}

	var fileCache diskcache.Interface
	// Use file cache if cacheDir is specified
	var err error
	fileCache, err = diskcache.NewFileCache(actualCacheDir)
	if err != nil {
		logger.Error("The cache dir could not be created. Make sure the user that you executed the program with has access to create directories in the local path. See  ")
		logger.Fatalf("failed to create file cache path, which is now require to run the server: %v", err)
	}
	// Create directories recursively using the determined cache directory
	err = os.MkdirAll(filepath.Join(actualCacheDir, "thumbnails", "docs"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(actualCacheDir, "thumbnails", "videos"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(actualCacheDir, "heic"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}

	// Single FFmpeg service shared by video preview and HEIC/JPEG fallback
	ffmpegService := ffmpeg.NewFFmpegService(ffmpegConcurrencyLimit, settings.Config.Integrations.Media.Debug, filepath.Join(actualCacheDir, "heic"))

	var imageSem, imageLargeSem chan struct{}

	if concurrencyLimit == 1 {
		// Single processor: no split, imageLargeSem will be nil
		imageSem = make(chan struct{}, 1)
		imageLargeSem = nil
	} else {
		var largeLimit int
		if concurrencyLimit >= 10 {
			largeLimit = 3
		} else if concurrencyLimit >= 3 {
			largeLimit = 2
		} else {
			largeLimit = 1
		}
		smallLimit := concurrencyLimit - largeLimit

		imageSem = make(chan struct{}, smallLimit)
		imageLargeSem = make(chan struct{}, largeLimit)

		logger.Debugf("Image processor split: %d small, %d large (total: %d)", smallLimit, largeLimit, concurrencyLimit)
	}

	// Total max memory = concurrencyLimit × 50MB
	// Example: concurrencyLimit=10 → ~500MB max
	settings.Env.MuPdfAvailable = docEnabled()

	return &Service{
		fileCache:     fileCache,
		cacheDir:      actualCacheDir,
		debug:         settings.Config.Integrations.Media.Debug,
		ffmpegService: ffmpegService,
		imageSem:      imageSem,
		imageLargeSem: imageLargeSem,
	}
}

// Global image processor semaphore methods
// These are used to ensure FFmpeg and document operations also respect the global image processor limit
func (s *Service) acquireImageSem(ctx context.Context) error {
	select {
	case s.imageSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseImageSem() {
	<-s.imageSem
}

func (s *Service) acquireImageLargeSem(ctx context.Context) error {
	if s.imageLargeSem == nil {
		// Fall back to imageSem if imageLargeSem is not available (single processor case)
		return s.acquireImageSem(ctx)
	}
	select {
	case s.imageLargeSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseImageLargeSem() {
	if s.imageLargeSem == nil {
		// Fall back to imageSem if imageLargeSem is not available (single processor case)
		s.releaseImageSem()
		return
	}
	<-s.imageLargeSem
}

func StartPreviewGenerator(concurrencyLimit int, cacheDir string) error {
	if service != nil {
		logger.Errorf("WARNING: StartPreviewGenerator called multiple times! This will create multiple semaphores!")
	}
	service = NewPreviewGenerator(concurrencyLimit, cacheDir)

	// Generate PWA icons after service is initialized
	GeneratePWAIcons()

	return nil
}

// GetService returns the preview service instance (can be nil if not started)
func GetService() *Service {
	return service
}

// GeneratePWAIcons generates all icon sizes from favicon.svg or favicon.png
func GeneratePWAIcons() {
	if service == nil {
		logger.Warning("Preview service not initialized, skipping icon generation")
		return
	}

	// Determine source icon - prefer custom, fallback to default
	var sourceIcon string
	var sourceData []byte
	var err error
	var isSVG bool

	if settings.Env.FaviconIsCustom {
		sourceIcon = settings.Env.FaviconPath
		logger.Debugf("Generating icons from custom favicon: %s", sourceIcon)
		isSVG = strings.ToLower(filepath.Ext(sourceIcon)) == ".svg"

		// Read the source file
		sourceData, err = os.ReadFile(sourceIcon)
		if err != nil {
			logger.Warningf("Failed to read custom favicon: %v", err)
			return
		}

		// If it's not a raster format that imaging library can decode, handle separately
		if isSVG {
			// SVG: Set PWA icons to use SVG directly, look for PNG for raster generation
			logger.Debug("Source is SVG - will use directly where supported")
			settings.Env.PWAIcon192 = "pwa-icon.svg"
			settings.Env.PWAIcon256 = "pwa-icon.svg"
			settings.Env.PWAIcon512 = "pwa-icon.svg"

			// Look for PNG in same directory with same base name
			basePath := sourceIcon[:len(sourceIcon)-len(filepath.Ext(sourceIcon))]
			pngPath := basePath + ".png"
			sourceData, err = os.ReadFile(pngPath)
			if err != nil {
				logger.Warningf("SVG favicon provided without PNG version at %s. For best compatibility across all browsers and platforms, provide a PNG version alongside your SVG.", pngPath)
				return
			}
			logger.Debugf("Using PNG version (%s) for generating raster icons", pngPath)
		} else {
			// For any raster format (JPEG, PNG, GIF, WebP, etc.), decode and convert to PNG
			// This creates a standardized PNG source for all icon generation
			img, decodeErr := imaging.Decode(bytes.NewReader(sourceData))
			if decodeErr != nil {
				logger.Warningf("Failed to decode favicon image: %v", decodeErr)
				return
			}
			// Re-encode as PNG in memory for consistent processing
			var buf bytes.Buffer
			if encodeErr := imaging.Encode(&buf, img, imaging.PNG); encodeErr != nil {
				logger.Warningf("Failed to convert favicon to PNG: %v", encodeErr)
				return
			}
			sourceData = buf.Bytes()
			logger.Debugf("Converted %s favicon to PNG for icon generation", filepath.Ext(sourceIcon))
		}
	} else {
		// Use default embedded favicon - we have both SVG and PNG versions
		logger.Debug("Generating icons from default favicon")
		// For default, use the PNG version for raster generation
		assetFs := fileutils.GetAssetFS()
		if assetFs == nil {
			logger.Warning("Asset filesystem not initialized, skipping default icon generation")
			return
		}
		pngPath := "img/icons/favicon.png"
		sourceData, err = fs.ReadFile(assetFs, pngPath)
		if err != nil {
			logger.Warningf("Failed to read default favicon PNG: %v", err)
			return
		}
		// Set PWA icons to use SVG for modern browsers
		settings.Env.PWAIcon192 = "pwa-icon.svg"
		settings.Env.PWAIcon256 = "pwa-icon.svg"
		settings.Env.PWAIcon512 = "pwa-icon.svg"
	}

	// Define all icon sizes we need to generate
	// Format: {size, filename, envVar pointer (optional)}
	iconSizes := []struct {
		size int
		name string
		env  *string
	}{
		// Browser favicon
		{32, "favicon-32x32.png", nil},
		// PWA icons
		{192, "pwa-icon-192.png", &settings.Env.PWAIcon192},
		{256, "pwa-icon-256.png", &settings.Env.PWAIcon256},
		{512, "pwa-icon-512.png", &settings.Env.PWAIcon512},
		// Platform-specific icons
		{180, "apple-touch-icon.png", nil}, // iOS home screen
		{256, "mstile-256x256.png", nil},   // Windows tile
	}

	allSuccess := true
	generatedCount := 0

	for _, iconSize := range iconSizes {
		outputPath := filepath.Join(settings.Env.PWAIconsDir, iconSize.name)

		// Create output file
		outFile, err := os.Create(outputPath)
		if err != nil {
			logger.Warningf("Failed to create icon %s: %v", iconSize.name, err)
			allSuccess = false
			continue
		}

		// Resize image using the service's Resize method
		err = service.Resize(
			bytes.NewReader(sourceData),
			outFile,
			ResizeOptions{
				Width:      iconSize.size,
				Height:     iconSize.size,
				ResizeMode: ResizeModeFill,
				Quality:    QualityHigh,
				Format:     FormatPng,
			},
		)
		outFile.Close()

		if err != nil {
			logger.Warningf("Failed to generate icon %s: %v", iconSize.name, err)
			os.Remove(outputPath)
			allSuccess = false
			continue
		}

		// Update environment variable if specified
		if iconSize.env != nil {
			*iconSize.env = filepath.Join("icons", iconSize.name)
		}
		generatedCount++
	}

	if allSuccess {
		logger.Debugf("Successfully generated %d icon sizes", generatedCount)
	} else {
		logger.Warningf("Generated %d/%d icon sizes (some failed)", generatedCount, len(iconSizes))
	}
}

func GetPreviewForFile(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize, url string, seekPercentage int) ([]byte, error) {
	if !file.HasPreview {
		return nil, ErrUnsupportedMedia
	}

	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Generate fast cache key based on file metadata
	var cacheHash string
	if file.Metadata != nil && file.Metadata.AlbumArt != "" {
		// For audio with album art, hash the album art content
		hasher := md5.New()
		_, _ = hasher.Write([]byte(file.Metadata.AlbumArt))
		cacheHash = hex.EncodeToString(hasher.Sum(nil))
	} else {
		// For all other files, use fast metadata-based hash
		hasher := md5.New()
		cacheString := fmt.Sprintf("%s:%d:%s", file.RealPath, file.Size, file.ModTime.Format(time.RFC3339Nano))
		_, _ = hasher.Write([]byte(cacheString))
		cacheHash = hex.EncodeToString(hasher.Sum(nil))
	}

	cacheKey := CacheKey(cacheHash, previewSize, seekPercentage)
	if data, found, err := service.fileCache.Load(ctx, cacheKey); err != nil {
		return nil, fmt.Errorf("failed to load from cache: %w", err)
	} else if found {
		return data, nil
	}
	return GeneratePreviewWithMD5(ctx, file, previewSize, url, seekPercentage, cacheHash)
}

func GeneratePreviewWithMD5(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int, fileMD5 string) ([]byte, error) {
	// Note: fileMD5 is actually a cache hash (metadata-based), not a true file content MD5
	// Validate that cache hash is not empty to prevent cache corruption
	if fileMD5 == "" {
		errorMsg := fmt.Sprintf("Cache hash is empty for file: %s (path: %s)", file.Name, file.RealPath)
		logger.Errorf("Preview generation failed: %s", errorMsg)
		return nil, fmt.Errorf("preview generation failed: %s", errorMsg)
	}

	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Acquire global image processor semaphore for ALL operations
	const largeFileSizeThreshold = 8 * 1024 * 1024 // 8MB
	if file.Size >= largeFileSizeThreshold && service.imageLargeSem != nil {
		// Large file path - use imageLargeSem
		if err := service.acquireImageLargeSem(ctx); err != nil {
			return nil, err
		}
		defer service.releaseImageLargeSem()
	} else {
		// Small file path or fallback - use imageSem
		if err := service.acquireImageSem(ctx); err != nil {
			return nil, err
		}
		defer service.releaseImageSem()
	}

	// Check if context is cancelled after acquiring semaphore
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Enforce file size limit for image preview generation to prevent memory exhaustion
	// 50MB limit is reasonable for preview generation while allowing most images
	if strings.HasPrefix(file.Type, "image") && file.Size > iteminfo.LargeFileSizeThreshold {
		logger.Warningf("Image file too large for preview: %s (size: %d bytes, limit: %d bytes)",
			file.Name, file.Size, iteminfo.LargeFileSizeThreshold)
		return nil, fmt.Errorf("image file too large for preview generation: %d MB (limit: 50 MB)",
			file.Size/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(file.Name))
	var (
		err        error
		imageBytes []byte
	)

	// Generate thumbnail image from video
	hasher := md5.New()
	_, _ = hasher.Write([]byte(CacheKey(fileMD5, previewSize, seekPercentage)))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Check if HEIC conversion is enabled (guaranteed non-nil after defaults)
	convertHEIC := strings.HasPrefix(file.Type, "image/heic") && *settings.Config.Integrations.Media.Convert.ImagePreview[settings.HEICImagePreview]

	// Generate an image from office document
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		tempFilePath := filepath.Join(service.cacheDir, "thumbnails", "docs", hash) + ".txt"
		imageBytes, err = service.GenerateImageFromDoc(ctx, file, tempFilePath, 0) // 0 for the first page
		if err != nil {
			return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
		}
	} else if file.OnlyOfficeId != "" {
		imageBytes, err = service.GenerateOfficePreview(ctx, filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if convertHEIC {
		// HEIC files need FFmpeg conversion to JPEG with proper size/quality handling
		imageBytes, err = service.convertHEICToJPEGWithFFmpeg(ctx, file.RealPath, previewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to process HEIC image file: %w", err)
		}
		// For HEIC files, we've already done the resize/conversion, so cache and return directly
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err = service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache HEIC image: %v", err)
		}
		return imageBytes, nil
	} else if strings.HasPrefix(file.Type, "image") {
		// Stream from file instead of os.ReadFile so we don't load every image fully into memory.
		// ResizeWithSize() holds the imaging semaphore and only reads in chunks / decodes one at a time.
		f, openErr := os.Open(file.RealPath)
		if openErr != nil {
			logger.Errorf("Failed to open image file '%s' (path: %s): %v", file.Name, file.RealPath, openErr)
			return nil, fmt.Errorf("failed to open image file: %w", openErr)
		}
		defer f.Close()
		imageBytes, err = service.CreatePreviewFromReaderWithSize(f, previewSize, file.Size)
		if err != nil {
			// Check if this is an unsupported JPEG format and FFmpeg fallback is enabled
			errMsg := err.Error()
			if strings.HasPrefix(file.Type, "image/jpeg") &&
				(strings.Contains(errMsg, "unrecognised marker") ||
					strings.Contains(errMsg, "invalid JPEG format") ||
					strings.Contains(errMsg, "Huffman")) {
				enableJPEGFallback := *settings.Config.Integrations.Media.Convert.ImagePreview[settings.JPEGImagePreview]
				if enableJPEGFallback && service.ffmpegService != nil {
					logger.Debugf("JPEG decode failed for '%s', falling back to FFmpeg: %v", file.Name, err)
					imageBytes, err = service.convertImageWithFFmpeg(ctx, file.RealPath, previewSize)
					if err != nil {
						return nil, fmt.Errorf("failed to resize preview image with FFmpeg fallback: %w", err)
					}
					cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
					if err = service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
						logger.Errorf("failed to cache FFmpeg-converted image: %v", err)
					}
					return imageBytes, nil
				}
				if !enableJPEGFallback {
					return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg conversion disabled in settings): %w", err)
				}
				return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg not available): %w", err)
			}
			return nil, fmt.Errorf("failed to create image preview: %w", err)
		}
		if len(imageBytes) < 100 {
			logger.Errorf("Generated image too small for '%s' (type: %s): %d bytes", file.Name, file.Type, len(imageBytes))
			return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
		}
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err = service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache image: %v", err)
		}
		return imageBytes, nil
	} else if strings.HasPrefix(file.Type, "video") {
		// Check if this video format is enabled for preview generation
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Name)), ".")
		if !settings.CanConvertVideo(ext) {
			return nil, fmt.Errorf("video preview generation is disabled for .%s files in settings", ext)
		}

		videoSeekPercentage := seekPercentage
		if videoSeekPercentage == 0 {
			videoSeekPercentage = 10
		}
		imageBytes, err = service.GenerateVideoPreview(ctx, file.RealPath, videoSeekPercentage)
		if err != nil {
			// Don't log client cancellations as errors
			if ctx.Err() != context.Canceled {
				logger.Errorf("Video preview generation failed for '%s' (path: %s, seek: %d%%): %v",
					file.Name, file.RealPath, videoSeekPercentage, err)
			}
			return nil, fmt.Errorf("failed to create image for video file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "audio") {
		// Extract album artwork from audio files
		if file.Metadata != nil && file.Metadata.AlbumArt != "" {
			imageBytes, err = base64.StdEncoding.DecodeString(file.Metadata.AlbumArt)
			if err != nil {
				return nil, fmt.Errorf("failed to decode album artwork: %w", err)
			}
		} else {
			return nil, nil
		}
	} else {
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}

	// Check if context was cancelled during processing
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if len(imageBytes) < 100 {
		logger.Errorf("Generated image too small for '%s' (type: %s): %d bytes - likely an error occurred",
			file.Name, file.Type, len(imageBytes))
		return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
	}

	if previewSize != "original" {
		// resize image
		resizedBytes, err := service.CreatePreview(imageBytes, previewSize)
		if err != nil {
			// Check if this is an unsupported JPEG format (extended sequential, etc.)
			errMsg := err.Error()
			if strings.HasPrefix(file.Type, "image/jpeg") &&
				(strings.Contains(errMsg, "unrecognised marker") ||
					strings.Contains(errMsg, "invalid JPEG format") ||
					strings.Contains(errMsg, "Huffman")) {

				// Check if JPEG FFmpeg conversion is enabled (guaranteed non-nil after defaults)
				enableJPEGFallback := *settings.Config.Integrations.Media.Convert.ImagePreview[settings.JPEGImagePreview]

				// Only attempt FFmpeg fallback if it's enabled and FFmpeg service is available
				if enableJPEGFallback && service.ffmpegService != nil {
					// Fall back to FFmpeg for problematic JPEG files
					logger.Debugf("JPEG decode failed for '%s', falling back to FFmpeg: %v", file.Name, err)
					resizedBytes, err = service.convertImageWithFFmpeg(ctx, file.RealPath, previewSize)
					if err != nil {
						return nil, fmt.Errorf("failed to resize preview image with FFmpeg fallback: %w", err)
					}
					// Cache and return the FFmpeg-converted image
					cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
					if err = service.fileCache.Store(ctx, cacheKey, resizedBytes); err != nil {
						logger.Errorf("failed to cache FFmpeg-converted image: %v", err)
					}
					return resizedBytes, nil
				}
				// If FFmpeg fallback is disabled or not available, return the original error
				if !enableJPEGFallback {
					return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg conversion disabled in settings): %w", err)
				}
				return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg not available): %w", err)
			}
			return nil, fmt.Errorf("failed to resize preview image: %w", err)
		}
		// Cache and return
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err := service.fileCache.Store(ctx, cacheKey, resizedBytes); err != nil {
			logger.Errorf("failed to cache resized image: %v", err)
		}
		return resizedBytes, nil
	} else {
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err := service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache original image: %v", err)
		}
		return imageBytes, nil
	}
}

func GeneratePreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int) ([]byte, error) {
	// Generate fast metadata-based cache key
	hasher := md5.New()
	cacheString := fmt.Sprintf("%s:%d:%s", file.RealPath, file.Size, file.ModTime.Format(time.RFC3339Nano))
	_, _ = hasher.Write([]byte(cacheString))
	cacheHash := hex.EncodeToString(hasher.Sum(nil))

	return GeneratePreviewWithMD5(ctx, file, previewSize, officeUrl, seekPercentage, cacheHash)
}

func (s *Service) CreatePreview(data []byte, previewSize string) ([]byte, error) {
	return s.CreatePreviewFromReader(bytes.NewReader(data), previewSize)
}

// CreatePreviewFromReader resizes an image from a reader (e.g. *os.File) without loading it fully into memory.
// Used for the image preview path so only concurrencyLimit decodes run at once.
func (s *Service) CreatePreviewFromReader(reader io.Reader, previewSize string) ([]byte, error) {
	return s.CreatePreviewFromReaderWithSize(reader, previewSize, 0)
}

// CreatePreviewFromReaderWithSize resizes an image with file size information for semaphore selection.
func (s *Service) CreatePreviewFromReaderWithSize(reader io.Reader, previewSize string, fileSize int64) ([]byte, error) {
	var options ResizeOptions

	switch previewSize {
	case "large":
		options = ResizeOptions{
			Width:      640,
			Height:     640,
			ResizeMode: ResizeModeFit,
			Quality:    QualityHigh,
			Format:     FormatJpeg,
		}
	case "small":
		options = ResizeOptions{
			Width:      256,
			Height:     256,
			ResizeMode: ResizeModeFit,
			Quality:    QualityMedium,
			Format:     FormatJpeg,
		}
	default:
		return nil, ErrUnsupportedFormat
	}

	output := &bytes.Buffer{}
	if err := s.ResizeWithSize(reader, output, fileSize, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func CacheKey(md5, previewSize string, percentage int) string {
	key := fmt.Sprintf("%x%x%x", md5, previewSize, percentage)
	return key
}

func DelThumbs(ctx context.Context, file iteminfo.ExtendedFileInfo) {
	// Generate metadata-based cache hash for deletion
	hasher := md5.New()
	cacheString := fmt.Sprintf("%s:%d:%s", file.RealPath, file.Size, file.ModTime.Format(time.RFC3339Nano))
	_, _ = hasher.Write([]byte(cacheString))
	cacheHash := hex.EncodeToString(hasher.Sum(nil))

	errSmall := service.fileCache.Delete(ctx, CacheKey(cacheHash, "small", 0))
	if errSmall != nil {
		errLarge := service.fileCache.Delete(ctx, CacheKey(cacheHash, "large", 0))
		if errLarge != nil {
			logger.Debugf("Could not delete thumbnail: %v", file.Name)
		}
	}
}
