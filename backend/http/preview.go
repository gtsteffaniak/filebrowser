package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/img"
)

type PreviewSize int

type ImgService interface {
	FormatFromExtension(ext string) (img.Format, error)
	Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...img.Option) error
}

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
}

// Handles the preview request for images
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
	}

	// Parse the URL path
	parts := strings.SplitN(r.URL.Path, "/", 4) // Splitting by "/"
	if len(parts) < 4 {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	// Extract size and path from URL
	previewSize, err := ParsePreviewSize(parts[2]) // Assuming "size" is the third part
	if err != nil {
		return http.StatusBadRequest, err
	}

	file, err := files.FileInfoFaster(files.FileOptions{
		Path:       "/" + parts[3], // Assuming "path" is the third part
		Modify:     d.user.Perm.Modify,
		Expand:     true,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
	})

	if err != nil {
		return errToStatus(err), err
	}

	setContentDisposition(w, r, file)
	if file.Type != "image" {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", file.Type)
	}

	if (previewSize == PreviewSizeBig && !config.Server.ResizePreview) ||
		(previewSize == PreviewSizeThumb && !config.Server.EnableThumbnails) {
		return rawFileHandler(w, r, file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return rawFileHandler(w, r, file)
	}
	if err != nil {
		return errToStatus(err), err
	}

	cacheKey := previewCacheKey(file, previewSize)
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return errToStatus(err), err
	}
	if !ok {
		resizedImage, err = createPreview(imgSvc, fileCache, file, previewSize)
		if err != nil {
			return errToStatus(err), err
		}
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, bytes.NewReader(resizedImage))

	return 0, nil
}

// Creates a preview image based on size
func createPreview(imgSvc ImgService, fileCache FileCache, file *files.FileInfo, previewSize PreviewSize) ([]byte, error) {
	fd, err := os.Open(file.Path)
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
	case previewSize == PreviewSizeBig:
		width = 1080
		height = 1080
		options = append(options, img.WithMode(img.ResizeModeFit), img.WithQuality(img.QualityMedium))
	case previewSize == PreviewSizeThumb:
		width = 256
		height = 256
		options = append(options, img.WithMode(img.ResizeModeFill), img.WithQuality(img.QualityLow), img.WithFormat(img.FormatJpeg))
	default:
		return nil, img.ErrUnsupportedFormat
	}

	buf := &bytes.Buffer{}
	if err := imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	go func() {
		cacheKey := previewCacheKey(file, previewSize)
		if err := fileCache.Store(context.Background(), cacheKey, buf.Bytes()); err != nil {
			fmt.Printf("failed to cache resized image: %v", err)
		}
	}()

	return buf.Bytes(), nil
}

// Generates a cache key for the preview image
func previewCacheKey(f *files.FileInfo, previewSize PreviewSize) string {
	return fmt.Sprintf("%x%x%x", f.RealPath(), f.ModTime.Unix(), previewSize)
}
