package preview

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
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

	ffmpegMainPath, err := CheckValidFFmpeg(ffmpegPath)
	if err != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path does not contain a valid ffmpeg binary %s, err: %v", ffmpegPath, err)
	}
	ffprobePath, errprobe := CheckValidFFprobe(ffmpegPath)
	if errprobe != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path is not a valid ffprobe binary %s, err: %v", ffmpegPath, err)
	}
	if errprobe == nil && err == nil {
		logger.Infof("Media Enabled            : %v", errprobe == nil)
		settings.Config.Integrations.Media.FfmpegPath = filepath.Base(ffmpegMainPath)
	}
	settings.Config.Server.PdfAvailable = pdfEnabled()
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
	cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime, seekPercentage)
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

	// Generate an image from office document
	if file.Type == "application/pdf" && settings.Config.Server.PdfAvailable {
		imageBytes, err = service.GenerateImageFromPDF(file.RealPath, 0) // 0 for the first page
		if err != nil {
			return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
		}
	} else if file.OnlyOfficeId != "" {
		imageBytes, err = service.GenerateOfficePreview(filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "image") {
		// get image bytes from file
		imageBytes, err = os.ReadFile(file.RealPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read image file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "video") {
		if seekPercentage == 0 {
			seekPercentage = 10
		}
		// Generate thumbnail image from video
		hasher := sha1.New() //nolint:gosec
		_, _ = hasher.Write([]byte(CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime, seekPercentage)))
		hash := hex.EncodeToString(hasher.Sum(nil))
		outPathPattern := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "video", hash)
		defer os.Remove(outPathPattern) // cleanup
		imageBytes, err = service.GenerateVideoPreview(file.RealPath, outPathPattern, seekPercentage)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for video file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}
	resizedBytes, err := service.CreatePreview(imageBytes, previewSize)
	if err != nil {
		return nil, fmt.Errorf("failed to resize preview image: %w", err)
	}
	// Cache and return
	cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime, seekPercentage)
	if err := service.fileCache.Store(context.Background(), cacheKey, resizedBytes); err != nil {
		logger.Errorf("failed to cache resized image: %v", err)
	}
	return resizedBytes, nil
}

func (s *Service) CreatePreview(data []byte, previewSize string) ([]byte, error) {
	var (
		width   int
		height  int
		options []Option
	)

	switch previewSize {
	case "large":
		width, height = 512, 512
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
	errSmall := service.fileCache.Delete(ctx, CacheKey(file.RealPath, "small", file.ItemInfo.ModTime, 0))
	if errSmall != nil {
		errLarge := service.fileCache.Delete(ctx, CacheKey(file.RealPath, "large", file.ItemInfo.ModTime, 0))
		if errLarge != nil {
			logger.Debugf("Could not delete thumbnail: %v", file.Name)
		}
	}
}

func AvailablePreview(file iteminfo.ExtendedFileInfo) bool {
	if strings.HasPrefix(file.Type, "video") && service.ffmpegPath != "" {
		return true
	}
	if file.Type == "application/pdf" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff":
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
