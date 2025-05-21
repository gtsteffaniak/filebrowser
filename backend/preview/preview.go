package preview

import (
	"bytes"
	"context"
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
}

func New(concurrencyLimit int, ffmpegPath string, cacheDir string) *Service {
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
	ffprobePath := ""
	ffmpegMainPath := ""
	var err error
	if ffmpegPath != "" {
		ffmpegMainPath, err = CheckValidFFmpeg(ffmpegPath)
		if err != nil {
			logger.Fatalf("the configured ffmpeg path does not contain a valid ffmpeg binary %s, err: %v", ffmpegPath, err)
		}
		ffprobePath, err = CheckValidFFprobe(ffmpegPath)
		if err != nil {
			logger.Fatalf("the configured ffmpeg path is not a valid ffprobe binary %s, err: %v", ffmpegPath, err)
		}
	}
	return &Service{
		sem:         make(chan struct{}, concurrencyLimit),
		ffmpegPath:  ffmpegMainPath,
		ffprobePath: ffprobePath,
		fileCache:   fileCache,
	}
}

func Start(concurrencyLimit int, ffmpegPath, cacheDir string) error {
	service = New(concurrencyLimit, ffmpegPath, cacheDir)
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
	if file.OnlyOfficeId != "" {
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
		outPathPattern := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "video", CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime, seekPercentage)+".jpg")
		defer os.Remove(outPathPattern) // cleanup
		if err = service.GenerateVideoPreview(file.RealPath, outPathPattern, seekPercentage); err != nil {
			return nil, fmt.Errorf("failed to generate video preview: %w", err)
		}
		imageBytes, err = os.ReadFile(outPathPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to read video thumbnail: %w", err)
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

	if err := s.Resize(context.Background(), input, width, height, output, options...); err != nil {
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
	if file.OnlyOfficeId != "" {
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
	if file.OnlyOfficeId != "" {
		return true
	}
	if file.Type == "application/pdf" {
		return true
	}
	return false
}

func CheckValidFFmpeg(path string) (string, error) {
	var exeExt string
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}

	ffmpegPath := filepath.Join(path, "ffmpeg"+exeExt)
	cmd := exec.Command(ffmpegPath, "-version")
	return ffmpegPath, cmd.Run()
}

func CheckValidFFprobe(path string) (string, error) {
	var exeExt string
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}

	ffprobePath := filepath.Join(path, "ffprobe"+exeExt)
	cmd := exec.Command(ffprobePath, "-version")
	return ffprobePath, cmd.Run()
}
