package http

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

// subtitlesHandler handles subtitle requests for both external files and embedded streams
// @Summary Get subtitle content
// @Description Returns raw subtitle content from external files or embedded streams
// @Tags Resources
// @Accept json
// @Produce text/plain
// @Param path query string true "Index path to the video file"
// @Param source query string true "Source name for the desired source"
// @Param name query string true "Subtitle track name (filename for external, descriptive name for embedded)"
// @Param embedded query bool false "Whether this is an embedded stream (true) or external file (false), defaults to false"
// @Success 200 {string} string "Raw subtitle content in original format"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/media/subtitles [get]
func subtitlesHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	name := r.URL.Query().Get("name")
	embedded := r.URL.Query().Get("embedded") == "true"

	if name == "" {
		return http.StatusBadRequest, fmt.Errorf("name parameter is required")
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

	parentDir := filepath.Dir(realPath)
	var content string

	if !embedded {
		content, err = utils.GetSubtitleSidecarContent(filepath.Join(parentDir, name))
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to get subtitle sidecar content: %v", err)
		}
	} else {
		// For embedded subtitles, we need to find the stream index by name
		// Get file modification time for caching
		fileInfo, err := os.Stat(realPath)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to stat file: %v", err)
		}

		// Detect embedded subtitles
		embeddedSubs := ffmpeg.DetectEmbeddedSubtitles(realPath, fileInfo.ModTime())

		// Find the subtitle track by name
		var streamIndex *int
		for _, sub := range embeddedSubs {
			if sub.Name == name {
				streamIndex = sub.Index
				break
			}
		}

		if streamIndex == nil {
			return http.StatusNotFound, fmt.Errorf("embedded subtitle track '%s' not found", name)
		}

		content, err = ffmpeg.ExtractSubtitleContent(realPath, *streamIndex)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to extract embedded subtitle: %v", err)
		}
	}

	// Return raw content with appropriate content type
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, name, time.Now(), bytes.NewReader([]byte(content)))
	return http.StatusOK, nil
}

// metadataHandler returns the same directory resource shape as GET /api/resources with metadata enabled,
// for client-side patching after a fast listing load.
// @Summary Directory with media metadata
// @Description Same ExtendedFileInfo as resources GET with metadata=true (typically used for directories).
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the directory"
// @Param source query string true "Source name"
// @Success 200 {object} iteminfo.ExtendedFileInfo
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/media/metadata [get]
func metadataHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		FollowSymlinks:           true,
		Path:                     path,
		Source:                   source,
		Expand:                   true,
		Content:                  false,
		Metadata:                 true,
		ExtractEmbeddedSubtitles: settings.Config.Integrations.Media.ExtractEmbeddedSubtitles,
		ShowHidden:               d.user.ShowHidden,
		SkipExtendedAttrs:        false,
		ShowSharedAttr:           true,
	}, store.Access, d.user, store.Share)
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, fileInfo)
}

// publicMetadataHandler is the share-link variant of metadataHandler.
// @Summary Directory with media metadata (public share)
// @Tags Shares
// @Produce json
// @Param hash query string true "Share hash"
// @Param path query string false "Path within the share"
// @Success 200 {object} iteminfo.ExtendedFileInfo
// @Router /public/api/media/metadata [get]
func publicMetadataHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("browsing is disabled for upload shares")
	}
	path := r.URL.Query().Get("path")
	sourceCfg, ok := config.Server.SourceMap[d.share.Source]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("source not found")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:                     d.IndexPath,
		Source:                   sourceCfg.Name,
		Expand:                   true,
		Content:                  false,
		Metadata:                 true,
		ExtractEmbeddedSubtitles: settings.Config.Integrations.Media.ExtractEmbeddedSubtitles && d.share.ExtractEmbeddedSubtitles,
		ShowHidden:               d.share.ShowHidden,
		FollowSymlinks:           false,
	}, store.Access, d.shareUser, store.Share)
	if err != nil {
		return errToStatus(err), err
	}
	fileInfo.Path = utils.AddTrailingSlashIfNotExists(path)
	return renderJSON(w, r, fileInfo)
}
