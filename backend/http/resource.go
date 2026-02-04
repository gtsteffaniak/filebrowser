package http

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"
)

// validateMoveOperation checks if a move/rename operation is valid at the HTTP level
// It prevents moving a directory into itself or its subdirectories
func validateMoveOperation(src, dst string, isSrcDir bool) error {
	// Clean and normalize paths
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	// If source is a directory, check if destination is within source
	if isSrcDir {
		// Get the parent directory of the destination
		dstParent := filepath.Dir(dst)

		// Check if destination parent is the source directory or a subdirectory of it
		if strings.HasPrefix(dstParent+string(filepath.Separator), src+string(filepath.Separator)) || dstParent == src {
			return fmt.Errorf("cannot move directory '%s' to a location within itself: '%s'", src, dst)
		}
	}

	// Check if destination parent directory exists
	dstParent := filepath.Dir(dst)
	if dstParent != "." && dstParent != "/" {
		if _, err := os.Stat(dstParent); os.IsNotExist(err) {
			return fmt.Errorf("destination directory does not exist: '%s'", dstParent)
		}
	}

	return nil
}

// resourceGetHandler retrieves information about a resource.
// @Summary Get resource information
// @Description Returns metadata and optionally file contents for a specified resource path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the resource"
// @Param source query string true "Source name for the desired source, default is used if not provided"
// @Param content query string false "Include file content if true"
// @Param metadata query string false "Extract audio/video metadata if true"
// @Param checksum query string false "Optional checksum validation"
// @Success 200 {object} iteminfo.FileInfo "Resource metadata"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [get]
func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	getContent := r.URL.Query().Get("content") == "true"
	getMetadata := r.URL.Query().Get("metadata") == "true"
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		FollowSymlinks:           true,
		Path:                     path,
		Source:                   source,
		Expand:                   true,
		Content:                  getContent,
		Metadata:                 getMetadata,
		ExtractEmbeddedSubtitles: settings.Config.Integrations.Media.ExtractEmbeddedSubtitles,
		ShowHidden:               d.user.ShowHidden,
	}, store.Access, d.user)
	if err != nil {
		return errToStatus(err), err
	}
	if !d.user.Permissions.Download && fileInfo.Content != "" {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to get content, requires download permission")
	}
	if fileInfo.Type == "directory" {
		return renderJSON(w, r, fileInfo)
	}
	if algo := r.URL.Query().Get("checksum"); algo != "" {
		checksum, err := utils.GetChecksum(fileInfo.RealPath, algo)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		fileInfo.Checksums = make(map[string]string)
		fileInfo.Checksums[algo] = checksum
	}
	return renderJSON(w, r, fileInfo)

}

// resourceDeleteHandler deletes a resource at a specified path.
// @Summary Delete a resource
// @Description Deletes a resource located at the specified path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the resource"
// @Param source query string true "Source name for the desired source, default is used if not provided"
// @Success 200 "Resource deleted successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [delete]
func resourceDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {

	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to delete")
	}

	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")

	if path == "/" {
		return http.StatusForbidden, fmt.Errorf("cannot delete your user's root directory")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:       path,
		Source:     source,
		Expand:     false,
		ShowHidden: d.user.ShowHidden,
	}, store.Access, d.user)
	if err != nil {
		return errToStatus(err), err
	}

	// delete thumbnails
	preview.DelThumbs(r.Context(), *fileInfo)

	err = files.DeleteFiles(source, fileInfo.RealPath, fileInfo.Type == "directory")
	if err != nil {
		return errToStatus(err), err
	}
	return http.StatusOK, nil

}

// BulkDeleteItem represents a single item in a bulk delete request
type BulkDeleteItem struct {
	Source  string `json:"source"`
	Path    string `json:"path"`
	Message string `json:"message,omitempty"`
}

// BulkDeleteResponse represents the response from a bulk delete operation
type BulkDeleteResponse struct {
	Succeeded []BulkDeleteItem `json:"succeeded"`
	Failed    []BulkDeleteItem `json:"failed"`
}

// MoveCopyItem represents a single item in a move/copy request
type MoveCopyItem struct {
	FromSource string `json:"fromSource,omitempty"`
	FromPath   string `json:"fromPath,omitempty"`
	ToSource   string `json:"toSource,omitempty"`
	ToPath     string `json:"toPath,omitempty"`
	Message    string `json:"message,omitempty"`
}

// MoveCopyRequest represents a move/copy operation request
type MoveCopyRequest struct {
	Items     []MoveCopyItem `json:"items"`
	Action    string         `json:"action"`    // "copy", "move", or "rename"
	Overwrite bool           `json:"overwrite"` // Overwrite if destination exists
	Rename    bool           `json:"rename"`    // Auto-rename if destination exists
}

// MoveCopyResponse represents the response from a move/copy operation
type MoveCopyResponse struct {
	Succeeded []MoveCopyItem `json:"succeeded"`
	Failed    []MoveCopyItem `json:"failed"`
}

// resourceBulkDeleteHandler deletes multiple resources in a single request.
// @Summary Bulk delete resources
// @Description Deletes multiple resources specified in the request body. Returns a list of succeeded and failed deletions.
// @Tags Resources
// @Accept json
// @Produce json
// @Param items body []BulkDeleteItem true "Array of items to delete, each with source and path"
// @Success 200 {object} BulkDeleteResponse "All resources deleted successfully"
// @Success 207 {object} BulkDeleteResponse "Partial success - some resources deleted, some failed"
// @Failure 400 {object} map[string]string "Bad request - invalid JSON or empty items array"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal server error - all deletions failed"
// @Router /api/resources/bulk/delete [post]
func resourceBulkDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Check permissions - either user delete permission or share delete permission
	if d.share == nil {
		if !d.user.Permissions.Delete {
			return http.StatusForbidden, fmt.Errorf("user is not allowed to delete")
		}
	}

	// Parse request body
	var items []BulkDeleteItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}

	if len(items) == 0 {
		return http.StatusBadRequest, fmt.Errorf("items array cannot be empty")
	}

	response := BulkDeleteResponse{
		Succeeded: make([]BulkDeleteItem, 0),
		Failed:    make([]BulkDeleteItem, 0),
	}

	// Process each item one at a time
	for _, item := range items {
		// Validate item
		if item.Path == "" {
			response.Failed = append(response.Failed, BulkDeleteItem{
				Source:  item.Source,
				Path:    item.Path,
				Message: "path was empty",
			})
			continue
		}

		// Prevent deletion of root
		if item.Path == "/" {
			response.Failed = append(response.Failed, BulkDeleteItem{
				Source:  item.Source,
				Path:    item.Path,
				Message: "cannot delete root directory",
			})
			continue
		}

		if d.share != nil {
			indexPath := utils.JoinPathAsUnix(d.share.Path, item.Path)
			source, err := d.share.GetSourceName()
			if err != nil {
				return http.StatusNotFound, fmt.Errorf("source not available")
			}

			fileInfo, err := files.FileInfoFaster(utils.FileOptions{
				FollowSymlinks: true,
				Path:           indexPath,
				Source:         source,
				ShowHidden:     true,
			}, store.Access, d.user)
			if err != nil {
				return http.StatusNotFound, fmt.Errorf("resource not available")
			}

			// Delete the file/directory
			err = files.DeleteFiles(source, fileInfo.RealPath, fileInfo.Type == "directory")
			if err != nil {
				logger.Errorf("resource bulk delete handler: error deleting file/directory: %v", err)
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: "error deleting file/directory, admin must check the logs",
				})
				continue
			}
			// Delete thumbnails
			preview.DelThumbs(r.Context(), *fileInfo)
		} else {
			// Regular user context - validate source and check user scope
			if item.Source == "" {
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: "source was empty, source is required",
				})
				continue
			}

			// Check user scope for this source
			_, err := d.user.GetScopeForSourceName(item.Source)
			if err != nil {
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: fmt.Sprintf("user does not have access: %v", err),
				})
				continue
			}

			idx := indexing.GetIndex(item.Source)
			if idx == nil {
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: "source not found",
				})
				continue
			}

			// Get file info
			fileInfo, err := files.FileInfoFaster(utils.FileOptions{
				FollowSymlinks: true,
				Path:           idx.MakeIndexPath(item.Path, false),
				Source:         item.Source,
				ShowHidden:     true,
			}, store.Access, d.user)
			if err != nil {
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: err.Error(),
				})
				continue
			}
			err = files.DeleteFiles(item.Source, fileInfo.RealPath, fileInfo.Type == "directory")
			if err != nil {
				response.Failed = append(response.Failed, BulkDeleteItem{
					Source:  item.Source,
					Path:    item.Path,
					Message: err.Error(),
				})
				continue
			}
			preview.DelThumbs(r.Context(), *fileInfo)
		}
		// Success
		response.Succeeded = append(response.Succeeded, item)
	}

	// Determine status code based on results
	statusCode := http.StatusOK
	if len(response.Failed) > 0 {
		statusCode = http.StatusMultiStatus
	}

	return renderJSON(w, r, response, statusCode)
}

// resourcePostHandler creates or uploads a new resource.
// @Summary Create or upload a resource
// @Description Creates a new resource or uploads a file at the specified path. Supports file uploads and directory creation.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "url encoded destination path where to place the files inside the destination source, a directory must end in / to create a directory"
// @Param source query string true "Name for the desired filebrowser destination source name, default is used if not provided"
// @Param override query bool false "Override existing file if true"
// @Param isDir query bool false "Explicitly specify if the resource is a directory"
// @Success 200 "Resource created successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 409 {object} map[string]string "Conflict - Resource already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [post]
func resourcePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	var err error
	accessStore := store.Access
	// if share is not nil, then set accessStore to nil
	if d.share != nil {
		accessStore = nil
	} else {
		// Go automatically decodes query params - no need for QueryUnescape
		if !d.user.Permissions.Create {
			return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
		}
		// Path is now handled by FileInfoFaster which will apply user scope
	}

	// Determine if this is a directory based on isDir query param or trailing slash (for backwards compatibility)
	isDirParam := r.URL.Query().Get("isDir")
	isDir := isDirParam == "true" || strings.HasSuffix(path, "/")
	fileOpts := utils.FileOptions{
		Path:           path,
		Source:         source,
		Expand:         false,
		FollowSymlinks: true,
	}
	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Debugf("source %s not found", source)
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}
	realPath, _, _ := idx.GetRealPath(path)

	// Check access control for the target path
	if accessStore != nil && !accessStore.Permitted(idx.Path, path, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to path %s", path)
	}

	// Check for file/folder conflicts before creation
	if stat, statErr := os.Stat(realPath); statErr == nil {
		// Path exists, check for type conflicts
		existingIsDir := stat.IsDir()
		requestingDir := isDir

		// If type mismatch (file vs folder or folder vs file) and not overriding
		if existingIsDir != requestingDir && r.URL.Query().Get("override") != "true" {
			logger.Debugf("Type conflict detected in chunked: existing is dir=%v, requesting dir=%v at path=%v", existingIsDir, requestingDir, realPath)
			return http.StatusConflict, nil
		}
	}

	// Directories creation on POST.
	if isDir {
		// Get user scope to resolve full index path for directory creation
		var userScope string
		userScope, err = d.user.GetScopeForSourceName(source)
		if err != nil {
			return http.StatusForbidden, err
		}
		fullIndexPath := utils.JoinPathAsUnix(userScope, path)

		// Create a new FileOptions with the full index path
		dirOpts := fileOpts
		dirOpts.Path = fullIndexPath

		err = files.WriteDirectory(dirOpts)
		if err != nil {
			logger.Debugf("error writing directory: %v", err)
			return errToStatus(err), err
		}

		return http.StatusOK, nil
	}

	// Handle Chunked Uploads
	chunkOffsetStr := r.Header.Get("X-File-Chunk-Offset")
	if chunkOffsetStr != "" {
		var offset int64
		offset, err = strconv.ParseInt(chunkOffsetStr, 10, 64)
		if err != nil {
			logger.Debugf("invalid chunk offset: %v", err)
			return http.StatusBadRequest, fmt.Errorf("invalid chunk offset: %v", err)
		}

		var totalSize int64
		totalSizeStr := r.Header.Get("X-File-Total-Size")
		totalSize, err = strconv.ParseInt(totalSizeStr, 10, 64)
		if err != nil {
			logger.Debugf("invalid total size: %v", err)
			return http.StatusBadRequest, fmt.Errorf("invalid total size: %v", err)
		}
		// On the first chunk, check for conflicts or handle override
		if offset == 0 {
			// Check for file/folder conflicts for chunked uploads
			if stat, statErr := os.Stat(realPath); statErr == nil {
				existingIsDir := stat.IsDir()
				requestingDir := false // Files are never directories

				// If type mismatch (existing dir vs requesting file) and not overriding
				if existingIsDir != requestingDir && r.URL.Query().Get("override") != "true" {
					logger.Debugf("Type conflict detected in chunked: existing is dir=%v, requesting dir=%v at path=%v", existingIsDir, requestingDir, realPath)
					return http.StatusConflict, nil
				}
			}

			var fileInfo *iteminfo.ExtendedFileInfo
			fileInfo, err = files.FileInfoFaster(fileOpts, accessStore, d.user)
			if err == nil { // File exists
				if r.URL.Query().Get("override") != "true" {
					logger.Debugf("resource already exists: %v", fileInfo.RealPath)
					return http.StatusConflict, nil
				}
				// If overriding, delete existing thumbnails
				preview.DelThumbs(r.Context(), *fileInfo)
			}
		}

		// Use a temporary file in the cache directory for chunks.
		// Create a unique name for the temporary file to avoid collisions.
		hasher := md5.New()
		hasher.Write([]byte(realPath))
		uploadID := hex.EncodeToString(hasher.Sum(nil))
		tempFilePath := filepath.Join(settings.Config.Server.CacheDir, "uploads", uploadID)

		if err = os.MkdirAll(filepath.Dir(tempFilePath), fileutils.PermDir); err != nil {
			logger.Debugf("could not create temp dir: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("could not create temp dir: %v", err)
		}
		// Create or open the temporary file
		var outFile *os.File
		outFile, err = os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY, fileutils.PermFile)
		if err != nil {
			logger.Debugf("could not open temp file: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("could not open temp file: %v", err)
		}
		defer outFile.Close()

		// Seek to the correct offset to write the chunk
		_, err = outFile.Seek(offset, 0)
		if err != nil {
			logger.Debugf("could not seek in temp file: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("could not seek in temp file: %v", err)
		}

		// Write the request body (the chunk) to the file
		var chunkSize int64
		chunkSize, err = io.Copy(outFile, r.Body)
		if err != nil {
			logger.Debugf("could not write chunk to temp file: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("could not write chunk to temp file: %v", err)
		}
		// check if the file is complete
		if (offset + chunkSize) >= totalSize {
			// close file before moving
			outFile.Close()
			// Move the completed file from the temp location to the final destination
			err = fileutils.MoveFile(tempFilePath, realPath)
			if err != nil {
				logger.Debugf("could not move file from %v to %v: %v", tempFilePath, realPath, err)
				return http.StatusInternalServerError, fmt.Errorf("could not move file from chunked folder to destination: %v", err)
			}
			// Refresh index with user scope
			userScope, scopeErr := d.user.GetScopeForSourceName(source)
			if scopeErr == nil {
				fullIndexPath := utils.JoinPathAsUnix(userScope, fileOpts.Path)
				go files.RefreshIndex(source, fullIndexPath, false, false) //nolint:errcheck
			}
		}

		return http.StatusOK, nil
	}

	fileInfo, err := files.FileInfoFaster(fileOpts, accessStore, d.user)
	if err == nil { // File exists
		if r.URL.Query().Get("override") != "true" {
			logger.Debugf("resource already exists: %v", fileInfo.RealPath)
			return http.StatusConflict, nil
		}
		// If overriding, delete existing thumbnails
		preview.DelThumbs(r.Context(), *fileInfo)
	}

	// Get user scope to resolve full index path for write operation
	userScope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullIndexPath := utils.JoinPathAsUnix(userScope, path)

	err = files.WriteFile(fileOpts.Source, fullIndexPath, r.Body)
	if err != nil {
		logger.Debugf("error writing file: %v", err)
		return errToStatus(err), err
	}
	return http.StatusOK, nil
}

// resourcePutHandler updates an existing file resource.
// @Summary Update a file resource
// @Description Updates an existing file at the specified path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Destination path where to place the files inside the destination source"
// @Param source query string true "Source name for the desired source, default is used if not provided"
// @Success 200 "Resource updated successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 405 {object} map[string]string "Method not allowed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [put]
func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")

	// Only allow PUT for files.
	if strings.HasSuffix(path, "/") {
		return http.StatusMethodNotAllowed, nil
	}
	// Get user scope to resolve full index path for write operation
	userScope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullIndexPath := utils.JoinPathAsUnix(userScope, path)

	// Check access control for the target path
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}
	if store.Access != nil && !store.Access.Permitted(idx.Path, fullIndexPath, d.user.Username) {
		logger.Debugf("user %s denied access to path %s", d.user.Username, fullIndexPath)
		return http.StatusForbidden, fmt.Errorf("access denied to path %s", path)
	}

	err = files.WriteFile(source, utils.JoinPathAsUnix(userScope, path), r.Body)
	return errToStatus(err), err
}

// resourcePatchHandler performs a patch operation (e.g., move, copy, rename) on resources.
// @Summary Move, copy, or rename resources
// @Description Performs move, copy, or rename operations on multiple resources. All operations are performed atomically.
// @Tags Resources
// @Accept json
// @Produce json
// @Param request body MoveCopyRequest true "Move/copy request with items and action"
// @Success 200 {object} MoveCopyResponse "All operations completed successfully"
// @Success 207 {object} MoveCopyResponse "Partial success - some operations succeeded, some failed"
// @Failure 400 {object} map[string]string "Bad request - invalid JSON or parameters"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} MoveCopyResponse "All operations failed"
// @Router /api/resources [patch]
func resourcePatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Modify && d.share == nil {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}

	req, ok := d.Data.(MoveCopyRequest)
	if req.Action == "" || !ok {
		// Parse request body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
		}
	}

	if len(req.Items) == 0 {
		return http.StatusBadRequest, fmt.Errorf("items array cannot be empty")
	}

	if req.Action == "" {
		return http.StatusBadRequest, fmt.Errorf("action is required (copy, move, or rename)")
	}

	response := MoveCopyResponse{
		Succeeded: make([]MoveCopyItem, 0),
		Failed:    make([]MoveCopyItem, 0),
	}

	// Process each item
	for _, item := range req.Items {
		// Validate all fields are provided
		if item.FromSource == "" || item.FromPath == "" || item.ToSource == "" || item.ToPath == "" {
			item.Message = "fromSource, fromPath, toSource, and toPath are required"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Check for root path modifications
		if item.ToPath == "/" || item.FromPath == "/" {
			item.Message = "cannot modify root directory"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Get user scopes for both sources
		// For shares, paths are already absolute, so use empty scope
		userscopeSrc := ""
		userscopeDst := ""
		if d.share == nil {
			var err error
			userscopeSrc, err = d.user.GetScopeForSourceName(item.FromSource)
			if err != nil {
				item.Message = "source not available"
				response.Failed = append(response.Failed, item)
				continue
			}
			userscopeDst, err = d.user.GetScopeForSourceName(item.ToSource)
			if err != nil {
				item.Message = "destination source not available"
				response.Failed = append(response.Failed, item)
				continue
			}
		}

		// Get source index
		srcIdx := indexing.GetIndex(item.FromSource)
		if srcIdx == nil {
			item.Message = "source not found"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Get destination index
		dstIdx := indexing.GetIndex(item.ToSource)
		if dstIdx == nil {
			item.Message = "destination source not found"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Build full index paths for access control
		fullSrcIndexPath := utils.JoinPathAsUnix(userscopeSrc, item.FromPath)
		fullDstIndexPath := utils.JoinPathAsUnix(userscopeDst, item.ToPath)

		// Check access control for both source and destination paths
		if !store.Access.Permitted(srcIdx.Path, fullSrcIndexPath, d.user.Username) {
			item.Message = "access denied to source path"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}
		if !store.Access.Permitted(dstIdx.Path, fullDstIndexPath, d.user.Username) {
			item.Message = "access denied to destination path"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Get real paths
		// Combine user scope with item paths BEFORE calling GetRealPath to avoid double scope application
		fullSrcPath := utils.JoinPathAsUnix(userscopeSrc, item.FromPath)
		realSrc, isSrcDir, err := srcIdx.GetRealPath(fullSrcPath)
		if err != nil {
			logger.Errorf("could not resolve source path: %v, item.FromPath: %v", err, item.FromPath)
			item.Message = "could not resolve source path"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}

		// Check destination parent directory exists
		dstParentPath := filepath.Dir(item.ToPath)
		fullDstParentPath := utils.JoinPathAsUnix(userscopeDst, dstParentPath)
		parentDir, _, err := dstIdx.GetRealPath(fullDstParentPath)
		if err != nil {
			item.Message = "destination directory does not exist"
			if d.share != nil {
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			response.Failed = append(response.Failed, item)
			continue
		}
		realDest := parentDir + "/" + filepath.Base(item.ToPath)

		// Auto-rename if requested
		if req.Rename {
			realDest = addVersionSuffix(realDest)
		}

		// Validate move/rename operation to prevent circular references
		if req.Action == "rename" || req.Action == "move" {
			if err = validateMoveOperation(realSrc, realDest, isSrcDir); err != nil {
				item.Message = "invalid move operation, circular reference"
				if d.share != nil {
					response.Failed = append(response.Failed, MoveCopyItem{
						Message: item.Message,
					})
					continue
				}
				response.Failed = append(response.Failed, item)
				continue
			}
		}

		// Perform the action
		err = patchAction(r.Context(), patchActionParams{
			action:   req.Action,
			srcIndex: item.FromSource,
			dstIndex: item.ToSource,
			src:      realSrc,
			dst:      realDest,
			d:        d,
			isSrcDir: isSrcDir,
		})
		if err != nil {
			logger.Errorf("Could not run patch action. src=%v dst=%v err=%v", realSrc, realDest, err)
			if d.share != nil {
				item.Message = "could not run patch action"
				response.Failed = append(response.Failed, MoveCopyItem{
					Message: item.Message,
				})
				continue
			}
			item.Message = err.Error()
			response.Failed = append(response.Failed, item)
			continue
		}

		// Success
		response.Succeeded = append(response.Succeeded, item)
	}

	if len(response.Failed) == 0 && len(response.Succeeded) == 0 {
		response.Failed = append(response.Failed, MoveCopyItem{
			Message: "no operations performed",
		})
	}

	// For shares, sanitize the response to only include messages (hide paths)
	if d.share != nil {
		sanitizedFailed := make([]MoveCopyItem, len(response.Failed))
		for i, item := range response.Failed {
			sanitizedFailed[i] = MoveCopyItem{
				Message: item.Message,
			}
		}
		response.Failed = sanitizedFailed

		// Clear succeeded items details for shares (only keep count implicitly via array length)
		sanitizedSucceeded := make([]MoveCopyItem, len(response.Succeeded))
		response.Succeeded = sanitizedSucceeded
	}

	// Determine status code based on results
	statusCode := http.StatusOK
	if len(response.Failed) > 0 && len(response.Succeeded) == 0 {
		// All operations failed - return 500 error
		statusCode = http.StatusInternalServerError
	} else if len(response.Failed) > 0 && len(response.Succeeded) > 0 {
		// Some succeeded, some failed - return 207 multi-status
		statusCode = http.StatusMultiStatus
	}
	// If all succeeded, statusCode remains 200 OK
	return renderJSON(w, r, response, statusCode)
}

func addVersionSuffix(source string) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for {
		if _, err := os.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}
	return source
}

type patchActionParams struct {
	action   string
	srcIndex string
	dstIndex string
	src      string
	dst      string
	d        *requestContext
	isSrcDir bool
}

func patchAction(ctx context.Context, params patchActionParams) error {
	switch params.action {
	case "copy":
		err := files.CopyResource(params.isSrcDir, params.srcIndex, params.dstIndex, params.src, params.dst)
		return err
	case "rename", "move":
		idx := indexing.GetIndex(params.srcIndex)
		srcPath := idx.MakeIndexPath(params.src, params.isSrcDir)
		userScope := ""
		userScope, _ = params.d.user.GetScopeForSourceName(params.srcIndex)
		if userScope != "" && userScope != "/" {
			// Strip the user scope from srcPath so FileInfoFaster doesn't double it
			srcPath = strings.TrimPrefix(srcPath, userScope)
		}

		fileInfo, err := files.FileInfoFaster(utils.FileOptions{
			FollowSymlinks: true,
			Path:           srcPath,
			Source:         params.srcIndex,
			IsDir:          params.isSrcDir,
			ShowHidden:     params.d.user.ShowHidden,
		}, store.Access, params.d.user)

		if err != nil {
			return err
		}

		// delete thumbnails
		preview.DelThumbs(ctx, *fileInfo)
		return files.MoveResource(params.isSrcDir, params.srcIndex, params.dstIndex, params.src, params.dst, store.Share, store.Access)
	default:
		return fmt.Errorf("unsupported action %s: %w", params.action, errors.ErrInvalidRequestParams)
	}
}

func inspectIndex(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	isNotDir := r.URL.Query().Get("isDir") == "false" // default to isDir true
	index := indexing.GetIndex(source)
	if index == nil {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	info, _ := index.GetReducedMetadata(path, !isNotDir)
	renderJSON(w, r, info) // nolint:errcheck
}

func mockData(w http.ResponseWriter, r *http.Request) {
	d := r.URL.Query().Get("numDirs")
	f := r.URL.Query().Get("numFiles")
	NumDirs, err := strconv.Atoi(d)
	numFiles, err2 := strconv.Atoi(f)
	if err != nil || err2 != nil {
		return
	}
	mockDir := indexing.CreateMockData(NumDirs, numFiles)
	renderJSON(w, r, mockDir) // nolint:errcheck
}
