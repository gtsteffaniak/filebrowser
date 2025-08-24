package http

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
// @Param checksum query string false "Optional checksum validation"
// @Success 200 {object} iteminfo.FileInfo "Resource metadata"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [get]
func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	// Decode the URL-encoded path
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
	scopePath := utils.JoinPathAsUnix(userscope, path)
	getContent := r.URL.Query().Get("content") == "true"
	if d.share != nil && d.share.DisableFileViewer {
		getContent = false
	}
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     scopePath,
		Modify:   d.user.Permissions.Modify,
		Source:   source,
		Expand:   true,
		Content:  getContent,
	})
	if err != nil {
		return errToStatus(err), err
	}
	if userscope != "/" {
		fileInfo.Path = strings.TrimPrefix(fileInfo.Path, userscope)
	}
	if fileInfo.Path == "" {
		fileInfo.Path = "/"
	}
	if fileInfo.Type == "directory" {
		return renderJSON(w, r, fileInfo)
	}
	if algo := r.URL.Query().Get("checksum"); algo != "" {
		idx := indexing.GetIndex(source)
		if idx == nil {
			return http.StatusNotFound, fmt.Errorf("source %s not found", source)
		}
		realPath, _, _ := idx.GetRealPath(userscope, path)
		checksums, err := files.GetChecksum(realPath, algo)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		fileInfo.Checksums = checksums
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
	// TODO source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	var err error
	// decode url encoded source name
	source, err = url.QueryUnescape(source)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if path == "/" {
		return http.StatusForbidden, nil
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     utils.JoinPathAsUnix(userscope, path),
		Source:   source,
		Modify:   d.user.Permissions.Modify,
		Expand:   false,
	})
	if err != nil {
		return errToStatus(err), err
	}

	// delete thumbnails
	preview.DelThumbs(r.Context(), fileInfo)

	err = files.DeleteFiles(source, fileInfo.RealPath, filepath.Dir(fileInfo.RealPath))
	if err != nil {
		return errToStatus(err), err
	}
	return http.StatusOK, nil

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
	// decode url encoded source name
	source, err = url.QueryUnescape(source)
	if err != nil {
		logger.Debugf("invalid source encoding: %v", err)
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}
	path, err = url.QueryUnescape(path)
	if err != nil {
		logger.Debugf("invalid path encoding: %v", err)
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if !d.user.Permissions.Modify {
		logger.Debugf("user is not allowed to create or modify")
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		logger.Debugf("error getting scope from source name: %v", err)
		return http.StatusForbidden, err
	}
	fileOpts := iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     utils.JoinPathAsUnix(userscope, path),
		Source:   source,
		Modify:   d.user.Permissions.Modify,
		Expand:   false,
	}
	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Debugf("source %s not found", source)
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}
	realPath, _, _ := idx.GetRealPath(userscope, path)
	isDir := strings.HasSuffix(path, "/")
	// Directories creation on POST.
	if isDir {
		err = files.WriteDirectory(fileOpts)
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
			var fileInfo iteminfo.ExtendedFileInfo
			fileInfo, err = files.FileInfoFaster(fileOpts)
			if err == nil { // File exists
				if r.URL.Query().Get("override") != "true" {
					logger.Debugf("resource already exists: %v", fileInfo.RealPath)
					logger.Debugf("Resource already exists: %v", fileInfo.RealPath)
					return http.StatusConflict, nil
				}
				// If overriding, delete existing thumbnails
				preview.DelThumbs(r.Context(), fileInfo)
			}
		}

		// Use a temporary file in the cache directory for chunks.
		// Create a unique name for the temporary file to avoid collisions.
		hasher := md5.New() //nolint:gosec
		hasher.Write([]byte(realPath))
		uploadID := hex.EncodeToString(hasher.Sum(nil))
		tempFilePath := filepath.Join(settings.Config.Server.CacheDir, "uploads", uploadID)

		if err = os.MkdirAll(filepath.Dir(tempFilePath), 0755); err != nil {
			logger.Debugf("could not create temp dir: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("could not create temp dir: %v", err)
		}
		// Create or open the temporary file
		var outFile *os.File
		outFile, err = os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY, 0644)
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
				logger.Debugf("could not move temp file to destination: %v", err)
				return http.StatusInternalServerError, fmt.Errorf("could not move temp file to destination: %v", err)
			}
			go files.RefreshIndex(source, realPath, false) //nolint:errcheck
		}

		return http.StatusOK, nil
	}

	logger.Debugf("non-chunked upload logic for %s", realPath)
	// Non-chunked upload logic (original code)
	fileInfo, err := files.FileInfoFaster(fileOpts)
	if err == nil {
		if r.URL.Query().Get("override") != "true" {
			logger.Debugf("Resource already exists: %v", fileInfo.RealPath)
			return http.StatusConflict, nil
		}

		preview.DelThumbs(r.Context(), fileInfo)
	}
	err = files.WriteFile(fileOpts, r.Body)
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
	var err error
	// decode url encoded source name
	source, err = url.QueryUnescape(source)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}
	if !d.user.Permissions.Modify {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	encodedPath := r.URL.Query().Get("path")

	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	// Only allow PUT for files.
	if strings.HasSuffix(path, "/") {
		return http.StatusMethodNotAllowed, nil
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileOpts := iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     utils.JoinPathAsUnix(userscope, path),
		Source:   source,
		Modify:   d.user.Permissions.Modify,
		Expand:   false,
	}
	err = files.WriteFile(fileOpts, r.Body)
	return errToStatus(err), err
}

// resourcePatchHandler performs a patch operation (e.g., move, rename) on a resource.
// @Summary Patch resource (move/rename)
// @Description Moves or renames a resource to a new destination.
// @Tags Resources
// @Accept json
// @Produce json
// @Param from query string true "Path from resource in <source_name>::<index_path> format"
// @Param destination query string true "Destination path for the resource"
// @Param action query string true "Action to perform (copy, rename)"
// @Param overwrite query bool false "Overwrite if destination exists"
// @Param rename query bool false "Rename if destination exists"
// @Success 200 "Resource moved/renamed successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 409 {object} map[string]string "Conflict - Destination exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [patch]
func resourcePatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	action := r.URL.Query().Get("action")
	if !d.user.Permissions.Modify {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}

	encodedFrom := r.URL.Query().Get("from")
	// Decode the URL-encoded path
	src, err := url.QueryUnescape(encodedFrom)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	dst := r.URL.Query().Get("destination")
	dst, err = url.QueryUnescape(dst)
	if err != nil {
		return errToStatus(err), err
	}

	splitSrc := strings.Split(src, "::")
	if len(splitSrc) <= 1 {
		return http.StatusBadRequest, fmt.Errorf("invalid source path: %v", src)
	}
	srcIndex := splitSrc[0]
	src = splitSrc[1]

	splitDst := strings.Split(dst, "::")
	if len(splitDst) <= 1 {
		return http.StatusBadRequest, fmt.Errorf("invalid destination path: %v", dst)
	}
	dstIndex := splitDst[0]
	dst = splitDst[1]

	if dst == "/" || src == "/" {
		return http.StatusForbidden, fmt.Errorf("forbidden: source or destination is attempting to modify root")
	}

	userscopeDst, err := settings.GetScopeFromSourceName(d.user.Scopes, dstIndex)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscopeSrc, err := settings.GetScopeFromSourceName(d.user.Scopes, srcIndex)
	if err != nil {
		return http.StatusForbidden, err
	}

	idx := indexing.GetIndex(dstIndex)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", dstIndex)
	}
	// check target dir exists
	parentDir, isDstDir, err := idx.GetRealPath(userscopeDst, filepath.Dir(dst))
	if err != nil {
		logger.Debugf("Could not get real path for parent dir: %v %v %v", userscopeDst, filepath.Dir(dst), err)
		return http.StatusNotFound, err
	}
	realDest := parentDir + "/" + filepath.Base(dst)

	idx2 := indexing.GetIndex(srcIndex)
	if idx2 == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", srcIndex)
	}

	realSrc, isSrcDir, err := idx2.GetRealPath(userscopeSrc, src)
	if err != nil {
		return http.StatusNotFound, err
	}
	overwrite := r.URL.Query().Get("overwrite") == "true"
	rename := r.URL.Query().Get("rename") == "true"
	if rename {
		realDest = addVersionSuffix(realDest)
	}
	// Permission for overwriting the file
	if overwrite && !d.user.Permissions.Modify {
		return http.StatusForbidden, fmt.Errorf("forbidden: user does not have permission to overwrite file")
	}

	// Validate move/rename operation to prevent circular references
	if action == "rename" || action == "move" {
		if err = validateMoveOperation(realSrc, realDest, isSrcDir); err != nil {
			return http.StatusBadRequest, err
		}
	}

	err = patchAction(r.Context(), patchActionParams{
		action:   action,
		srcIndex: srcIndex,
		dstIndex: dstIndex,
		src:      realSrc,
		dst:      realDest,
		d:        d,
		isSrcDir: isSrcDir,
		isDstDir: isDstDir,
	})
	if err != nil {
		logger.Debugf("Could not run patch action. src=%v dst=%v err=%v", realSrc, realDest, err)
	}
	return errToStatus(err), err
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
	isDstDir bool
}

func patchAction(ctx context.Context, params patchActionParams) error {
	switch params.action {
	case "copy":
		err := files.CopyResource(params.isSrcDir, params.isDstDir, params.srcIndex, params.dstIndex, params.src, params.dst)
		return err
	case "rename", "move":
		idx := indexing.GetIndex(params.srcIndex)
		srcPath := idx.MakeIndexPath(params.src)
		fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
			Access:     store.Access,
			Username:   params.d.user.Username,
			Path:       srcPath,
			Source:     params.srcIndex,
			IsDir:      params.isSrcDir,
			Modify:     params.d.user.Permissions.Modify,
			Expand:     false,
			ReadHeader: false,
		})

		if err != nil {
			return err
		}

		// delete thumbnails
		preview.DelThumbs(ctx, fileInfo)
		return files.MoveResource(params.isSrcDir, params.isDstDir, params.srcIndex, params.dstIndex, params.src, params.dst, store.Share)
	default:
		return fmt.Errorf("unsupported action %s: %w", params.action, errors.ErrInvalidRequestParams)
	}
}

func inspectIndex(w http.ResponseWriter, r *http.Request) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	// Decode the URL-encoded path
	path, _ := url.QueryUnescape(encodedPath)
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
