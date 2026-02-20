package http

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"
)

// archiveCreateHandler creates an archive on the server at the given destination.
// POST /resources/archive — server-side only; does not return archive data.
//
// @Summary Create an archive on the server
// @Description Creates a zip or tar.gz archive on the server from the given items (files and/or directories). Server-side only; no archive bytes are returned. All items must be from the same source. Folders are walked recursively; access-denied paths are silently skipped. Requires create permission.
// @Description
// @Description **Request body parameters:**
// @Description - **source** (string, required): Source name where the items to archive live. Example: `"default"`
// @Description - **toSource** (string, optional): Source name where the archive file will be written. Defaults to `source` if omitted. Example: `"backups"`
// @Description - **items** (array of strings, required): Paths of files or directories to add to the archive (relative to source). Directories are walked; access-denied entries are skipped. Example: `["/docs/file.txt", "/photos"]`
// @Description - **destination** (string, required): Full path where the archive file will be created (on toSource). Must end with .zip or .tar.gz (or format is inferred). Example: `"/backups/my-archive.zip"`
// @Description - **format** (string, optional): Archive format. One of: `"zip"`, `"tar.gz"`. Default inferred from destination extension. Example: `"zip"`
// @Description - **compression** (integer, optional): Gzip compression level for tar.gz only (0–9). 0 = default. Ignored for zip. Example: `6`
// @Description - **deleteAfter** (boolean, optional): If true, delete source files/directories after successful creation. Requires delete permission. Example: `true`
// @Tags Resources
// @Accept json
// @Produce json
// @Param body body archiveCreateRequest true "Request body: source, toSource (optional), items, destination, format (optional), compression (optional)"
// @Success 200 {object} map[string]string "Created; returns {\"path\": \"<destination path>\"}"
// @Failure 400 {object} map[string]string "Invalid request (e.g. missing required field, invalid path)"
// @Failure 403 {object} map[string]string "Forbidden (create permission or access denied)"
// @Failure 404 {object} map[string]string "Source not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/archive [post]
func archiveCreateHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Archive {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create archives")
	}
	if d.share != nil {
		return http.StatusForbidden, fmt.Errorf("archive create not allowed for shares")
	}
	if !d.user.Permissions.Create {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create resources")
	}

	var req archiveCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if req.Source == "" || len(req.Items) == 0 || req.Destination == "" {
		return http.StatusBadRequest, fmt.Errorf("source, items, and destination are required")
	}

	destClean, err := utils.SanitizeUserPath(req.Destination)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid destination path: %v", err)
	}
	req.Destination = destClean
	itemsClean := make([]string, 0, len(req.Items))
	for _, p := range req.Items {
		var clean string
		clean, err = utils.SanitizeUserPath(p)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid item path %q: %v", p, err)
		}
		itemsClean = append(itemsClean, clean)
	}
	req.Items = itemsClean

	destSource := req.ToSource
	if destSource == "" {
		destSource = req.Source
	}

	idx := indexing.GetIndex(req.Source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", req.Source)
	}
	userScope, err := d.user.GetScopeForSourceName(req.Source)
	if err != nil {
		return http.StatusForbidden, err
	}

	// Resolve destination on ToSource (or Source if not set)
	idxTo := indexing.GetIndex(destSource)
	if idxTo == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", destSource)
	}
	userScopeTo, err := d.user.GetScopeForSourceName(destSource)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullDest := utils.JoinPathAsUnix(userScopeTo, req.Destination)
	if store.Access != nil && !store.Access.Permitted(idxTo.Path, fullDest, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to destination %s", req.Destination)
	}
	// Destination is a file we are about to create; resolve parent dir only (parent must exist or be creatable).
	destDir := filepath.Dir(req.Destination)
	if destDir == "." {
		destDir = "/"
	}
	fullDestDir := utils.JoinPathAsUnix(userScopeTo, destDir)
	if store.Access != nil && !store.Access.Permitted(idxTo.Path, fullDestDir, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to destination directory %s", destDir)
	}
	destParentReal, _, err := idxTo.GetRealPath(fullDestDir)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("destination directory invalid: %v", err)
	}
	destRealPath := filepath.Join(destParentReal, filepath.Base(req.Destination))
	if err = os.MkdirAll(destParentReal, fileutils.PermDir); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("cannot create destination directory: %v", err)
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		ext := strings.ToLower(filepath.Ext(destRealPath))
		if ext == ".gz" && len(destRealPath) > 3 && strings.HasSuffix(strings.ToLower(destRealPath), ".tar.gz") {
			format = "tar.gz"
		} else if ext == ".zip" {
			format = "zip"
		} else {
			format = "zip"
		}
	}
	if format != "zip" && format != "tar.gz" {
		return http.StatusBadRequest, fmt.Errorf("format must be zip or tar.gz")
	}

	compression := req.Compression
	if compression < 0 || compression > 9 {
		compression = 0
	}

	// Build full paths for items (same source)
	itemPaths := make([]string, 0, len(req.Items))
	for _, it := range req.Items {
		full := utils.JoinPathAsUnix(userScope, it)
		if store.Access != nil && !store.Access.Permitted(idx.Path, full, d.user.Username) {
			continue // silently skip
		}
		itemPaths = append(itemPaths, full)
	}
	if len(itemPaths) == 0 {
		return http.StatusBadRequest, fmt.Errorf("no items accessible; add at least one path you have access to")
	}

	estimatedSize, err := computeArchiveSize(req.Source, req.Items, d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if config.Server.MaxArchiveSizeGB > 0 {
		maxSize := config.Server.MaxArchiveSizeGB * 1024 * 1024 * 1024
		if estimatedSize > maxSize {
			return http.StatusRequestEntityTooLarge, fmt.Errorf("pre-archive combined size of files exceeds maximum limit of %d GB", config.Server.MaxArchiveSizeGB)
		}
	}

	var createErr error
	if format == "zip" {
		createErr = createZip(d, req.Source, destRealPath, itemPaths...)
	} else {
		createErr = createTarGzWithLevel(d, req.Source, destRealPath, compression, itemPaths...)
	}
	if createErr != nil {
		return http.StatusInternalServerError, createErr
	}

	if req.DeleteAfter && d.user.Permissions.Delete {
		type itemToDelete struct {
			realPath string
			isDir    bool
		}
		var toDelete []itemToDelete
		for _, full := range itemPaths {
			realPath, isDir, err := idx.GetRealPath(full)
			if err != nil {
				continue
			}
			toDelete = append(toDelete, itemToDelete{realPath: realPath, isDir: isDir})
		}
		for i := 0; i < len(toDelete); i++ {
			for j := i + 1; j < len(toDelete); j++ {
				if len(toDelete[j].realPath) > len(toDelete[i].realPath) {
					toDelete[i], toDelete[j] = toDelete[j], toDelete[i]
				}
			}
		}
		for _, item := range toDelete {
			if err := files.DeleteFiles(req.Source, item.realPath, item.isDir); err != nil {
				logger.Errorf("Failed to delete source after archive: %v", err)
			}
		}
	}

	return renderJSON(w, r, map[string]string{"path": req.Destination}, http.StatusOK)
}

// unarchiveHandler extracts an archive on the server. POST /resources/unarchive — server-side only.
//
// @Summary Extract an archive on the server
// @Description Extracts a zip or tar.gz archive on the server into the given destination directory. Server-side only; no extracted bytes are returned. Supports extracting to a different source via toSource. Requires create permission.
// @Description
// @Description **Request body parameters:**
// @Description - **fromSource** (string, required): Source name where the archive file lives. Example: `"default"`
// @Description - **toSource** (string, optional): Source name where contents will be extracted. Defaults to fromSource if omitted. Example: `"restored"`
// @Description - **path** (string, required): Path to the archive file (on fromSource). Must be .zip, .tar.gz, or .tgz. Example: `"/downloads/data.zip"`
// @Description - **destination** (string, required): Directory path (on toSource) to extract into. Example: `"/projects/imported"`
// @Description - **deleteAfter** (boolean, optional): If true, delete the archive file after successful extraction. Default: false. Example: `true`
// @Tags Resources
// @Accept json
// @Produce json
// @Param body body unarchiveRequest true "Request body: fromSource, toSource (optional), path, destination, deleteAfter (optional)"
// @Success 200 {object} map[string]string "Extracted; returns {\"path\": \"<destination path>\", \"source\": \"<toSource>\"}"
// @Failure 400 {object} map[string]string "Invalid request (e.g. missing required field, unsupported format)"
// @Failure 403 {object} map[string]string "Forbidden (create permission or access denied)"
// @Failure 404 {object} map[string]string "Source or archive file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/unarchive [post]
func unarchiveHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Archive {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to extract archives")
	}
	if d.share != nil {
		return http.StatusForbidden, fmt.Errorf("unarchive not allowed for shares")
	}
	if !d.user.Permissions.Create {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create resources")
	}

	var req unarchiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if req.FromSource == "" || req.Path == "" || req.Destination == "" {
		return http.StatusBadRequest, fmt.Errorf("fromSource, path, and destination are required")
	}
	if req.ToSource == "" {
		req.ToSource = req.FromSource
	}

	pathClean, err := utils.SanitizeUserPath(req.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %v", err)
	}
	destClean, err := utils.SanitizeUserPath(req.Destination)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid destination: %v", err)
	}
	req.Path = pathClean
	req.Destination = destClean

	idxFrom := indexing.GetIndex(req.FromSource)
	if idxFrom == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", req.FromSource)
	}
	userScopeFrom, err := d.user.GetScopeForSourceName(req.FromSource)
	if err != nil {
		return http.StatusForbidden, err
	}

	fullArchivePath := utils.JoinPathAsUnix(userScopeFrom, req.Path)
	if store.Access != nil && !store.Access.Permitted(idxFrom.Path, fullArchivePath, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to archive %s", req.Path)
	}
	archiveReal, _, err := idxFrom.GetRealPath(fullArchivePath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("archive path not found: %v", err)
	}

	idxTo := indexing.GetIndex(req.ToSource)
	if idxTo == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", req.ToSource)
	}
	userScopeTo, err := d.user.GetScopeForSourceName(req.ToSource)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullDestPath := utils.JoinPathAsUnix(userScopeTo, req.Destination)
	if store.Access != nil && !store.Access.Permitted(idxTo.Path, fullDestPath, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to destination %s", req.Destination)
	}
	// Destination may not exist yet; resolve parent dir then build destination path.
	destDir := filepath.Dir(req.Destination)
	if destDir == "." {
		destDir = "/"
	}
	fullDestDir := utils.JoinPathAsUnix(userScopeTo, destDir)
	if store.Access != nil && !store.Access.Permitted(idxTo.Path, fullDestDir, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to destination directory %s", destDir)
	}
	destParentReal, _, err := idxTo.GetRealPath(fullDestDir)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("destination directory invalid: %v", err)
	}
	destReal := filepath.Join(destParentReal, filepath.Base(req.Destination))
	if err = os.MkdirAll(destReal, fileutils.PermDir); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("cannot create destination directory: %v", err)
	}

	info, err := os.Stat(archiveReal)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("archive not found: %v", err)
	}
	if info.IsDir() {
		return http.StatusBadRequest, fmt.Errorf("path is not an archive file: %s", req.Path)
	}

	lower := strings.ToLower(archiveReal)
	var extractErr error
	if strings.HasSuffix(lower, ".zip") {
		extractErr = extractZip(archiveReal, destReal)
	} else if strings.HasSuffix(lower, ".tar.gz") || (strings.HasSuffix(lower, ".tgz")) {
		extractErr = extractTarGz(archiveReal, destReal)
	} else {
		return http.StatusBadRequest, fmt.Errorf("unsupported archive format (use .zip or .tar.gz)")
	}
	if extractErr != nil {
		return http.StatusInternalServerError, extractErr
	}

	if req.DeleteAfter {
		if err := os.Remove(archiveReal); err != nil {
			logger.Errorf("Failed to delete archive after extract: %v", err)
		}
	}

	return renderJSON(w, r, map[string]string{"path": req.Destination, "source": req.ToSource}, http.StatusOK)
}

// addFile adds a file or directory to a tar or zip archive, respecting access rules.
// For shares, path is already resolved; for users, access is checked via store.Access.
func addFile(source string, path string, d *requestContext, tarWriter *tar.Writer, zipWriter *zip.Writer, flatten bool) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("source %s is not available", source)
	}

	// Check access control directly for each file and silently skip if access is denied
	if d.share == nil && store.Access != nil {
		if !store.Access.Permitted(idx.Path, path, d.user.Username) {
			return nil // Silently skip this file/folder
		}
	}

	// Verify file exists
	_, err := files.FileInfoFaster(utils.FileOptions{
		Path:           path,
		Source:         source,
		Expand:         false,
		FollowSymlinks: true,
	}, store.Access, d.user, store.Share)
	if err != nil {
		return err
	}
	realPath, _, _ := idx.GetRealPath(path)
	info, err := os.Stat(realPath)
	if err != nil {
		return err
	}

	// Get the base name of the top-level folder or file
	baseName := filepath.Base(realPath)

	if info.IsDir() {
		// Walk through directory contents
		return filepath.Walk(realPath, func(filePath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate the relative path
			relPath, err := filepath.Rel(realPath, filePath)
			if err != nil {
				return err
			}

			// Normalize for tar: convert \ to /
			relPath = filepath.ToSlash(relPath)

			// Skip adding `.` (current directory)
			if relPath == "." {
				return nil
			}

			// Check access control for each file/folder during walk
			if d.share == nil {
				indexRelPath := utils.JoinPathAsUnix(path, relPath)
				indexRelPath = filepath.ToSlash(indexRelPath)
				if !store.Access.Permitted(idx.Path, indexRelPath, d.user.Username) {
					if fileInfo.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// Prepend base folder name unless flatten is true
			if !flatten {
				relPath = filepath.Join(baseName, relPath)
				relPath = filepath.ToSlash(relPath)
			}

			if fileInfo.IsDir() {
				if tarWriter != nil {
					header := &tar.Header{
						Name:     relPath + "/",
						Mode:     int64(fileutils.PermDir),
						Typeflag: tar.TypeDir,
						ModTime:  fileInfo.ModTime(),
					}
					return tarWriter.WriteHeader(header)
				}
				if zipWriter != nil {
					_, err := zipWriter.Create(relPath + "/")
					return err
				}
				return nil
			}
			return addSingleFile(filePath, relPath, zipWriter, tarWriter)
		})
	}
	// For a single file, use the base name as the archive path
	return addSingleFile(realPath, baseName, zipWriter, tarWriter)
}

// addSingleFile writes one file into the given zip or tar writer.
func addSingleFile(realPath, archivePath string, zipWriter *zip.Writer, tarWriter *tar.Writer) error {
	file, err := os.Open(realPath)
	if err != nil {
		if strings.Contains(err.Error(), "is a directory") {
			return nil
		}
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	if tarWriter != nil {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(archivePath)
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, file)
		return err
	}

	if zipWriter != nil {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = archivePath
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		return err
	}

	return nil
}

// computeArchiveSize returns the combined size of the given paths, respecting access rules.
// Paths denied by access are skipped (not counted).
func computeArchiveSize(source string, fileList []string, d *requestContext) (int64, error) {
	var estimatedSize int64
	idx := indexing.GetIndex(source)
	if idx == nil {
		return 0, fmt.Errorf("source %s is not available", source)
	}

	var userScope string
	var err error
	if d.share == nil {
		userScope, err = d.user.GetScopeForSourceName(source)
		if err != nil {
			return 0, fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
		}
	}

	for _, path := range fileList {
		var fullPath string
		if d.share == nil {
			fullPath = utils.JoinPathAsUnix(userScope, path)
			if store.Access != nil && !store.Access.Permitted(idx.Path, fullPath, d.user.Username) {
				continue
			}
		} else {
			fullPath = path
		}

		realPath, isDir, err := idx.GetRealPath(fullPath)
		if err != nil {
			return 0, err
		}
		indexPath := idx.MakeIndexPath(realPath, isDir)
		info, ok := idx.GetReducedMetadata(indexPath, isDir)
		if !ok {
			info, err = idx.GetFsInfo(indexPath, false, true)
			if err != nil {
				return 0, fmt.Errorf("failed to get file info for %s : %v", path, err)
			}
		}
		estimatedSize += info.Size
	}
	return estimatedSize, nil
}

// createZip writes a ZIP archive to tmpPath containing the given paths; access rules apply.
func createZip(d *requestContext, source string, tmpPath string, filenames ...string) error {
	file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)

	for _, fname := range filenames {
		err := addFile(source, fname, d, nil, zipWriter, false)
		if err != nil {
			logger.Errorf("Failed to add %s to ZIP: %v", fname, err)
			return err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize ZIP archive: %w", err)
	}
	return nil
}

// createTarGz writes a tar.gz archive to tmpPath containing the given paths; access rules apply.
func createTarGz(d *requestContext, source string, tmpPath string, filenames ...string) error {
	file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzWriter)

	for _, fname := range filenames {
		err := addFile(source, fname, d, tarWriter, nil, false)
		if err != nil {
			logger.Errorf("Failed to add %s to TAR.GZ: %v", fname, err)
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize TAR archive: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize GZIP compression: %w", err)
	}
	return nil
}

// createTarGzWithLevel writes a tar.gz archive with the given gzip compression level (0=default, 1-9).
func createTarGzWithLevel(d *requestContext, source string, destPath string, level int, filenames ...string) error {
	file, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var gzWriter *gzip.Writer
	if level >= 1 && level <= 9 {
		gzWriter, err = gzip.NewWriterLevel(file, level)
		if err != nil {
			return err
		}
	} else {
		gzWriter = gzip.NewWriter(file)
	}
	defer gzWriter.Close()
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, fname := range filenames {
		err := addFile(source, fname, d, tarWriter, nil, false)
		if err != nil {
			logger.Errorf("Failed to add %s to TAR.GZ: %v", fname, err)
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize TAR archive: %w", err)
	}
	return nil
}

// BuildAndStreamArchive resolves paths, creates a zip or tar.gz archive, and streams it to w.
// It respects access rules and max archive size. Used only by the raw handler for multi-file/directory download.
func BuildAndStreamArchive(w http.ResponseWriter, r *http.Request, d *requestContext, source string, fileList []string) (int, error) {
	firstFilePath := fileList[0]
	var userscope string
	var err error

	if d.share == nil {
		userscope, err = d.user.GetScopeForSourceName(source)
		if err != nil {
			return http.StatusForbidden, err
		}
		firstFilePath = utils.JoinPathAsUnix(userscope, firstFilePath)
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	realPath, isDir, err := idx.GetRealPath(firstFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	estimatedSize, err := computeArchiveSize(source, fileList, d)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if config.Server.MaxArchiveSizeGB > 0 {
		maxSize := config.Server.MaxArchiveSizeGB * 1024 * 1024 * 1024
		if estimatedSize > maxSize {
			return http.StatusRequestEntityTooLarge, fmt.Errorf("pre-archive combined size of files exceeds maximum limit of %d GB", config.Server.MaxArchiveSizeGB)
		}
	}

	algo := r.URL.Query().Get("algo")
	var extension string
	switch algo {
	case "zip", "true", "":
		extension = ".zip"
	case "tar.gz":
		extension = ".tar.gz"
	default:
		return http.StatusInternalServerError, errors.New("format not implemented")
	}

	baseDirName := filepath.Base(filepath.Dir(firstFilePath))
	if baseDirName == "" || baseDirName == "/" {
		baseDirName = "download"
	}
	if len(fileList) == 1 && isDir {
		baseDirName = filepath.Base(realPath)
	}
	originalFileName := baseDirName + extension

	archiveData := filepath.Join(config.Server.CacheDir, utils.InsecureRandomIdentifier(10))
	if extension == ".zip" {
		archiveData = archiveData + ".zip"
		err = createZip(d, source, archiveData, fileList...)
	} else {
		archiveData = archiveData + ".tar.gz"
		err = createTarGz(d, source, archiveData, fileList...)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	fd, err := os.Open(archiveData)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	fileInfo, err := fd.Stat()
	if err != nil {
		os.Remove(archiveData)
		return http.StatusInternalServerError, err
	}

	sizeInMB := fileInfo.Size() / 1024 / 1024
	if sizeInMB > 500 {
		logger.Debugf("User %v is downloading large (%d MB) file: %v", d.user.Username, sizeInMB, originalFileName)
	}

	setContentDisposition(w, r, originalFileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Content-Type", "application/octet-stream")

	var reader io.Reader = fd
	if d.share != nil && d.share.MaxBandwidth > 0 {
		limit := rate.Limit(d.share.MaxBandwidth * 1024)
		burst := d.share.MaxBandwidth * 1024
		reader = newThrottledReadSeeker(fd, limit, burst, r.Context())
	}
	_, err = io.Copy(w, reader)
	os.Remove(archiveData)
	if err != nil {
		logger.Errorf("Failed to copy archive data to response: %v", err)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// archiveCreateRequest is the body for POST /resources/archive (server-side create).
type archiveCreateRequest struct {
	// Source name where the items to archive live (required). Example: "default"
	Source string `json:"source"`
	// Source name where the archive file will be written (optional; default: source). Example: "backups"
	ToSource string `json:"toSource"`
	// Paths of files or directories to add; directories are walked; access-denied entries skipped (required). Example: ["/docs/file.txt", "/photos"]
	Items []string `json:"items"`
	// Full path where the archive will be created; use .zip or .tar.gz extension (required). Example: "/backups/my-archive.zip"
	Destination string `json:"destination"`
	// Archive format: "zip" or "tar.gz" (optional; inferred from destination if omitted). Example: "zip"
	Format string `json:"format"`
	// Gzip compression level for tar.gz only, 0-9; 0 = default; ignored for zip (optional). Example: 6
	Compression int `json:"compression"`
	// If true, delete the source files/directories after successful archive creation (optional; requires delete permission). Example: true
	DeleteAfter bool `json:"deleteAfter"`
}

// unarchiveRequest is the body for POST /resources/unarchive (server-side extract).
type unarchiveRequest struct {
	// Source name where the archive file lives (required). Example: "default"
	FromSource string `json:"fromSource"`
	// Source name where contents will be extracted (optional; default: fromSource). Example: "restored"
	ToSource string `json:"toSource"`
	// Path to the archive file on fromSource; .zip, .tar.gz, or .tgz (required). Example: "/downloads/data.zip"
	Path string `json:"path"`
	// Directory path on toSource to extract into (required). Example: "/projects/imported"
	Destination string `json:"destination"`
	// If true, delete the archive file after successful extraction (optional; default: false). Example: true
	DeleteAfter bool `json:"deleteAfter"`
}

// safeExtractPath ensures name does not escape destDir (no ".." or absolute).
func safeExtractPath(destDir, name string) (string, error) {
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid entry path: %s", name)
	}
	abs := filepath.Join(destDir, clean)
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	entryAbs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(entryAbs, destAbs) {
		return "", fmt.Errorf("invalid entry path: %s", name)
	}
	return abs, nil
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		destPath, err := safeExtractPath(destDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(destPath, fileutils.PermDir); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(destPath), fileutils.PermDir); err != nil {
			return err
		}
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		destPath, err := safeExtractPath(destDir, h.Name)
		if err != nil {
			return err
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, fileutils.PermDir); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), fileutils.PermDir); err != nil {
				return err
			}
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
