package preview

import (
	"bytes"
	"context"
	"crypto/md5"
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
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"

	// heic support
	_ "github.com/adrium/goheif"
)

var (
	ErrUnsupportedFormat = errors.New("preview is not available for provided file format")
	ErrUnsupportedMedia  = errors.New("unsupported media type")
	service              *Service
)

type Service struct {
	sem         chan struct{}
	ffmpegPath  string
	ffprobePath string
	fileCache   diskcache.Interface
	debug       bool
	docGenMutex sync.Mutex // Mutex to serialize access to doc generation
}

func NewPreviewGenerator(concurrencyLimit int, ffmpegPath string, cacheDir string) *Service {
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
	// Create directories recursively with 0755 permissions
	err := os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs"), 0755)
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "videos"), 0755)
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
		logger.Debugf("Media Enabled            : %v", errprobe == nil)
		settings.Config.Integrations.Media.FfmpegPath = filepath.Base(ffmpegMainPath)
	}
	settings.Config.Server.MuPdfAvailable = docEnabled()
	logger.Debugf("MuPDF Enabled            : %v", settings.Config.Server.MuPdfAvailable)
	return &Service{
		sem:         make(chan struct{}, concurrencyLimit),
		ffmpegPath:  ffmpegMainPath,
		ffprobePath: ffprobePath,
		fileCache:   fileCache,
		debug:       settings.Config.Server.DebugMedia,
	}
}

func StartPreviewGenerator(concurrencyLimit int, ffmpegPath, cacheDir string) error {
	service = NewPreviewGenerator(concurrencyLimit, ffmpegPath, cacheDir)
	return nil
}

func GetPreviewForFile(file iteminfo.ExtendedFileInfo, previewSize, url string, seekPercentage int) ([]byte, error) {
	if !AvailablePreview(file) {
		return nil, ErrUnsupportedMedia
	}
	cacheKey := CacheKey(file.RealPath, previewSize, file.ModTime, seekPercentage)
	if data, found, err := service.fileCache.Load(context.Background(), cacheKey); err != nil {
		return nil, fmt.Errorf("failed to load from cache: %w", err)
	} else if found {
		return data, nil
	}

	return GeneratePreview(file, previewSize, url, seekPercentage)
}

func GeneratePreview(file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	var (
		err        error
		imageBytes []byte
	)
	// Generate thumbnail image from video
	hasher := md5.New() //nolint:gosec
	_, _ = hasher.Write([]byte(CacheKey(file.RealPath, previewSize, file.ModTime, seekPercentage)))
	hash := hex.EncodeToString(hasher.Sum(nil))
	// Generate an image from office document
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		tempFilePath := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs", hash) + ".txt"
		imageBytes, err = service.GenerateImageFromDoc(file, tempFilePath, 0) // 0 for the first page
		if err != nil {
			return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
		}
	} else if file.OnlyOfficeId != "" {
		imageBytes, err = service.GenerateOfficePreview(filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "image") {
		if file.Type == "image/heic" {
			// HEIC files need conversion to JPEG with proper size/quality handling
			imageBytes, err = service.convertHEICToJPEGWithSize(file.RealPath, previewSize)
			if err != nil {
				return nil, fmt.Errorf("failed to convert HEIC image file: %w", err)
			}
			// For HEIC files, we've already done the resize/conversion, so cache and return directly
			cacheKey := CacheKey(file.RealPath, previewSize, file.ModTime, seekPercentage)
			if err = service.fileCache.Store(context.Background(), cacheKey, imageBytes); err != nil {
				logger.Errorf("failed to cache HEIC image: %v", err)
			}
			return imageBytes, nil
		} else {
			// get image bytes from file (non-HEIC images)
			imageBytes, err = os.ReadFile(file.RealPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read image file: %w", err)
			}
		}
	} else if strings.HasPrefix(file.Type, "video") {
		if seekPercentage == 0 {
			seekPercentage = 10
		}
		outPathPattern := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "videos", hash) + ".jpg"
		defer os.Remove(outPathPattern) // cleanup
		imageBytes, err = service.GenerateVideoPreview(file.RealPath, outPathPattern, seekPercentage)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for video file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}
	if len(imageBytes) < 100 {
		return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
	}

	if previewSize != "original" {
		// resize image
		resizedBytes, err := service.CreatePreview(imageBytes, previewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to resize preview image: %w", err)
		}
		// Cache and return
		cacheKey := CacheKey(file.RealPath, previewSize, file.ModTime, seekPercentage)
		if err := service.fileCache.Store(context.Background(), cacheKey, resizedBytes); err != nil {
			logger.Errorf("failed to cache resized image: %v", err)
		}
		return resizedBytes, nil
	} else {
		cacheKey := CacheKey(file.RealPath, previewSize, file.ModTime, seekPercentage)
		if err := service.fileCache.Store(context.Background(), cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache resized image: %v", err)
		}
		return imageBytes, nil
	}

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

func CacheKey(realPath, previewSize string, modTime time.Time, percentage int) string {
	return fmt.Sprintf("%x%x%x%x", realPath, modTime.Unix(), previewSize, percentage)
}

func DelThumbs(ctx context.Context, file iteminfo.ExtendedFileInfo) {
	errSmall := service.fileCache.Delete(ctx, CacheKey(file.RealPath, "small", file.ModTime, 0))
	if errSmall != nil {
		errLarge := service.fileCache.Delete(ctx, CacheKey(file.RealPath, "large", file.ModTime, 0))
		if errLarge != nil {
			logger.Debugf("Could not delete thumbnail: %v", file.Name)
		}
	}
}

func AvailablePreview(file iteminfo.ExtendedFileInfo) bool {
	if strings.HasPrefix(file.Type, "video") && service.ffmpegPath != "" {
		return true
	}
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".heic", ".heif":
		return true
	}
	return file.OnlyOfficeId != ""
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

// convertHEICToJPEGWithSize converts a HEIC file to JPEG format with proper size and quality settings
func (s *Service) convertHEICToJPEGWithSize(filePath string, previewSize string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open HEIC file: %w", err)
	}
	defer file.Close()

	// Determine target dimensions and quality based on preview size
	var width, height int
	var options []Option

	switch previewSize {
	case "large":
		width, height = 640, 640
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityHigh), WithFormat(FormatJpeg)}
	case "small":
		width, height = 256, 256
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityMedium), WithFormat(FormatJpeg)}
	case "original":
		// For original size HEIC, use very large dimensions to preserve original size
		// The Fit mode will maintain aspect ratio and orientation
		width, height = 8192, 8192
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityHigh), WithFormat(FormatJpeg)}
	default:
		return nil, ErrUnsupportedFormat
	}

	// Pre-allocate buffer with reasonable size for HEIC->JPEG conversion
	output := bytes.NewBuffer(make([]byte, 0, 256*1024))

	// Convert HEIC to JPEG with proper dimensions, quality, and orientation handling
	err = s.Resize(file, width, height, output, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HEIC to JPEG: %w", err)
	}

	return output.Bytes(), nil
}
