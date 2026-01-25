package http

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

// subtitlesHandler handles subtitle extraction requests
// @Summary Extract embedded subtitles
// @Description Extracts embedded subtitle content from video files by stream index and returns raw WebVTT content
// @Tags Subtitles
// @Accept json
// @Produce text/vtt
// @Param path query string true "Index path to the video file"
// @Param source query string true "Source name for the desired source"
// @Param index query int false "Stream index for embedded subtitle extraction, defaults to 0"
// @Success 200 {string} string "Raw WebVTT subtitle content"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/media/subtitles [get]
func subtitlesHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	indexParam := r.URL.Query().Get("index")

	if indexParam == "" {
		indexParam = "0" // default to first subtitle stream
	}

	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}
	realPath, _, err := idx.GetRealPath(userscope, path)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("file not found: %v", err)
	}
	metadata, exists := idx.GetMetadataInfo(userscope, true, false)
	if !exists {
		return http.StatusNotFound, fmt.Errorf("file not found: %v", err)
	}

	index, err := strconv.Atoi(indexParam)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid index parameter: %v", err)
	}
	parentDir := filepath.Dir(realPath)
	subtitle, err := ffmpeg.ExtractSingleSubtitle(realPath, parentDir, index, metadata.ModTime)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to extract subtitle: %v", err)
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, fmt.Sprintf("%s-%d.vtt", subtitle.Name, index),
		time.Now(), bytes.NewReader([]byte(subtitle.Content)))
	return http.StatusOK, nil
}
