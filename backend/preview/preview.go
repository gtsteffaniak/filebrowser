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

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview/img"
	"github.com/gtsteffaniak/filebrowser/backend/preview/video"
)

var (
	ErrUnsupportedMedia = errors.New("unsupported media type")
)

// Service wraps dependencies for preview generation.
type Service struct {
	imgSvc     *img.Service
	ffmpegPath string
}

// New creates a new preview service.
func New(numImageProcessors int, ffmpegPath string) *Service {
	imgSvc := img.New(numImageProcessors)
	return &Service{
		imgSvc:     imgSvc,
		ffmpegPath: ffmpegPath,
	}
}

// GeneratePreview handles thumbnail generation for both image and video files.
// outPathPattern should be something like "/tmp/preview_%03d.jpg" for videos or a file path for images.
func (s *Service) GeneratePreview(ctx context.Context, in io.Reader, fileName string, outPathPattern string) error {
	ext := strings.ToLower(filepath.Ext(fileName))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		return s.generateImagePreview(ctx, in, outPathPattern)
	case ".mp4", ".mov", ".avi", ".webm", ".mkv":
		return s.generateVideoPreview(fileName, outPathPattern)
	default:
		return ErrUnsupportedMedia
	}
}

// generateImagePreview uses the img package to create and save a thumbnail.
func (s *Service) generateImagePreview(ctx context.Context, in io.Reader, outPath string) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return s.imgSvc.Resize(ctx, in, 640, 360, outFile)
}

// generateVideoPreview uses ffmpeg to extract thumbnails from a video file.
func (s *Service) generateVideoPreview(videoPath, outputPathPattern string) error {
	return video.GeneratePreviewImages(s.ffmpegPath, videoPath, outputPathPattern, 1)
}

func (s *Service) CreatePreview(fileCache FileCache, file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	fd, err := os.Open(file.RealPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	var (
		width   int
		height  int
		options []img.Option
	)

	switch {
	case previewSize == "large":
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == "small":
		width = 256
		height = 256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err := s.imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	go func() {
		cacheKey := previewCacheKey(file.RealPath, previewSize, file.ItemInfo.ModTime)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			logger.Error(fmt.Sprintf("failed to cache resized image: %v", err))
		}
	}()

	return buf.Bytes(), nil
}

// Generates a cache key for the preview image
func previewCacheKey(realPath, previewSize string, modTime time.Time) string {
	return fmt.Sprintf("%x%x%x", realPath, modTime.Unix(), previewSize)
}

func movetoPreview() {
	format, err := previewSvc.FormatFromExtension(filepath.Ext(fileInfo.Name))
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandler(w, r, fileInfo)
	}
	if err != nil {
		return errToStatus(err), err
	}
	cacheKey := previewCacheKey(fileInfo.RealPath, previewSize, fileInfo.ModTime)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return errToStatus(err), err
	}

	if !ok {
		resizedImage, err = previewSvc.CreatePreview(fileCache, fileInfo, previewSize)
		if err != nil {
			return errToStatus(err), err
		}
	}
}
