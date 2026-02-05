package preview

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
//	1 processor:  single image processor for all sizes
//	2 processors: 1 large, 1 small
//	3-6 processors: 2 large, rest small
//	7+ processors: 3 large, rest small
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
		if concurrencyLimit >= 7 {
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
	return nil
}

// GetService returns the preview service instance (can be nil if not started)
func GetService() *Service {
	return service
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
	if file.Metadata != nil && len(file.Metadata.AlbumArt) > 0 {
		// For audio with album art, hash the album art content
		hasher := md5.New()
		_, _ = hasher.Write(file.Metadata.AlbumArt)
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

// filePreviewType represents the type of preview generation needed
type filePreviewType int

const (
	previewTypeDocument filePreviewType = iota
	previewTypeOffice
	previewTypeHEIC
	previewTypeImage
	previewTypeVideo
	previewTypeAudio
	previewTypeUnsupported
)

// determinePreviewType determines what type of preview generation is needed
func determinePreviewType(file iteminfo.ExtendedFileInfo) filePreviewType {
	// Check document conversion first
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		return previewTypeDocument
	}

	// Check office files
	if file.OnlyOfficeId != "" {
		return previewTypeOffice
	}

	// Check HEIC with conversion enabled
	if strings.HasPrefix(file.Type, "image/heic") &&
		*settings.Config.Integrations.Media.Convert.ImagePreview[settings.HEICImagePreview] {
		return previewTypeHEIC
	}

	// Check by MIME type prefix
	switch {
	case strings.HasPrefix(file.Type, "image"):
		return previewTypeImage
	case strings.HasPrefix(file.Type, "video"):
		return previewTypeVideo
	case strings.HasPrefix(file.Type, "audio") && file.Metadata != nil && len(file.Metadata.AlbumArt) > 0:
		return previewTypeAudio
	default:
		return previewTypeUnsupported
	}
}

// generateRawPreview generates the initial preview image bytes based on file type
func (s *Service) generateRawPreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int, hash string) ([]byte, error) {
	previewType := determinePreviewType(file)

	switch previewType {
	case previewTypeDocument:
		return s.generateDocumentPreview(ctx, file, hash)

	case previewTypeOffice:
		return s.generateOfficeFilePreview(ctx, file, officeUrl)

	case previewTypeHEIC:
		return s.generateHEICPreview(ctx, file, previewSize)

	case previewTypeImage:
		return s.generateImagePreview(ctx, file, previewSize)

	case previewTypeVideo:
		return s.generateVideoPreviewBytes(ctx, file, seekPercentage)

	case previewTypeAudio:
		if file.Metadata != nil && len(file.Metadata.AlbumArt) > 0 {
			return file.Metadata.AlbumArt, nil
		}
		return nil, nil
	case previewTypeUnsupported:
		ext := strings.ToLower(filepath.Ext(file.Name))
		return nil, fmt.Errorf("unsupported media type: %s", ext)

	default:
		return nil, errors.New("unknown preview type")
	}
}

// generateDocumentPreview generates preview for PDF and document files
func (s *Service) generateDocumentPreview(ctx context.Context, file iteminfo.ExtendedFileInfo, hash string) ([]byte, error) {
	tempFilePath := filepath.Join(s.cacheDir, "thumbnails", "docs", hash) + ".txt"
	imageBytes, err := s.GenerateImageFromDoc(ctx, file, tempFilePath, 0) // 0 for the first page
	if err != nil {
		return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
	}
	return imageBytes, nil
}

// generateOfficeFilePreview generates preview for Office files via OnlyOffice
func (s *Service) generateOfficeFilePreview(ctx context.Context, file iteminfo.ExtendedFileInfo, officeUrl string) ([]byte, error) {
	imageBytes, err := s.GenerateOfficePreview(ctx, filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create image for office file: %w", err)
	}
	return imageBytes, nil
}

// generateHEICPreview generates preview for HEIC files using FFmpeg
func (s *Service) generateHEICPreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	imageBytes, err := s.convertHEICToJPEGWithFFmpeg(ctx, file.RealPath, previewSize)
	if err != nil {
		return nil, fmt.Errorf("failed to process HEIC image file: %w", err)
	}
	return imageBytes, nil
}

// generateVideoPreviewBytes generates preview frame from video file
func (s *Service) generateVideoPreviewBytes(ctx context.Context, file iteminfo.ExtendedFileInfo, seekPercentage int) ([]byte, error) {
	// Check if this video format is enabled for preview generation
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Name)), ".")
	if !settings.CanConvertVideo(ext) {
		return nil, fmt.Errorf("video preview generation is disabled for .%s files in settings", ext)
	}

	videoSeekPercentage := seekPercentage
	if videoSeekPercentage == 0 {
		videoSeekPercentage = 10
	}

	imageBytes, err := s.GenerateVideoPreview(ctx, file.RealPath, videoSeekPercentage)
	if err != nil {
		// Don't log client cancellations as errors
		if ctx.Err() != context.Canceled {
			logger.Errorf("Video preview generation failed for '%s' (path: %s, seek: %d%%): %v",
				file.Name, file.RealPath, videoSeekPercentage, err)
		}
		return nil, fmt.Errorf("failed to create image for video file: %w", err)
	}
	return imageBytes, nil
}

// generateImagePreview generates preview for regular image files
func (s *Service) generateImagePreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	// Stream from file instead of os.ReadFile so we don't load every image fully into memory
	f, err := os.Open(file.RealPath)
	if err != nil {
		logger.Errorf("Failed to open image file '%s' (path: %s): %v", file.Name, file.RealPath, err)
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer f.Close()

	options, err := getPreviewOptions(previewSize)
	if err != nil {
		return nil, err
	}

	imageBytes, err := s.CreatePreview(f, file.Size, options)
	if err != nil {
		// For JPEG files, try FFmpeg fallback if initial decode failed
		if strings.HasPrefix(file.Type, "image/jpeg") {
			return handleJPEGFallback(ctx, s, file, previewSize, err)
		}
		return nil, fmt.Errorf("failed to create image preview: %w", err)
	}

	if len(imageBytes) < 100 {
		logger.Errorf("Generated image too small for '%s' (type: %s): %d bytes", file.Name, file.Type, len(imageBytes))
		return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
	}

	return imageBytes, nil
}

// handleJPEGFallback attempts to use FFmpeg for problematic JPEG files
// This is preview-specific logic for user files, not used for icon generation
func handleJPEGFallback(ctx context.Context, s *Service, file iteminfo.ExtendedFileInfo, previewSize string, originalErr error) ([]byte, error) {
	enableJPEGFallback := *settings.Config.Integrations.Media.Convert.ImagePreview[settings.JPEGImagePreview]

	if !enableJPEGFallback {
		return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg conversion disabled in settings): %w", originalErr)
	}

	if s.ffmpegService == nil {
		return nil, fmt.Errorf("failed to resize preview image (unsupported JPEG format, FFmpeg not available): %w", originalErr)
	}

	logger.Debugf("JPEG decode failed for '%s', falling back to FFmpeg: %v", file.Name, originalErr)
	imageBytes, err := s.convertImageWithFFmpeg(ctx, file.RealPath, previewSize)
	if err != nil {
		return nil, fmt.Errorf("failed to resize preview image with FFmpeg fallback: %w", err)
	}

	return imageBytes, nil
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
		if err := service.acquireImageLargeSem(ctx); err != nil {
			return nil, err
		}
		defer service.releaseImageLargeSem()
	} else {
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
	if strings.HasPrefix(file.Type, "image") && file.Size > iteminfo.LargeFileSizeThreshold {
		message := fmt.Sprintf("Image file too large for preview: %s (size: %d bytes, limit: %d bytes)",
			file.Name, file.Size, iteminfo.LargeFileSizeThreshold)
		logger.Warning(message)
		return nil, errors.New(message)
	}

	// Generate hash for temp file paths
	hasher := md5.New()
	_, _ = hasher.Write([]byte(CacheKey(fileMD5, previewSize, seekPercentage)))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Generate raw preview based on file type
	imageBytes, err := service.generateRawPreview(ctx, file, previewSize, officeUrl, seekPercentage, hash)
	if err != nil {
		return nil, err
	}

	// For HEIC files, we've already done the resize/conversion, cache and return directly
	previewType := determinePreviewType(file)
	if previewType == previewTypeHEIC {
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err = service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache HEIC image: %v", err)
		}
		return imageBytes, nil
	}

	// For regular images that were already resized, cache and return
	if previewType == previewTypeImage {
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err = service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache image: %v", err)
		}
		return imageBytes, nil
	}

	// Check if context was cancelled during processing
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Validate generated image size
	if len(imageBytes) < 100 {
		logger.Errorf("Generated image too small for '%s' (type: %s): %d bytes - likely an error occurred",
			file.Name, file.Type, len(imageBytes))
		return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
	}

	// Resize if needed (for videos, audio, documents, office files)
	if previewSize != "original" {
		options, err := getPreviewOptions(previewSize)
		if err != nil {
			return nil, err
		}

		resizedBytes, err := service.CreatePreview(bytes.NewReader(imageBytes), 0, options)
		if err != nil {
			// For JPEG files, try FFmpeg fallback if resize failed
			if strings.HasPrefix(file.Type, "image/jpeg") {
				resizedBytes, err = handleJPEGFallback(ctx, service, file, previewSize, err)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("failed to resize preview image: %w", err)
			}
		}

		// Cache and return resized image
		cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
		if err := service.fileCache.Store(ctx, cacheKey, resizedBytes); err != nil {
			logger.Errorf("failed to cache resized image: %v", err)
		}
		return resizedBytes, nil
	}

	// Cache and return original size
	cacheKey := CacheKey(fileMD5, previewSize, seekPercentage)
	if err := service.fileCache.Store(ctx, cacheKey, imageBytes); err != nil {
		logger.Errorf("failed to cache original image: %v", err)
	}
	return imageBytes, nil
}

func GeneratePreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int) ([]byte, error) {
	// Generate fast metadata-based cache key
	hasher := md5.New()
	cacheString := fmt.Sprintf("%s:%d:%s", file.RealPath, file.Size, file.ModTime.Format(time.RFC3339Nano))
	_, _ = hasher.Write([]byte(cacheString))
	cacheHash := hex.EncodeToString(hasher.Sum(nil))

	return GeneratePreviewWithMD5(ctx, file, previewSize, officeUrl, seekPercentage, cacheHash)
}

// getPreviewOptions returns resize options for the given preview size
func getPreviewOptions(previewSize string) (ResizeOptions, error) {
	switch previewSize {
	case "large":
		return ResizeOptions{
			Width:      640,
			Height:     640,
			ResizeMode: ResizeModeFit,
			Quality:    QualityHigh,
			Format:     FormatJpeg,
		}, nil
	case "small":
		return ResizeOptions{
			Width:      256,
			Height:     256,
			ResizeMode: ResizeModeFit,
			Quality:    QualityMedium,
			Format:     FormatJpeg,
		}, nil
	default:
		return ResizeOptions{}, ErrUnsupportedFormat
	}
}

// CreatePreview resizes an image from a reader with the given options
func (s *Service) CreatePreview(reader io.Reader, fileSize int64, options ResizeOptions) ([]byte, error) {
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
