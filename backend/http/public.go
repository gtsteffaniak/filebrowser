package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"

	_ "github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
)

// publicRawHandler serves the raw content of a file, multiple files, or directory via a public share.
// @Summary Download files from a public share
// @Description Downloads raw content from a public share. Supports single files, multiple files, or directories as archives. Enforces download limits (global or per-user) and blocks anonymous users when per-user limits are enabled.
// @Description
// @Description **Multiple Files:**
// @Description - Use repeated query parameters: `?file=file1.txt&file=file2.txt&file=file3.txt`
// @Description - This supports filenames containing commas and special characters
// @Tags Public Shares
// @Accept json
// @Produce octet-stream
// @Param hash query string true "Share hash for authentication"
// @Param file query []string true "File path (can be repeated for multiple files)"
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

	// Get all "file" parameter values (supports repeated params)
	files := r.URL.Query()["file"]
	if len(files) == 0 {
		return http.StatusBadRequest, fmt.Errorf("no files specified")
	}

	// Get the actual source name from the share's source mapping
	sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	actualSourceName := sourceInfo.Name

	// Process each file path
	fileList := []string{}
	for _, file := range files {
		// Rule 1: Validate each file path to prevent path traversal
		cleanFile, err := utils.SanitizeUserPath(file)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
		}

		// Join the share path with the requested path
		filePath := utils.JoinPathAsUnix(d.share.Path, cleanFile)
		fileList = append(fileList, filePath)
	}

	status, err := rawFilesHandler(w, r, d, actualSourceName, fileList)
	if err != nil {
		if err == errors.ErrDownloadNotAllowed {
			return http.StatusForbidden, errors.ErrDownloadNotAllowed
		}
		logger.Errorf("public share handler: error processing filelist: %v with error %v", files, err)
		return status, fmt.Errorf("error processing filelist: %v", files)
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
// @Param content query string false "Include file content if true"
// @Param metadata query string false "Extract audio/video metadata if true"
// @Success 200 {object} iteminfo.FileInfo "File or directory metadata"
// @Failure 403 {object} map[string]string "Share unavailable or access denied"
// @Failure 404 {object} map[string]string "Share not found or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Browsing disabled for upload shares"
// @Router /public/api/resources [get]
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
// @Param path query string true "path within the share to upload to. Must be relative to share root."
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
	// Go automatically decodes query params
	source := config.Server.SourceMap[d.share.Source].Name
	// adjust query params to match resourcePostHandler
	q := r.URL.Query()
	q.Set("source", source)
	q.Set("path", d.IndexPath)
	r.URL.RawQuery = q.Encode()
	status, err := resourcePostHandler(w, r, d)
	if err != nil {
		logger.Errorf("public upload handler: error uploading with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("upload failure occured on backend")
	}
	return status, nil
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
	status, err := previewHelperFunc(w, r, d)
	if err != nil {
		logger.Errorf("public preview handler: error getting preview with error %v", err)
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
	// Go automatically decodes query params
	path := r.URL.Query().Get("path")

	// Rule 1: Validate user-provided path to prevent path traversal
	cleanPath, err := utils.SanitizeUserPath(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %v", err)
	}

	resolvedPath := utils.JoinPathAsUnix(d.share.Path, cleanPath)
	err = files.WriteFile(source, resolvedPath, r.Body)
	// hide the error
	if err != nil {
		logger.Errorf("public put handler: error updating resource with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("an error occurred while updating the resource")
	}
	return http.StatusOK, nil
}

// deprecated -- see publicBulkDeleteHandler
func publicDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.share.AllowDelete {
		return http.StatusForbidden, fmt.Errorf("delete is not allowed for this share")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		FollowSymlinks: true,
		Path:           d.IndexPath,
		Source:         d.share.Source,
	}, store.Access, d.shareUser, store.Share)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("resource not available")
	}
	err = files.DeleteFiles(d.share.Source, fileInfo.RealPath, fileInfo.Type == "directory")
	if err != nil {
		logger.Errorf("public delete handler: error deleting resource with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("an error occured while deleting the resource")
	}
	// delete thumbnails
	preview.DelThumbs(r.Context(), *fileInfo)
	return http.StatusOK, nil
}

// publicBulkDeleteHandler deletes multiple resources from a public share in a single request.
// @Summary Bulk delete resources from public share
// @Description Deletes multiple resources specified in the request body. Returns a list of succeeded and failed deletions.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param items body []BulkDeleteItem true "Array of items to delete, each with source and path"
// @Success 200 {object} BulkDeleteResponse "All resources deleted successfully"
// @Success 207 {object} BulkDeleteResponse "Partial success - some resources deleted, some failed"
// @Failure 400 {object} map[string]string "Bad request - invalid JSON or empty items array"
// @Failure 403 {object} map[string]string "Forbidden - delete not allowed for this share"
// @Failure 500 {object} map[string]string "Internal server error - all deletions failed"
// @Router /public/api/resources/bulk [delete]
func publicBulkDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.share.AllowDelete {
		return http.StatusForbidden, fmt.Errorf("delete is not allowed for this share")
	}
	// hide the error
	status, err := resourceBulkDeleteHandler(w, r, d)
	if err != nil {
		logger.Errorf("public bulk delete handler: error deleting resources with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("an error occurred while processing the request")
	}
	return status, nil
}

// publicPatchHandler performs a patch operation (e.g., move, copy, rename) on resources in a public share.
// @Summary Move, copy, or rename resources in a public share
// @Description Performs move, copy, or rename operations on multiple resources within a public share. All operations are performed atomically.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param request body MoveCopyRequest true "Move/copy request with items and action"
// @Success 200 {object} MoveCopyResponse "All operations completed successfully"
// @Success 207 {object} MoveCopyResponse "Partial success - some operations succeeded, some failed"
// @Failure 400 {object} map[string]string "Bad request - invalid JSON or parameters"
// @Failure 403 {object} map[string]string "Forbidden - modify not allowed for this share"
// @Failure 404 {object} map[string]string "Share or resource not found"
// @Failure 500 {object} MoveCopyResponse "Internal server error"
// @Router /public/api/resources [patch]
func publicPatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.share.AllowModify {
		return http.StatusForbidden, fmt.Errorf("edit permission not allowed for this share")
	}

	// Get the source from the share
	source, err := d.share.GetSourceName()
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("source not available")
	}

	// Replace user with the share creator's user for proper permission checking
	shareCreatedByUser, err := store.Users.Get(d.share.UserID)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("user for share no longer exists")
	}
	d.user = shareCreatedByUser

	// Parse the request body
	var req MoveCopyRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}

	if req.Action == "" {
		return http.StatusBadRequest, fmt.Errorf("action is required (copy, move, or rename)")
	}

	// Transform the request: prepend share path and add source to each item
	// This normalizes the request to look like a regular user request
	// Note: Share paths are absolute, so we don't strip user scope here
	// resourcePatchHandler will skip adding scope for shares
	for i := range req.Items {
		req.Items[i].FromSource = source
		req.Items[i].FromPath = utils.JoinPathAsUnix(d.share.Path, req.Items[i].FromPath)
		req.Items[i].ToSource = source
		req.Items[i].ToPath = utils.JoinPathAsUnix(d.share.Path, req.Items[i].ToPath)
	}
	d.Data = req

	// Call the regular handler (will treat this like a normal user request now)
	status, err := resourcePatchHandler(w, r, d)

	// For shares, we need to sanitize the response to hide internal details
	// The response has already been written by resourcePatchHandler, but we can still return error
	if err != nil {
		logger.Errorf("public patch handler: error processing patch with error %v", err)
		// Obfuscate errors for security
		return http.StatusInternalServerError, fmt.Errorf("an error occurred while processing the request")
	}

	return status, err
}

// getShareImage serves banner or favicon files for shares as resizable previews
// @Summary Get share image (banner or favicon) as preview
// @Description Returns a resizable preview (large size) for the banner or favicon file of a share
// @Tags Public Shares
// @Produce image/jpeg
// @Param hash query string true "Share hash"
// @Param banner query bool false "Request banner file"
// @Param favicon query bool false "Request favicon file"
// @Success 200 {file} file "Preview image content (JPEG)"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 403 {object} map[string]string "Permission denied"
// @Failure 404 {object} map[string]string "Asset not found"
// @Router /public/api/share/image [get]
func getShareImage(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Determine which asset is being requested
	isBanner := r.URL.Query().Get("banner") == "true"
	isFavicon := r.URL.Query().Get("favicon") == "true"

	if !isBanner && !isFavicon {
		return http.StatusBadRequest, fmt.Errorf("either banner or favicon parameter must be true")
	}

	shareCreatedByUser, err := store.Users.Get(d.share.UserID)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("user for share no longer exists")
	}

	sourceName, assetPath, err := getShareImagePartsHelper(d.share, isBanner)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid asset configuration: %v", err)
	}

	// Get file info
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:           assetPath,
		Source:         sourceName,
		Expand:         false,
		Content:        false,
		Metadata:       false,
		ShowHidden:     false,
		FollowSymlinks: true,
	}, store.Access, shareCreatedByUser, store.Share)

	if err != nil {
		logger.Errorf("error accessing share asset: source=%v path=%v error=%v", sourceName, assetPath, err)
		return http.StatusNotFound, fmt.Errorf("asset file not found or not accessible")
	}

	// Ensure it's an image file
	if !strings.HasPrefix(fileInfo.Type, "image/") {
		return http.StatusBadRequest, fmt.Errorf("invalid file type, must be image")
	}

	// Set file info in request context for preview generation
	d.fileInfo = *fileInfo
	q := r.URL.Query()
	if isBanner {
		q.Set("size", "xlarge")
	} else {
		q.Set("size", "small")
	}
	r.URL.RawQuery = q.Encode()

	// Use the preview helper to generate and serve a resized preview
	status, err := previewHelperFunc(w, r, d)
	if err != nil {
		logger.Errorf("error generating preview for share asset: source=%v path=%v error=%v", sourceName, assetPath, err)
		return http.StatusNotFound, fmt.Errorf("preview not available for this asset")
	}

	return status, err
}

func getShareImagePartsHelper(share *share.Link, isBanner bool) (string, string, error) {

	// Get the asset query string from share
	var assetQueryString string
	if isBanner {
		assetQueryString = share.Banner
	} else {
		assetQueryString = share.Favicon
	}

	if assetQueryString == "" {
		return "", "", fmt.Errorf("asset not configured for this share")
	}

	// Parse the query string to extract source and path
	assetParams, err := url.ParseQuery(assetQueryString)
	if err != nil {
		return "", "", fmt.Errorf("invalid asset configuration: %v", err)
	}

	sourceName := assetParams.Get("source")
	assetPath := assetParams.Get("path")

	return sourceName, assetPath, nil
}
