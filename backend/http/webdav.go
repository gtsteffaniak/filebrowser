package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	commonerrors "github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// fileInfoWrapper wraps iteminfo.ItemInfo to implement os.FileInfo
type fileInfoWrapper struct {
	iteminfo.ItemInfo
}

func (f *fileInfoWrapper) Name() string       { return f.ItemInfo.Name }
func (f *fileInfoWrapper) Size() int64        { return f.ItemInfo.Size }
func (f *fileInfoWrapper) Mode() os.FileMode  { return f.mode() }
func (f *fileInfoWrapper) ModTime() time.Time { return f.ItemInfo.ModTime }
func (f *fileInfoWrapper) IsDir() bool        { return f.Type == "directory" }
func (f *fileInfoWrapper) Sys() interface{}   { return nil }

func (f *fileInfoWrapper) mode() os.FileMode {
	if f.IsDir() {
		return os.ModeDir | fileutils.PermDir
	}
	return fileutils.PermFile
}

// filteredFileSystem wraps a webdav.FileSystem and filters directory listings using FileInfoFaster
type filteredFileSystem struct {
	fs     webdav.FileSystem
	source string
	user   *users.User
	// Cache FileInfoFaster results per path to avoid redundant calls within the same request
	fileInfoCache map[string]*iteminfo.ExtendedFileInfo
}

// getFileInfo retrieves file information with caching and access control
// requestPath should NOT include user scope - FileInfoFaster applies it internally
// This function prefers cache, then tries FileInfoFaster which applies access control
func (ffs *filteredFileSystem) getFileInfo(requestPath string, expand bool) (*iteminfo.ExtendedFileInfo, error) {
	// Initialize cache if needed
	if ffs.fileInfoCache == nil {
		ffs.fileInfoCache = make(map[string]*iteminfo.ExtendedFileInfo)
	}

	// Create cache key (don't modify requestPath itself!)
	cacheKey := requestPath
	if expand {
		cacheKey += ":expand"
	}
	if cached, found := ffs.fileInfoCache[cacheKey]; found {
		return cached, nil
	}

	// If we're looking for non-expanded but have expanded cached, we can use it
	// (expanded contains all the same info plus more)
	if !expand {
		expandedKey := requestPath + ":expand"
		if cached, found := ffs.fileInfoCache[expandedKey]; found {
			return cached, nil
		}
	}

	// Call FileInfoFaster with clean requestPath (without scope or cache suffix)
	// FileInfoFaster applies user scope internally AND enforces access control
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:              requestPath,
		Source:            ffs.source,
		Expand:            expand,
		ShowHidden:        ffs.user.ShowHidden,
		SkipExtendedAttrs: true,
		FollowSymlinks:    true,
	}, accessStore, ffs.user, shareStore)
	if err != nil {
		return nil, err
	}

	// Cache result using the cache key
	ffs.fileInfoCache[cacheKey] = fileInfo

	// If we got an expanded directory listing, also cache individual file entries
	// so Stat calls on those files can use the cache
	if expand && fileInfo.Type == "directory" {
		dirPath := requestPath
		// Cache each file and folder from the directory listing
		for _, file := range fileInfo.Files {
			filePath := utils.JoinPathAsUnix(dirPath, file.Name)
			// Create a minimal ExtendedFileInfo for the file (without expanding its contents)
			fileEntry := &iteminfo.ExtendedFileInfo{
				FileInfo: iteminfo.FileInfo{
					ItemInfo: file.ItemInfo,
				},
			}
			ffs.fileInfoCache[filePath] = fileEntry
		}
		for _, folder := range fileInfo.Folders {
			folderPath := utils.JoinPathAsUnix(dirPath, folder.Name)
			// Create a minimal ExtendedFileInfo for the folder (without expanding its contents)
			folderEntry := &iteminfo.ExtendedFileInfo{
				FileInfo: iteminfo.FileInfo{
					ItemInfo: folder,
				},
			}
			ffs.fileInfoCache[folderPath] = folderEntry
		}
	}

	return fileInfo, nil
}

// checkAccess validates if the user can access a given path
// This is used by all write operations (mkdir, delete, rename, etc.)
// Returns nil if access is allowed, error otherwise
func (ffs *filteredFileSystem) checkAccess(requestPath string) error {
	// First, validate permissions using CheckPermissions
	indexPath, _, err := files.CheckPermissions(utils.FileOptions{
		FollowSymlinks: false,
		Path:           requestPath,
		Source:         ffs.source,
		ShowHidden:     ffs.user.ShowHidden,
	}, accessStore, ffs.user)
	if err != nil {
		logger.Debugf("checkAccess: CheckPermissions denied for %s: %v", requestPath, err)
		return err
	}

	// Try to get file info to verify it's accessible
	// Use non-expanded to be faster (we just need to know if it exists and is accessible)
	_, err = ffs.getFileInfo(requestPath, false)
	if err == nil {
		// Successfully got info - access allowed
		return nil
	}

	// Handle specific error cases
	if errors.Is(err, commonerrors.ErrAccessDenied) {
		logger.Debugf("checkAccess: access explicitly denied for %s", requestPath)
		return os.ErrPermission
	}

	// CRITICAL: If item is not indexed AND not viewable, deny access
	// This prevents WebDAV from creating/modifying files in non-indexed areas
	if errors.Is(err, commonerrors.ErrNotViewable) {
		logger.Debugf("checkAccess: path not viewable for %s", requestPath)
		return os.ErrPermission
	}

	// If not indexed but potentially viewable, we need to check more carefully
	if errors.Is(err, commonerrors.ErrNotIndexed) {
		// Get the index to check viewability
		idx := indexing.GetIndex(ffs.source)
		if idx == nil {
			return fmt.Errorf("source not found")
		}

		// Check if the item is viewable using GetFileInfo (without expand)
		info, getErr := idx.GetFileInfo(indexing.FileInfoRequest{
			IndexPath:         indexPath,
			FollowSymlinks:    false,
			ShowHidden:        ffs.user.ShowHidden,
			Expand:            false,
			SkipExtendedAttrs: true,
		})

		// If GetFileInfo succeeds, the item is viewable despite not being indexed
		if getErr == nil && info != nil {
			logger.Debugf("checkAccess: path not indexed but viewable for %s", requestPath)
			return nil // Allow access to viewable items
		}

		// Not viewable - deny access
		logger.Debugf("checkAccess: path not indexed and not viewable for %s", requestPath)
		return os.ErrPermission
	}

	// For other errors (like file not found), that's okay for new file/directory creation
	// The caller will handle whether the operation is appropriate
	logger.Debugf("checkAccess: path check returned: %v (may be acceptable for new items)", err)
	return nil
}

// filteredFile wraps a webdav.File and filters Readdir results based on FileInfoFaster
type filteredFile struct {
	webdav.File
	fs          *filteredFileSystem
	requestPath string // The request path (without user scope)
	isDir       bool   // Whether this is a directory
}

func (ff *filteredFile) Readdir(count int) ([]os.FileInfo, error) {
	// If not a directory, use the underlying file's Readdir
	if !ff.isDir {
		return ff.File.Readdir(count)
	}

	// Pass the requestPath (without scope) to getCachedFileInfo
	// FileInfoFaster will apply the user's scope internally
	fileInfo, err := ff.fs.getFileInfo(ff.requestPath, true)
	if err != nil {
		logger.Debugf("readdir: getFileInfo failed for requestPath=%s: %v", ff.requestPath, err)
		// Handle errors gracefully - return empty directory for access/indexing issues
		// This is especially important when user's scope points to a non-viewable directory
		if errors.Is(err, commonerrors.ErrAccessDenied) ||
			errors.Is(err, commonerrors.ErrNotIndexed) ||
			errors.Is(err, commonerrors.ErrNotViewable) {
			logger.Debugf("readdir: access issue for %s: %v - returning empty", ff.requestPath, err)
			return []os.FileInfo{}, nil
		}
		// Other errors - propagate them
		return nil, err
	}

	// Build os.FileInfo list directly from FileInfoFaster's filtered results
	// No need to read from underlying filesystem - FileInfoFaster already filtered by permissions
	entries := make([]os.FileInfo, 0, len(fileInfo.Files)+len(fileInfo.Folders))

	// Add folders first (common convention)
	for _, folder := range fileInfo.Folders {
		entries = append(entries, &fileInfoWrapper{ItemInfo: folder})
	}

	// Add files
	for _, file := range fileInfo.Files {
		entries = append(entries, &fileInfoWrapper{ItemInfo: file.ItemInfo})
	}

	// Handle count parameter
	if count > 0 && len(entries) > count {
		return entries[:count], nil
	}
	return entries, nil
}

func (ffs *filteredFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if !ffs.user.Permissions.Create {
		return fmt.Errorf("create permission required")
	}

	// Check access before creating directory
	if err := ffs.checkAccess(name); err != nil {
		logger.Debugf("Mkdir: access denied for %s: %v", name, err)
		return err
	}

	return ffs.fs.Mkdir(ctx, name, perm)
}

func (ffs *filteredFileSystem) OpenFile(ctx context.Context, requestPath string, flag int, perm os.FileMode) (webdav.File, error) {
	// Check if this is a write operation
	isWrite := (flag&os.O_WRONLY) != 0 || (flag&os.O_RDWR) != 0 || (flag&os.O_CREATE) != 0

	if isWrite {
		// Check user permissions first
		if !ffs.user.Permissions.Create && !ffs.user.Permissions.Modify {
			return nil, fmt.Errorf("write permission required")
		}

		// For write operations, check access
		if err := ffs.checkAccess(requestPath); err != nil {
			logger.Debugf("OpenFile: write access denied for %s: %v", requestPath, err)
			return nil, err
		}
	}

	file, err := ffs.fs.OpenFile(ctx, requestPath, flag, perm)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	// Wrap the file to filter directory listings
	// name is the request path (without user scope)
	return &filteredFile{
		File:        file,
		fs:          ffs,
		requestPath: requestPath,
		isDir:       stat.IsDir(),
	}, nil
}

func (ffs *filteredFileSystem) RemoveAll(ctx context.Context, requestPath string) error {
	if !ffs.user.Permissions.Delete {
		return fmt.Errorf("delete permission required")
	}

	// Check access before deleting
	if err := ffs.checkAccess(requestPath); err != nil {
		logger.Debugf("RemoveAll: access denied for %s: %v", requestPath, err)
		return err
	}

	return ffs.fs.RemoveAll(ctx, requestPath)
}

func (ffs *filteredFileSystem) Rename(ctx context.Context, oldPath, newPath string) error {
	if !ffs.user.Permissions.Modify {
		return fmt.Errorf("modify permission required")
	}

	// Check access for both old and new paths
	if err := ffs.checkAccess(oldPath); err != nil {
		logger.Debugf("Rename: access denied for source %s: %v", oldPath, err)
		return err
	}

	if err := ffs.checkAccess(newPath); err != nil {
		logger.Debugf("Rename: access denied for destination %s: %v", newPath, err)
		return err
	}

	return ffs.fs.Rename(ctx, oldPath, newPath)
}

func (ffs *filteredFileSystem) Stat(ctx context.Context, requestPath string) (os.FileInfo, error) {
	// Try to get file info (uses cache if available, FileInfoFaster otherwise)
	_, err := ffs.getFileInfo(requestPath, false)
	if err == nil {
		// Successfully got info - use underlying filesystem
		return ffs.fs.Stat(ctx, requestPath)
	}

	// Handle specific error cases
	if errors.Is(err, commonerrors.ErrAccessDenied) || errors.Is(err, commonerrors.ErrNotViewable) {
		return nil, os.ErrPermission
	}

	// For not indexed, check if it's viewable
	if errors.Is(err, commonerrors.ErrNotIndexed) {
		// Use checkAccess to determine if viewable
		if accessErr := ffs.checkAccess(requestPath); accessErr != nil {
			return nil, os.ErrPermission
		}
		// Viewable - allow underlying filesystem
		return ffs.fs.Stat(ctx, requestPath)
	}

	// For other errors (like file not found), let underlying filesystem handle it
	return ffs.fs.Stat(ctx, requestPath)
}

// webDAVHandler serves WebDAV requests.
func webDAVHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Download {
		return http.StatusForbidden, fmt.Errorf("download permission required")
	}
	if r.Method == "DELETE" && !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("delete permission required")
	}
	isWrite := r.Method == http.MethodPut || r.Method == "MKCOL"
	if isWrite && !userCanWrite(d.user.Permissions) {
		return http.StatusForbidden, fmt.Errorf("user has no permission to modify")
	}
	requestPath := utils.AddTrailingSlashIfNotExists(r.PathValue("path"))
	source := r.PathValue("source")
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	_, userScope, err := files.CheckPermissions(utils.FileOptions{
		FollowSymlinks: false,
		Path:           requestPath,
		Source:         source,
		ShowHidden:     d.user.ShowHidden,
	}, accessStore, d.user)
	if err != nil {
		return http.StatusForbidden, err
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}

	// Get the user's scope to determine the WebDAV root directory
	// Resolve the scope path to get the real filesystem root for WebDAV
	// This is the root directory that WebDAV will use to resolve relative paths.
	// userScope is an index path (e.g. "/", "/public"); strip the leading "/" so
	// filepath.Join inside GetRealPath does not treat it as a host-absolute path
	// and discard idx.Path (e.g. Join(sourceRoot, "/") would become "/").
	scopeRel := strings.TrimPrefix(userScope, "/")
	scopePath, _, err := idx.GetRealPath(scopeRel)
	if err != nil {
		return http.StatusNotFound, err
	}

	// Construct the WebDAV prefix from BaseURL
	webDavPrefix := config.Server.BaseURL + "dav"
	prefix := webDavPrefix + "/" + source
	// Wrap the filesystem to filter directory listings using FileInfoFaster
	// We pass requestPath (without scope) to FileInfoFaster, which applies scope internally
	filteredFS := &filteredFileSystem{
		fs:     webdav.Dir(scopePath),
		source: source,
		user:   d.user,
	}

	wd := &webdav.Handler{
		Prefix:     prefix,
		FileSystem: filteredFS,
		LockSystem: idx.WebdavLock,
		Logger: func(req *http.Request, err error) {
			if err != nil {
				errStr := err.Error()
				// Filter out expected/benign errors that don't indicate actual failures
				if strings.Contains(errStr, "no such file or directory") ||
					strings.Contains(errStr, "skip this directory") {
					return
				}
				logger.Errorf("webdav handler failed on path %s: %s", req.URL.Path, err)
			}
		},
	}

	wd.ServeHTTP(w, r)
	return 200, nil // errors and responses (XML-formatted) are handled by webdav handler
}

func userCanWrite(permissions users.Permissions) bool {
	return permissions.Create && permissions.Modify && permissions.Delete
}
