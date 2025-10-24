package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"

	_ "github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
)

// publicRawHandler serves the raw content of a file, multiple files, or directory via a public share.
// @Summary Download files from a public share
// @Description Downloads raw content from a public share. Supports single files, multiple files, or directories as archives. Enforces download limits (global or per-user) and blocks anonymous users when per-user limits are enabled.
// @Tags Public Shares
// @Accept json
// @Produce octet-stream
// @Param hash query string true "Share hash for authentication"
// @Param files query string true "Files to download in format: 'source::path||source::path'. Example: '/file1||/folder/file2'"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 400 {object} map[string]string "Invalid request path or encoding"
// @Failure 403 {object} map[string]string "Download limit reached, anonymous access blocked, or share unavailable"
// @Failure 404 {object} map[string]string "Share not found or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Downloads disabled for upload shares"
// @Router /public/api/raw [get]
func publicRawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("downloads are disabled for upload shares")
	}

	// Check DisableDownload permission for normal shares
	if d.share.DisableDownload {
		return http.StatusForbidden, fmt.Errorf("downloads are not allowed for this share")
	}

	// Check global download limit (if not using per-user limits)
	if !d.share.PerUserDownloadLimit && d.share.DownloadsLimit > 0 && d.share.Downloads >= d.share.DownloadsLimit {
		return http.StatusForbidden, fmt.Errorf("share downloads limit reached")
	}

	// Check per-user download limit
	if d.share.PerUserDownloadLimit {
		// Block anonymous users
		if d.user.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("anonymous downloads are not allowed with per-user limits")
		}
		// Check if user has reached their limit
		if d.share.HasReachedUserLimit(d.user.Username) {
			return http.StatusForbidden, fmt.Errorf("user download limit reached for this share")
		}
	}

	d.share.Mu.Lock()
	d.share.Downloads++
	d.share.Mu.Unlock()

	// Track per-user download if enabled
	if d.share.PerUserDownloadLimit {
		d.share.IncrementUserDownload(d.user.Username)
	}
	encodedFiles := r.URL.Query().Get("files")

	// Decode the URL-encoded path - use PathUnescape to preserve + as literal character
	f, err := url.PathUnescape(encodedFiles)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	// Get the actual source name from the share's source mapping
	sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	actualSourceName := sourceInfo.Name

	fileList := []string{}
	for _, file := range strings.Split(f, "||") {
		// Check if file already contains source prefix (source::path format)
		if strings.Contains(file, "::") {
			splitFile := strings.SplitN(file, "::", 2)
			if len(splitFile) == 2 {
				source := splitFile[0]
				path := splitFile[1]
				// Join the share path with the requested path
				filePath := utils.JoinPathAsUnix(d.share.Path, path)
				fileList = append(fileList, source+"::"+filePath)
			} else {
				// Fallback: treat as plain path
				filePath := utils.JoinPathAsUnix(d.share.Path, file)
				fileList = append(fileList, actualSourceName+"::"+filePath)
			}
		} else {
			// Plain path without source prefix - use the actual source name from share
			filePath := utils.JoinPathAsUnix(d.share.Path, file)
			fileList = append(fileList, actualSourceName+"::"+filePath)
		}
	}

	var status int
	status, err = rawFilesHandler(w, r, d, fileList)
	if err != nil {
		if err == errors.ErrDownloadNotAllowed {
			return http.StatusForbidden, errors.ErrDownloadNotAllowed
		}
		logger.Errorf("public share handler: error processing filelist: '%v' with error %v", f, err)
		return status, fmt.Errorf("error processing filelist: %v", f)
	}
	return status, nil
}

// publicShareHandler returns file or directory information from a public share.
// @Summary Get file/directory information from a public share
// @Description Returns metadata for files or directories accessible via a public share link. Browsing is disabled for upload-only shares.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param path query string false "Path within the share to retrieve information for. Defaults to share root."
// @Success 200 {object} iteminfo.FileInfo "File or directory metadata"
// @Failure 403 {object} map[string]string "Share unavailable or access denied"
// @Failure 404 {object} map[string]string "Share not found or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Browsing disabled for upload shares"
// @Router /public/api/share [get]
func publicGetResourceHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("browsing is disabled for upload shares")
	}
	return renderJSON(w, r, d.fileInfo)
}

// publicUploadHandler processes file uploads to a public upload share.
// @Summary Upload files to a public upload share
// @Description Handles file and directory uploads to an upload-only public share. Supports chunked uploads, conflict resolution (override), and directory creation.
// @Tags Public Shares
// @Accept multipart/form-data
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param targetPath query string true "Target path within the share to upload to. Must be relative to share root."
// @Param override query bool false "If true, overwrite existing files/folders. Defaults to false."
// @Param action query string false "Upload action: 'override' to replace files, 'rename' to auto-rename"
// @Param file formData file true "File to upload"
// @Success 200 {object} map[string]string "Upload successful"
// @Failure 400 {object} map[string]string "Invalid request or parameters"
// @Failure 403 {object} map[string]string "Share unavailable or upload not allowed"
// @Failure 404 {object} map[string]string "Share not found"
// @Failure 409 {object} map[string]string "File or directory already exists (conflict)"
// @Failure 500 {object} map[string]string "Internal server error during upload"
// @Failure 501 {object} map[string]string "Uploading disabled for non-upload shares"
// @Router /public/api/resources [post]
func publicUploadHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType != "upload" && !d.share.AllowCreate {
		return http.StatusForbidden, fmt.Errorf("uploading is disabled for this share")
	}

	if !d.share.AllowReplacements && r.URL.Query().Get("action") == "override" {
		return http.StatusForbidden, fmt.Errorf("cannot overwrite files for this share")
	}
	path, err := url.QueryUnescape(r.URL.Query().Get("targetPath"))
	if err != nil {
		logger.Debugf("invalid path encoding: %v", err)
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	fullPath := filepath.Join(d.share.Path, path)
	source := config.Server.SourceMap[d.share.Source].Name
	// adjust query params to match resourcePostHandler
	q := r.URL.Query()
	q.Set("source", source)
	q.Set("path", fullPath)
	r.URL.RawQuery = q.Encode()
	status, err := resourcePostHandler(w, r, d)
	if err != nil {
		logger.Errorf("public upload handler: error uploading with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("upload failure occured on backend")
	}
	return status, err
}

// health godoc
// @Summary Health Check
// @Schemes
// @Description Returns the health status of the API.
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} HttpResponse "successful health check response"
// @Router /health [get]
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HttpResponse{Message: "ok"}    // Create response with status "ok"
	err := json.NewEncoder(w).Encode(response) // Encode the response into JSON
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// publicPreviewHandler handles the preview request for images from public shares.
// @Summary Get image/video preview from a public share
// @Description Returns a preview (thumbnail) for images or videos accessible via a public share. Preview generation can be disabled globally or per-share. Not available for upload-only shares.
// @Tags Public Shares
// @Accept json
// @Produce image/jpeg
// @Param hash query string true "Share hash for authentication"
// @Param path query string true "File path within the share to preview"
// @Param size query string false "Preview size: 'small' or 'large'. Default is based on server config."
// @Success 200 {file} file "Preview image content (JPEG)"
// @Failure 403 {object} map[string]string "Share unavailable or access denied"
// @Failure 404 {object} map[string]string "File not found or preview not available"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Previews disabled globally, for this share, or for upload shares"
// @Router /public/api/preview [get]
func publicPreviewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Server.DisablePreviews || d.share.DisableThumbnails {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}

	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled for upload shares")
	}

	// Restore source name from share for preview generation
	// The middleware clears file.Source for security, but we need it for index lookups
	source, err := d.share.GetSourceName()
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("source not available")
	}
	fileInfo, err := FileInfoFasterFunc(utils.FileOptions{
		Path:     utils.JoinPathAsUnix(d.share.Path, d.fileInfo.Path),
		Source:   source,
		Metadata: true,
	}, nil)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("resource not available")
	}
	d.fileInfo = *fileInfo
	status, err := previewHelperFunc(w, r, d)
	if err != nil {
		// Obfuscate errors for shares to prevent information leakage
		return http.StatusNotFound, fmt.Errorf("preview not available for this item")
	}
	return status, err
}

// publicPutHandler handles the PUT request for a public share.
// @Summary Update a file in a public share
// @Description Updates the content of a file in a public share.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param path query string true "Path to the file to update"
// @Param content body string true "New content for the file"
// @Success 200 {object} map[string]string "File updated successfully"
// @Failure 400 {object} map[string]string "Invalid request or parameters"
// @Failure 403 {object} map[string]string "Share unavailable or update not allowed"
// @Failure 404 {object} map[string]string "Share not found or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /public/api/resources [put]
func publicPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// update path to be the source path
	if !d.share.AllowModify {
		return http.StatusForbidden, fmt.Errorf("create is not allowed for this share")
	}
	source, err := d.share.GetSourceName()
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("source not available")
	}

	if !d.share.AllowModify {
		return http.StatusForbidden, fmt.Errorf("edit permission not allowed for this share")
	}
	path := r.URL.Query().Get("path")

	// Decode the URL-encoded path
	path, err = url.QueryUnescape(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	resolvedPath := utils.JoinPathAsUnix(d.share.Path, path)
	err = files.WriteFile(source, resolvedPath, r.Body)
	return errToStatus(err), err
}

func publicDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.share.AllowDelete {
		return http.StatusForbidden, fmt.Errorf("delete is not allowed for this share")
	}

	// TODO source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")

	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	indexPath := utils.JoinPathAsUnix(d.share.Path, path)
	source, err := d.share.GetSourceName()
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("source not available")
	}

	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Username: d.user.Username,
		Path:     indexPath,
		Source:   source,
		Expand:   false,
	}, nil)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("resource not available")
	}

	// delete thumbnails
	preview.DelThumbs(r.Context(), *fileInfo)

	err = files.DeleteFiles(source, fileInfo.RealPath, filepath.Dir(fileInfo.RealPath))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("an error occured while deleting the resource")
	}
	return http.StatusOK, nil
}

func publicPatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.share.AllowModify {
		return http.StatusForbidden, fmt.Errorf("edit permission not allowed for this share")
	}
	// The middleware clears file.Source for security, but we need it for index lookups
	source, err := d.share.GetSourceName()
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("source not available")
	}
	action := r.URL.Query().Get("action")
	encodedFrom := r.URL.Query().Get("from")
	// Decode the URL-encoded path
	src, err := url.QueryUnescape(encodedFrom)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	dst := r.URL.Query().Get("destination")
	dst, err = url.QueryUnescape(dst)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source for share not found")
	}

	// get full paths for both locations
	srcFullPath := utils.JoinPathAsUnix(d.share.Path, src)
	if srcFullPath == "/" {
		return http.StatusForbidden, fmt.Errorf("an error occured accessing the share")
	}
	dstFullPath := utils.JoinPathAsUnix(d.share.Path, dst)
	if dstFullPath == "/" {
		return http.StatusForbidden, fmt.Errorf("an error occured accessing the share")
	}

	// get real source and isDir
	realSrc, isSrcDir, err := idx.GetRealPath(srcFullPath)
	if err != nil {
		return http.StatusNotFound, err
	}
	// get real destination and isDir parent
	parentRealDest, _, err := idx.GetRealPath(filepath.Dir(dstFullPath))
	if err != nil {
		return http.StatusNotFound, err
	}
	dstFullPath = parentRealDest + "/" + filepath.Base(dst)
	rename := r.URL.Query().Get("rename") == "true"
	if rename {
		dstFullPath = addVersionSuffix(dstFullPath)
	}
	// Validate move/rename operation to prevent circular references
	if action == "rename" || action == "move" {
		err = validateMoveOperation(realSrc, dstFullPath, isSrcDir)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid move or rename operation")
		}
	}
	err = patchAction(r.Context(), patchActionParams{
		action:   action,
		srcIndex: source,
		dstIndex: source,
		src:      realSrc,
		dst:      dstFullPath,
		d:        d,
		isSrcDir: isSrcDir,
	})
	if err != nil {
		logger.Debugf("Could not run patch action. src=%v dst=%v err=%v", realSrc, dstFullPath, err)
		return http.StatusInternalServerError, fmt.Errorf("an error occured while processing the request")
	}
	return http.StatusOK, nil
}
