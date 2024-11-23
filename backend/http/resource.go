package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/utils"
)

// resourceGetHandler retrieves information about a resource.
// @Summary Get resource information
// @Description Returns metadata and optionally file contents for a specified resource path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the resource"
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Param content query string false "Include file content if true"
// @Param checksum query string false "Optional checksum validation"
// @Success 200 {object} files.FileInfo "Resource metadata"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [get]
func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {

	// TODO source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	file, err := files.FileInfoFaster(files.FileOptions{
		Path:       filepath.Join(d.user.Scope, path),
		Modify:     d.user.Perm.Modify,
		Expand:     true,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
		Content:    r.URL.Query().Get("content") == "true",
	})
	if err != nil {
		return errToStatus(err), err
	}
	if file.Type == "directory" {
		return renderJSON(w, r, file)
	}
	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
	}
	return renderJSON(w, r, file)

}

// resourceDeleteHandler deletes a resource at a specified path.
// @Summary Delete a resource
// @Description Deletes a resource located at the specified path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the resource"
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Success 200 "Resource deleted successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [delete]
func resourceDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// TODO source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if path == "/" || !d.user.Perm.Delete {
		return http.StatusForbidden, nil
	}
	realPath, isDir, err := files.GetRealPath(d.user.Scope, path)
	if err != nil {
		return http.StatusNotFound, err
	}
	fileOpts := files.FileOptions{
		Path:       filepath.Join(d.user.Scope, path),
		IsDir:      isDir,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
	}
	file, err := files.FileInfoFaster(fileOpts)
	if err != nil {
		return errToStatus(err), err
	}

	// delete thumbnails
	err = delThumbs(r.Context(), fileCache, file)
	if err != nil {
		return errToStatus(err), err
	}

	err = files.DeleteFiles(realPath, fileOpts)
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
// @Param path query string true "Path to the resource"
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Param override query bool false "Override existing file if true"
// @Success 200 "Resource created successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 409 {object} map[string]string "Conflict - Resource already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [post]
func resourcePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// TODO source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if !d.user.Perm.Create || !d.user.Check(path) {
		return http.StatusForbidden, nil
	}
	fileOpts := files.FileOptions{
		Path:    filepath.Join(d.user.Scope, path),
		Modify:  d.user.Perm.Modify,
		Expand:  false,
		Checker: d.user,
	}
	// Directories creation on POST.
	if strings.HasSuffix(path, "/") {
		err := files.WriteDirectory(fileOpts)
		if err != nil {
			return errToStatus(err), err
		}
		return http.StatusOK, nil
	}
	file, err := files.FileInfoFaster(fileOpts)
	if err == nil {
		if r.URL.Query().Get("override") != "true" {
			return http.StatusConflict, nil
		}

		// Permission for overwriting the file
		if !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		err = delThumbs(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}
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
// @Param path query string true "Path to the resource"
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Success 200 "Resource updated successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 405 {object} map[string]string "Method not allowed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [put]
func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// TODO source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if !d.user.Perm.Modify || !d.user.Check(path) {
		return http.StatusForbidden, nil
	}

	// Only allow PUT for files.
	if strings.HasSuffix(path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	realPath, isDir, err := files.GetRealPath(d.user.Scope, path)
	if err != nil {
		return http.StatusNotFound, err
	}
	fileOpts := files.FileOptions{
		Path:       realPath,
		IsDir:      isDir,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
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
// @Param from query string true "Path from resource"
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
	// TODO source := r.URL.Query().Get("source")
	src := r.URL.Query().Get("from")
	dst := r.URL.Query().Get("destination")
	action := r.URL.Query().Get("action")
	dst, err := url.QueryUnescape(dst)
	if !d.user.Check(src) || !d.user.Check(dst) {
		return http.StatusForbidden, nil
	}
	if err != nil {
		return errToStatus(err), err
	}
	if dst == "/" || src == "/" {
		return http.StatusForbidden, nil
	}

	// check target dir exists
	parentDir, _, err := files.GetRealPath(d.user.Scope, filepath.Dir(dst))
	if err != nil {
		return http.StatusNotFound, err
	}
	realDest := parentDir + "/" + filepath.Base(dst)
	realSrc, isSrcDir, err := files.GetRealPath(d.user.Scope, src)
	if err != nil {
		return http.StatusNotFound, err
	}
	overwrite := r.URL.Query().Get("overwrite") == "true"
	rename := r.URL.Query().Get("rename") == "true"
	if rename {
		realDest = addVersionSuffix(realDest)
	}
	// Permission for overwriting the file
	if overwrite && !d.user.Perm.Modify {
		return http.StatusForbidden, nil
	}
	err = d.RunHook(func() error {
		return patchAction(r.Context(), action, realSrc, realDest, d, fileCache, isSrcDir)
	}, action, realSrc, realDest, d.user)

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

func delThumbs(ctx context.Context, fileCache FileCache, file *files.FileInfo) error {
	if err := fileCache.Delete(ctx, previewCacheKey(file, "small")); err != nil {
		return err
	}
	return nil
}

func patchAction(ctx context.Context, action, src, dst string, d *requestContext, fileCache FileCache, isSrcDir bool) error {
	switch action {
	// TODO: use enum
	case "copy":
		if !d.user.Perm.Create {
			return errors.ErrPermissionDenied
		}
		return files.CopyResource(src, dst, isSrcDir)
	case "rename", "move":
		if !d.user.Perm.Rename {
			return errors.ErrPermissionDenied
		}
		file, err := files.FileInfoFaster(files.FileOptions{
			Path:       src,
			IsDir:      isSrcDir,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: false,
			Checker:    d.user,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = delThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}
		return files.MoveResource(src, dst, isSrcDir)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}

type DiskUsageResponse struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// diskUsage returns the disk usage information for a given directory.
// @Summary Get disk usage
// @Description Returns the total and used disk space for a specified directory.
// @Tags Resources
// @Accept json
// @Produce json
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Success 200 {object} DiskUsageResponse "Disk usage details"
// @Failure 404 {object} map[string]string "Directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/usage [get]
func diskUsage(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "/"
	}

	value, ok := utils.DiskUsageCache.Get(source).(DiskUsageResponse)
	if ok {
		return renderJSON(w, r, &value)
	}

	fPath, isDir, err := files.GetRealPath(d.user.Scope, source)
	if err != nil {
		return errToStatus(err), err
	}
	if !isDir {
		return http.StatusNotFound, fmt.Errorf("not a directory: %s", source)
	}
	usage, err := disk.UsageWithContext(r.Context(), fPath)
	if err != nil {
		return errToStatus(err), err
	}
	latestUsage := DiskUsageResponse{
		Total: usage.Total,
		Used:  usage.Used,
	}
	utils.DiskUsageCache.Set(source, latestUsage)
	return renderJSON(w, r, &latestUsage)
}

func inspectIndex(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	isDir := r.URL.Query().Get("isDir") == "true"
	index := files.GetIndex(config.Server.Root)
	info, _ := index.GetReducedMetadata(path, isDir)
	renderJSON(w, r, info) // nolint:errcheck
}
