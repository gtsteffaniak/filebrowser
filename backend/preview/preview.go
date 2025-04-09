package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

var (
	ErrUnsupportedMedia = errors.New("unsupported media type")
	service             *Service
)

type Service struct {
	sem        chan struct{}
	ffmpegPath string
	fileCache  diskcache.Interface
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
			logger.Fatal(fmt.Sprintf("failed to create file cache path, which is now require to run the server: %v", err))
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	return &Service{
		sem:        make(chan struct{}, concurrencyLimit),
		ffmpegPath: ffmpegPath,
		fileCache:  fileCache,
	}
}

func Start(concurrencyLimit int, ffmpegPath, cacheDir string) error {
	service = New(concurrencyLimit, ffmpegPath, cacheDir)
	return nil
}

func GetPreviewForFile(file iteminfo.ExtendedFileInfo, previewSize, rawUrl string) ([]byte, error) {
	if !AvailablePreview(file) {
		return nil, ErrUnsupportedMedia
	}
	cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
	if data, found, err := service.fileCache.Load(context.Background(), cacheKey); err != nil {
		return nil, fmt.Errorf("failed to load from cache: %w", err)
	} else if found {
		return data, nil
	}
	return GeneratePreview(file, previewSize, rawUrl)
}

func GeneratePreview(file iteminfo.ExtendedFileInfo, previewSize, rawUrl string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	var err error
	var data []byte
	if file.OnlyOfficeId != "" {
		data, err = service.GenerateOfficePreview(filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, rawUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "image") {
		data, err = service.CreatePreview(file, previewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for video: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "video") {
		outPathPattern := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "video", CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)+".jpg")
		defer os.Remove(outPathPattern) // always clean up preview after its used (should be in cache now)

		if err = service.GenerateVideoPreview(file.RealPath, outPathPattern, 5); err != nil {
			return nil, fmt.Errorf("failed to generate video preview: %w", err)
		}

		// Read and return the generated preview
		outFile, err := os.Open(outPathPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to open generated preview file: %w", err)
		}
		defer outFile.Close()
		data, err = io.ReadAll(outFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read generated preview file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}

	go func() {
		cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
		if err := service.fileCache.Store(context.Background(), cacheKey, data); err != nil {
			logger.Error(fmt.Sprintf("failed to cache resized image: %v", err))
		}
	}()
	return data, nil
}

func (s *Service) CreatePreview(file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	fd, err := os.Open(file.RealPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	var (
		width   int
		height  int
		options []Option
	)

	switch previewSize {
	case "large":
		width, height = 1080, 1080
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityMedium)}
	case "small":
		width, height = 256, 256
		options = []Option{WithMode(ResizeModeFill), WithQuality(QualityLow), WithFormat(FormatJpeg)}
	default:
		return nil, ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err := s.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func CacheKey(realPath, previewSize string, modTime time.Time) string {
	return fmt.Sprintf("%x%x%x", realPath, modTime.Unix(), previewSize)
}

func DelThumbs(ctx context.Context, file iteminfo.ExtendedFileInfo) {
	err := service.fileCache.Delete(ctx, CacheKey(file.RealPath, "small", file.ItemInfo.ModTime))
	if err != nil {
		logger.Debug(fmt.Sprintf("Could not delete small thumbnail: %v", err))
	}
}

func AvailablePreview(file iteminfo.ExtendedFileInfo) bool {
	if strings.HasPrefix(file.Type, "video") && settings.Config.Integrations.Media.FfmpegPath != "" {
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
	return false
}
