package http

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

func viewFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, source string, scopedFileList []string) (int, error) {
	if len(scopedFileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("view supports single file only")
	}
	scopedFilePath := scopedFileList[0]
	displayName := filepath.Base(scopedFilePath)
	if isMediaStreamFile(displayName) {
		return http.StatusForbidden, fmt.Errorf("view endpoint does not support audio or video; use /media/stream")
	}
	return serveSingleFile(w, r, d, source, scopedFilePath, displayName, serveSingleFileOptions{
		forceInline: true,
		rangeOnly:   false,
	})
}

// viewHandler serves inline file content for UI viewing with a valid viewToken.
// @Summary View content of a single non-media file inline
// @Description Returns raw file bytes for inline UI viewing. Requires a viewToken minted by GET /resources. Never counts toward download limits or activity and does not require download permission.
// @Tags Resources
// @Accept json
// @Param source query string true "Source name for the file (required)"
// @Param file query string true "File path"
// @Param viewToken query string true "Opaque view grant token from file metadata"
// @Success 200 {file} file "Raw file content (inline)"
// @Failure 403 {object} map[string]string "Missing or invalid view token"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/view [get]
func viewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if r.URL.Query().Get("archiveToken") != "" || r.URL.Query().Get("algo") != "" {
		return http.StatusForbidden, fmt.Errorf("archives not supported on view endpoint")
	}
	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]
	if len(fileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("view supports single file only")
	}
	token := r.URL.Query().Get("viewToken")
	if token == "" {
		return http.StatusForbidden, fmt.Errorf("view token required")
	}
	cleanPath, err := utils.SanitizePath(fileList[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	if err = validateViewGrant(token, d, source, cleanPath); err != nil {
		return http.StatusForbidden, err
	}

	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopedPath := utils.JoinPathAsUnix(userscope, cleanPath)
	return viewFilesHandler(w, r, d, source, []string{scopedPath})
}

// publicViewHandler serves inline file content from a public share with a valid viewToken.
// @Summary View a single non-media file from a public share inline
// @Description Returns raw file bytes for inline UI viewing on a share link. Requires viewToken from GET /public/api/resources. Does not count toward download limits.
// @Tags Resources
// @Accept json
// @Produce octet-stream
// @Param hash query string true "Share hash for authentication"
// @Param file query string true "File path within the share"
// @Param viewToken query string true "Opaque view grant token from share file metadata"
// @Success 200 {file} file "Raw file content (inline)"
// @Failure 403 {object} map[string]string "Missing or invalid view token"
// @Failure 404 {object} map[string]string "Share or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /public/api/resources/view [get]
func publicViewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("viewing is disabled for upload shares")
	}
	if r.URL.Query().Get("archiveToken") != "" || r.URL.Query().Get("algo") != "" {
		return http.StatusForbidden, fmt.Errorf("archives not supported on view endpoint")
	}
	files := r.URL.Query()["file"]
	if len(files) != 1 {
		return http.StatusForbidden, fmt.Errorf("view supports single file only")
	}
	token := r.URL.Query().Get("viewToken")
	if token == "" {
		return http.StatusForbidden, fmt.Errorf("view token required")
	}
	cleanFile, err := utils.SanitizePath(files[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	sourceInfo, ok := config.Server.SourceMap[d.share.SourcePath]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	if err = validateViewGrant(token, d, sourceInfo.Name, cleanFile); err != nil {
		return http.StatusForbidden, err
	}
	scopedPath := utils.JoinPathAsUnix(d.share.Path, cleanFile)
	status, err := viewFilesHandler(w, r, d, sourceInfo.Name, []string{scopedPath})
	if err != nil {
		if status == http.StatusForbidden {
			return http.StatusForbidden, fmt.Errorf("access denied")
		}
		return status, fmt.Errorf("error viewing file")
	}
	return status, nil
}
