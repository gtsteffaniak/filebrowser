package http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
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
func (f *fileInfoWrapper) IsDir() bool        { return f.ItemInfo.Type == "directory" }
func (f *fileInfoWrapper) Sys() interface{}   { return nil }

func (f *fileInfoWrapper) mode() os.FileMode {
	if f.IsDir() {
		return os.ModeDir | fileutils.PermDir
	}
	return fileutils.PermFile
}

// filteredFileSystem wraps a webdav.FileSystem and filters directory listings using FileInfoFaster
type filteredFileSystem struct {
	fs        webdav.FileSystem
	source    string
	user      *users.User
	userscope string
	// Cache FileInfoFaster results per path to avoid redundant calls within the same request
	fileInfoCache map[string]*iteminfo.ExtendedFileInfo
}

// getCachedFileInfo retrieves cached FileInfoFaster result, or calls FileInfoFaster and caches it
func (ffs *filteredFileSystem) getCachedFileInfo(permissionPath string, expand bool) (*iteminfo.ExtendedFileInfo, error) {
	// Initialize cache if needed
	if ffs.fileInfoCache == nil {
		ffs.fileInfoCache = make(map[string]*iteminfo.ExtendedFileInfo)
	}

	// Check cache - first try exact match
	cacheKey := permissionPath
	if expand {
		cacheKey += ":expand"
	}
	if cached, found := ffs.fileInfoCache[cacheKey]; found {
		return cached, nil
	}

	// If we're looking for non-expanded but have expanded cached, we can use it
	// (expanded contains all the same info plus more)
	if !expand {
		expandedKey := permissionPath + ":expand"
		if cached, found := ffs.fileInfoCache[expandedKey]; found {
			return cached, nil
		}
	}
	// Call FileInfoFaster
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:       permissionPath,
		Source:     ffs.source,
		Expand:     expand,
		ShowHidden: ffs.user.ShowHidden,
	}, store.Access, ffs.user)
	if err != nil {
		return nil, err
	}

	// Cache result
	ffs.fileInfoCache[cacheKey] = fileInfo

	// If we got an expanded directory listing, also cache individual file entries
	// so Stat calls on those files can use the cache
	if expand && fileInfo.Type == "directory" {
		dirPath := permissionPath
		if !strings.HasSuffix(dirPath, "/") && dirPath != "" {
			dirPath += "/"
		}

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

// filteredFile wraps a webdav.File and filters Readdir results based on FileInfoFaster
type filteredFile struct {
	webdav.File
	fs      *filteredFileSystem
	dirPath string // The directory path this file represents (relative to scope root)
	isDir   bool   // Whether this is a directory
}

func (ff *filteredFile) Readdir(count int) ([]os.FileInfo, error) {
	// If not a directory, use the underlying file's Readdir
	if !ff.isDir {
		return ff.File.Readdir(count)
	}

	// For directories, use FileInfoFaster to get filtered contents
	// Convert dirPath (relative to scope root) to permission path (relative to userscope)
	// Handle root path specially
	var permissionPath string
	if ff.dirPath == "" || ff.dirPath == "/" {
		permissionPath = ff.fs.userscope
	} else {
		permissionPath = utils.JoinPathAsUnix(ff.fs.userscope, ff.dirPath)
	}

	// Use cached FileInfoFaster result if available
	fileInfo, err := ff.fs.getCachedFileInfo(permissionPath, true)
	if err != nil {
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
	return ffs.fs.Mkdir(ctx, name, perm)
}

func (ffs *filteredFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := ffs.fs.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	// Check if it's a directory
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	// Wrap the file to filter directory listings
	return &filteredFile{
		File:    file,
		fs:      ffs,
		dirPath: name,
		isDir:   stat.IsDir(),
	}, nil
}

func (ffs *filteredFileSystem) RemoveAll(ctx context.Context, name string) error {
	return ffs.fs.RemoveAll(ctx, name)
}

func (ffs *filteredFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	return ffs.fs.Rename(ctx, oldName, newName)
}

func (ffs *filteredFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	// Root path "/" should always be allowed
	if name == "" || name == "/" {
		return ffs.fs.Stat(ctx, name)
	}

	// Check permission before stat using cached result if available
	permissionPath := utils.JoinPathAsUnix(ffs.userscope, name)
	_, err := ffs.getCachedFileInfo(permissionPath, false)
	if err != nil {
		return nil, err
	}

	return ffs.fs.Stat(ctx, name)
}

// webDAVHandler serves WebDAV requests.
func webDAVHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.PathValue("path")
	source := r.PathValue("source")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if !userHasReadWriteAccess(d) {
		// (reddec): we're currently allowing WebDAV access only for users with R/W access for simplicity.
		// in the follow-ups, we need to add mapping of webdav operations and permissions (which is trickier than it looks like).
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Debugf("source %s not found", source)
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}

	// Use FileInfoFaster to validate permissions and get file info
	// This handles access control and returns Path (relative to idx.Path) and RealPath (absolute)
	// The permission check happens in FileInfoFaster - if access is denied, it returns an error
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		FollowSymlinks: false,
		Path:           path,
		Source:         source,
		ShowHidden:     d.user.ShowHidden,
	}, store.Access, d.user)
	if err != nil {
		logger.Debugf("error getting file info: %v", err)
		return errToStatus(err), err
	}

	// Get the user's scope to determine the WebDAV root directory
	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		logger.Debugf("error getting scope from source name: %v", err)
		return http.StatusForbidden, err
	}

	// Resolve the scope path to get the real filesystem root for WebDAV
	// This is the root directory that WebDAV will use to resolve relative paths
	scopePath, _, err := idx.GetRealPath(userscope)
	if err != nil {
		logger.Debugf("error resolving scope path: %v", err)
		return http.StatusNotFound, err
	}

	// Construct the WebDAV prefix from BaseURL
	webDavPrefix := config.Server.BaseURL + "dav"
	prefix := webDavPrefix + "/" + source
	err = store.Access.CheckChildItemAccess(&iteminfo.ExtendedFileInfo{
		FileInfo: fileInfo.FileInfo,
		Source:   source,
		RealPath: scopePath,
	}, idx, d.user.Username)
	if err != nil {
		logger.Debugf("error checking child item access: %v", err)
		return http.StatusForbidden, err
	}

	logger.Debugf("webdav: method=%s, request=%s, source=%s, path=%s, virtual_path=%s, real_path=%s, scope_path=%s", r.Method, r.URL.Path, source, path, fileInfo.Path, fileInfo.RealPath, scopePath)

	// Wrap the filesystem to filter directory listings using FileInfoFaster
	// This prevents items the user can't access from appearing in listings,
	// which stops clients from repeatedly trying to access them
	filteredFS := &filteredFileSystem{
		fs:        webdav.Dir(scopePath),
		source:    source,
		user:      d.user,
		userscope: userscope,
	}

	wd := &webdav.Handler{
		Prefix:     prefix,
		FileSystem: filteredFS,
		LockSystem: idx.WebdavLock,
		Logger: func(req *http.Request, err error) {
			if err != nil {
				logger.Errorf("webdav handler failed on path %s: %s", req.URL.Path, err)
			}
		},
	}

	wd.ServeHTTP(w, r)
	return 0, nil // errors and responses (XML-formatted) are handled by webdav handler
}
func userHasReadWriteAccess(d *requestContext) bool {
	return d.user.Permissions.Create && d.user.Permissions.Delete && d.user.Permissions.Modify && d.user.Permissions.Download
}
