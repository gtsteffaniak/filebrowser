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

func GeneratePreview(ctx context.Context, in io.Reader, fileName string, outPathPattern string) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		f, err := os.Create(outPathPattern)
		if err != nil {
			return err
		}
		defer f.Close()
		return service.Resize(ctx, in, 640, 360, f)
	case ".mp4", ".mov", ".avi", ".webm", ".mkv":
		return service.GeneratePreviewImages(fileName, outPathPattern, 1)
	default:
		return ErrUnsupportedMedia
	}
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
		cacheKey := previewCacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
		if err := s.fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			logger.Error(fmt.Sprintf("failed to cache resized image: %v", err))
		}
	}()

	return buf.Bytes(), nil
}

func previewCacheKey(realPath, previewSize string, modTime time.Time) string {
	return fmt.Sprintf("%x%x%x", realPath, modTime.Unix(), previewSize)
}
