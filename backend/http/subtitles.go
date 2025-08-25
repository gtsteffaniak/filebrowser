package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// subtitlesHandler handles subtitle extraction requests
// @Summary Extract embedded subtitles
// @Description Extracts embedded subtitle content from video files by stream index and returns raw WebVTT content
// @Tags Subtitles
// @Accept json
// @Produce text/vtt
// @Param path query string true "Path to the video file"
// @Param source query string true "Source name for the desired source"
// @Param index query int true "Stream index for embedded subtitle extraction"
// @Success 200 {string} string "Raw WebVTT subtitle content"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/subtitles [get]
func subtitlesHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	indexParam := r.URL.Query().Get("index")

	if indexParam == "" {
		return http.StatusBadRequest, fmt.Errorf("index parameter is required")
	}

	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	source, err = url.QueryUnescape(source)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}

	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
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

	return extractSubtitle(w, r, realPath, indexParam)
}

func extractSubtitle(w http.ResponseWriter, r *http.Request, videoPath, indexParam string) (int, error) {
	index, err := strconv.Atoi(indexParam)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid index parameter: %v", err)
	}

	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", index),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"-") // output to stdout

	output, err := cmd.Output()
	if err != nil {
		logger.Debugf("ffmpeg subtitle extraction failed for index %d: %v", index, err)
		return http.StatusInternalServerError, fmt.Errorf("subtitle extraction failed: %v", err)
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private")

	http.ServeContent(w, r, fmt.Sprintf("subtitle-%d.vtt", index), 
		time.Now(), bytes.NewReader(output))
	
	return 0, nil
}