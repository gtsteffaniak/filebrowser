package preview

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	ffmpegPath   string
	ffprobePath  string
	fileCache    diskcache.Interface
	debug        bool
	docGenMutex  sync.Mutex    // Mutex to serialize access to doc generation
	docSemaphore chan struct{} // Semaphore for document generation
	officeSem    chan struct{} // Semaphore for office document processing
	videoService *ffmpeg.VideoService
	imageService *ffmpeg.ImageService
}

func NewPreviewGenerator(concurrencyLimit int, ffmpegPath string, cacheDir string) *Service {
	// Hard limit ffmpeg concurrency to prevent I/O lockup
	// Users can configure this, but we enforce a reasonable maximum
	const maxFFmpegConcurrency = 4
	if concurrencyLimit > maxFFmpegConcurrency {
		concurrencyLimit = maxFFmpegConcurrency
	}
	if concurrencyLimit < 1 {
		concurrencyLimit = 1
	}

	var fileCache diskcache.Interface
	// Use file cache if cacheDir is specified
	if cacheDir != "" {
		var err error
		fileCache, err = diskcache.NewFileCache(cacheDir)
		if err != nil {
			if cacheDir == "tmp" {
				logger.Error("The cache dir could not be created. Make sure the user that you executed the program with has access to create directories in the local path. filebrowser is trying to use the default `server.cacheDir: tmp` , but you can change this location if you need to. Please see configuration wiki for more information about this error. https://github.com/gtsteffaniak/filebrowser/wiki/Configuration")
			}
			logger.Fatalf("failed to create file cache path, which is now require to run the server: %v", err)
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	// Create directories recursively
	err := os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "videos"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "heic"), fileutils.PermDir)
	if err != nil {
		logger.Error(err)
	}
	ffmpegMainPath, err := CheckValidFFmpeg(ffmpegPath)
	if err != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path does not contain a valid ffmpeg binary %s, err: %v", ffmpegPath, err)
	}
	ffprobePath, errprobe := CheckValidFFprobe(ffmpegPath)
	if errprobe != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path is not a valid ffprobe binary %s, err: %v", ffmpegPath, err)
	}
	if errprobe == nil && err == nil {
		settings.Config.Integrations.Media.FfmpegPath = filepath.Base(ffmpegMainPath)
	}
	logger.Debugf("Media Enabled            : %v", ffmpegMainPath != "" && ffprobePath != "")
	logger.Debugf("FFmpeg Concurrency Limit : %d", concurrencyLimit)
	settings.Config.Server.MuPdfAvailable = docEnabled()
	logger.Debugf("MuPDF Enabled            : %v", settings.Config.Server.MuPdfAvailable)

	// Create shared ffmpeg services
	var videoService *ffmpeg.VideoService
	var imageService *ffmpeg.ImageService

	if ffmpegMainPath != "" && ffprobePath != "" {
		videoService = ffmpeg.NewVideoService(ffmpegMainPath, ffprobePath, concurrencyLimit, settings.Config.Server.DebugMedia)
		imageService = ffmpeg.NewImageService(ffmpegMainPath, ffprobePath, concurrencyLimit, settings.Config.Server.DebugMedia, filepath.Join(settings.Config.Server.CacheDir, "heic"))
	}

	return &Service{
		ffmpegPath:  ffmpegMainPath,
		ffprobePath: ffprobePath,
		fileCache:   fileCache,
		debug:       settings.Config.Server.DebugMedia,
		// CGo library (go-fitz) is NOT thread-safe - only 1 concurrent operation allowed
		docSemaphore: make(chan struct{}, 1),
		officeSem:    make(chan struct{}, concurrencyLimit),
		videoService: videoService,
		imageService: imageService,
	}
}

// Document semaphore methods
func (s *Service) acquireDoc(ctx context.Context) error {
	select {
	case s.docSemaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseDoc() {
	<-s.docSemaphore
}

// Office semaphore methods
func (s *Service) acquireOffice(ctx context.Context) error {
	select {
	case s.officeSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseOffice() {
	<-s.officeSem
}

func StartPreviewGenerator(concurrencyLimit int, ffmpegPath, cacheDir string) error {
	if service != nil {
		logger.Errorf("WARNING: StartPreviewGenerator called multiple times! This will create multiple semaphores!")
	}
	service = NewPreviewGenerator(concurrencyLimit, ffmpegPath, cacheDir)
	return nil
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
	if file.AudioMeta != nil && file.AudioMeta.AlbumArt != "" {
		// For audio with album art, hash the album art content
		hasher := md5.New()
		_, _ = hasher.Write([]byte(file.AudioMeta.AlbumArt))
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

	ext := strings.ToLower(filepath.Ext(file.Name))
	var (
		err        error
		imageBytes []byte
	)

	// Generate thumbnail image from video
	hasher := md5.New()
	_, _ = hasher.Write([]byte(CacheKey(fileMD5, previewSize, seekPercentage)))
	hash := hex.EncodeToString(hasher.Sum(nil))
	// Generate an image from office document
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		tempFilePath := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs", hash) + ".txt"
		imageBytes, err = service.GenerateImageFromDoc(ctx, file, tempFilePath, 0) // 0 for the first page
		if err != nil {
			return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
		}
	} else if file.OnlyOfficeId != "" {
		imageBytes, err = service.GenerateOfficePreview(ctx, filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "image/heic") {
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
		imageBytes, err = os.ReadFile(file.RealPath)
		if err != nil {
			logger.Errorf("Failed to read image file '%s' (path: %s): %v", file.Name, file.RealPath, err)
			return nil, fmt.Errorf("failed to read image file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "video") {
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
		if file.AudioMeta != nil && file.AudioMeta.AlbumArt != "" {
			imageBytes, err = base64.StdEncoding.DecodeString(file.AudioMeta.AlbumArt)
			if err != nil {
				return nil, fmt.Errorf("failed to decode album artwork: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no album artwork available for audio file: %s", file.Name)
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
	var (
		width   int
		height  int
		options []Option
	)

	switch previewSize {
	case "large":
		width, height = 640, 640
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityHigh), WithFormat(FormatJpeg)}
	case "small":
		width, height = 256, 256
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityMedium), WithFormat(FormatJpeg)}
	default:
		return nil, ErrUnsupportedFormat
	}

	input := bytes.NewReader(data)
	output := &bytes.Buffer{}

	if err := s.Resize(input, width, height, output, options...); err != nil {
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

// CheckValidFFmpeg checks for a valid ffmpeg executable.
// If a path is provided, it looks there. Otherwise, it searches the system's PATH.
func CheckValidFFmpeg(path string) (string, error) {
	return checkExecutable(path, "ffmpeg")
}

// CheckValidFFprobe checks for a valid ffprobe executable.
// If a path is provided, it looks there. Otherwise, it searches the system's PATH.
func CheckValidFFprobe(path string) (string, error) {
	return checkExecutable(path, "ffprobe")
}

// checkExecutable is an internal helper function to find and validate an executable.
// It checks a specific path if provided, otherwise falls back to searching the system PATH.
func checkExecutable(providedPath, execName string) (string, error) {
	// Add .exe extension for Windows systems
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	var finalPath string
	var err error

	if providedPath != "" {
		// A path was provided, so we'll use it.
		finalPath = filepath.Join(providedPath, execName)
	} else {
		// No path was provided, so search the system's PATH for the executable.
		finalPath, err = exec.LookPath(execName)
		if err != nil {
			// The executable was not found in the system's PATH.
			return "", err
		}
	}

	// Verify the executable is valid by running the "-version" command.
	cmd := exec.Command(finalPath, "-version")
	err = cmd.Run()

	return finalPath, err
}
