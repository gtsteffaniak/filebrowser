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
			logger.Fatal(fmt.Sprintf("failed to create file cache: %v", err))
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

func GetPreviewForFile(file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
	if data, found, err := service.fileCache.Load(context.Background(), cacheKey); err != nil {
		return nil, fmt.Errorf("failed to load from cache: %w", err)
	} else if found {
		return data, nil
	}
	return service.CreatePreview(file, previewSize)

}

func GeneratePreview(ctx context.Context, in io.Reader, fileName string, outPathPattern string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	var err error
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		outFile, err := os.Create(outPathPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file for image: %w", err)
		}
		defer outFile.Close()

		if err := service.Resize(ctx, in, 640, 360, outFile); err != nil {
			return nil, fmt.Errorf("failed to resize image: %w", err)
		}

	case ".mp4", ".mov", ".avi", ".webm", ".mkv":
		if err := service.GeneratePreviewImages(fileName, outPathPattern, 1); err != nil {
			return nil, fmt.Errorf("failed to generate video preview: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}

	// Read and return the generated preview
	outFile, err := os.Open(outPathPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to open generated preview file: %w", err)
	}
	defer outFile.Close()

	data, err := io.ReadAll(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated preview file: %w", err)
	}

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

	go func() {
		cacheKey := CacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
		if err := s.fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			logger.Error(fmt.Sprintf("failed to cache resized image: %v", err))
		}
	}()

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
