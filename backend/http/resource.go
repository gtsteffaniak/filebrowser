package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"
)

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
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopePath := utils.JoinPathAsUnix(userscope, path)
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:    scopePath,
		Modify:  d.user.Permissions.Modify,
		Source:  source,
		Expand:  true,
		Content: r.URL.Query().Get("content") == "true",
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
		Path:   utils.JoinPathAsUnix(userscope, path),
		Source: source,
		Modify: d.user.Permissions.Modify,
		Expand: false,
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
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}
	if !d.user.Permissions.Modify {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileOpts := iteminfo.FileOptions{
		Path:   utils.JoinPathAsUnix(userscope, path),
		Source: source,
		Modify: d.user.Permissions.Modify,
		Expand: false,
	}
	// Directories creation on POST.
	if strings.HasSuffix(path, "/") {
		err = files.WriteDirectory(fileOpts)
		if err != nil {
			return errToStatus(err), err
		}
		return http.StatusOK, nil
	}
	fileInfo, err := files.FileInfoFaster(fileOpts)
	if err == nil {
		if r.URL.Query().Get("override") != "true" {
			logger.Debugf("Resource already exists: %v", fileInfo.RealPath)
			return http.StatusConflict, nil
		}

		// Permission for overwriting the file
		if !d.user.Permissions.Modify {
			return http.StatusForbidden, nil
		}

		preview.DelThumbs(r.Context(), fileInfo)
	}
	err = files.WriteFile(fileOpts, r.Body)
	if err != nil {
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
		Path:   utils.JoinPathAsUnix(userscope, path),
		Source: source,
		Modify: d.user.Permissions.Modify,
		Expand: false,
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
	parentDir, _, err := idx.GetRealPath(userscopeDst, filepath.Dir(dst))
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
	err = patchAction(r.Context(), action, realSrc, realDest, d, isSrcDir, srcIndex, dstIndex)
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

func patchAction(ctx context.Context, action, src, dst string, d *requestContext, isSrcDir bool, srcIndex, destIndex string) error {
	switch action {
	case "copy":
		err := files.CopyResource(srcIndex, destIndex, src, dst)
		return err
	case "rename", "move":
		idx := indexing.GetIndex(srcIndex)
		srcPath := idx.MakeIndexPath(src)
		fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
			Path:       srcPath,
			Source:     srcIndex,
			IsDir:      isSrcDir,
			Modify:     d.user.Permissions.Modify,
			Expand:     false,
			ReadHeader: false,
		})

		if err != nil {
			return err
		}

		// delete thumbnails
		preview.DelThumbs(ctx, fileInfo)
		return files.MoveResource(srcIndex, destIndex, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
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
	mockDir := utils.CreateMockData(NumDirs, numFiles)
	renderJSON(w, r, mockDir) // nolint:errcheck
}
