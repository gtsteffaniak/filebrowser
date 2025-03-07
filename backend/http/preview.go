package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/img"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

type ImgService interface {
	FormatFromExtension(ext string) (img.Format, error)
	Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...img.Option) error
}

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
}

// previewHandler handles the preview request for images.
// @Summary Get image preview
// @Description Returns a preview image based on the requested path and size.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "File path of the image to preview"
// @Param size query string false "Preview size ('small' or 'large'). Default is based on server config."
// @Success 200 {file} file "Preview image content"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "Unsupported file type for preview"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}
	previewSize := r.URL.Query().Get("size")
	if previewSize != "small" {
		previewSize = "large"
	}
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileInfo, err := files.FileInfoFaster(files.FileOptions{
		Path:   filepath.Join(userscope, path),
		Modify: d.user.Perm.Modify,
		Source: source,
		Expand: true,
	})
	if err != nil {
		return errToStatus(err), err
	}
	if fileInfo.Type == "directory" {
		return http.StatusBadRequest, fmt.Errorf("can't create preview for directory")
	}
	setContentDisposition(w, r, fileInfo.Name)
	if !strings.HasPrefix(fileInfo.Type, "image") {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileInfo.Type)
	}

	if (previewSize == "large" && !config.Server.ResizePreview) ||
		(previewSize == "small" && !config.Server.EnableThumbnails) {
		return rawFileHandler(w, r, fileInfo)
	}

	format, err := imgSvc.FormatFromExtension(filepath.Ext(fileInfo.Name))
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
		resizedImage, err = createPreview(imgSvc, fileCache, fileInfo, previewSize)
		if err != nil {
			return errToStatus(err), err
		}
	}
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, fileInfo.RealPath, fileInfo.ModTime, bytes.NewReader(resizedImage))
	return 0, nil
}

func createPreview(imgSvc ImgService, fileCache FileCache, file files.ExtendedFileInfo, previewSize string) ([]byte, error) {
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
	if err := imgSvc.Resize(context.Background(), fd, width, height, buf, options...); err != nil {
		return nil, err
	}

	go func() {
		cacheKey := previewCacheKey(file.RealPath, previewSize, file.FileInfo.ModTime)
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

func rawFileHandler(w http.ResponseWriter, r *http.Request, file files.ExtendedFileInfo) (int, error) {
	idx := files.GetIndex(file.Source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source not found: %s", file.Source)
	}
	realPath, _, _ := idx.GetRealPath(file.Path)
	fd, err := os.Open(realPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}
