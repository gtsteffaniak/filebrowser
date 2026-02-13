package files

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// processDirectoryMetadata extracts metadata for audio/video files in directories
func processDirectoryMetadata(response *iteminfo.ExtendedFileInfo, idx *indexing.Index, opts utils.FileOptions) {
	metadataCount := 0
	for i := range response.Files {
		fileItem := &response.Files[i]
		isItemAudio := strings.HasPrefix(fileItem.Type, "audio")
		isItemVideo := strings.HasPrefix(fileItem.Type, "video")
		if isItemAudio || isItemVideo {
			metadataCount++
		}
	}
	// Process files concurrently using goroutines
	var wg sync.WaitGroup
	var mu sync.Mutex // Protects processedCount

	// Refresh directory to update folder sizes
	wg.Go(func() {
		indexPath := idx.MakeIndexPath(response.Path, true)
		err := idx.RefreshDirectory(indexPath, false)
		if err != nil {
			logger.Debugf("Failed to refresh directory for folder size update: %v", err)
		}
	})

	// Set hasMetadata flag if there are files with potential metadata
	if metadataCount > 0 {
		response.HasMetadata = true
	}

	// Only process metadata if explicitly requested
	if opts.Metadata && metadataCount > 0 {
		processedCount := 0

		// Create a single shared FFmpegService instance for all files to coordinate concurrency
		sharedFFmpegService := ffmpeg.NewFFmpegService(10, false, "")
		if sharedFFmpegService != nil {

			for i := range response.Files {
				fileItem := &response.Files[i]
				isItemAudio := strings.HasPrefix(fileItem.Type, "audio")
				isItemVideo := strings.HasPrefix(fileItem.Type, "video")

				if isItemAudio || isItemVideo {
					wg.Go(func() {
						// Extract metadata for audio files (without album art for performance)
						if isItemAudio {
							err := extractAudioMetadata(context.Background(), fileItem, response.RealPath+"/"+fileItem.Name, opts.AlbumArt, opts.Metadata, sharedFFmpegService)
							if err != nil {
								logger.Debugf("failed to extract metadata for file: %s, error: %v", fileItem.Name, err)
							} else {
								mu.Lock()
								processedCount++
								mu.Unlock()
							}
						} else {
							// Extract duration for video files
							err := extractVideoMetadata(context.Background(), fileItem, response.RealPath+"/"+fileItem.Name, sharedFFmpegService)
							if err != nil {
								logger.Debugf("failed to extract video metadata for file: %s, error: %v", fileItem.Name, err)
							} else {
								mu.Lock()
								processedCount++
								mu.Unlock()
							}
						}
					})
				}
			}

		}
	}
	// Wait for all goroutines to complete
	wg.Wait()
}

// finalizeResponse handles final response adjustments (OnlyOffice ID, scope stripping)
func finalizeResponse(response *iteminfo.ExtendedFileInfo, info *iteminfo.FileInfo, realPath string, user *users.User, userScope string) {
	// Add OnlyOffice ID if applicable
	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}

	// Strip user scope from response path to return path relative to user's context
	if user != nil && userScope != "" && userScope != "/" {
		response.Path = strings.TrimPrefix(response.Path, userScope)
		if response.Path == "" {
			response.Path = "/"
		}
	}
}

func CheckPermissions(opts utils.FileOptions, access *access.Storage, user *users.User) (string, string, error) {
	if access == nil {
		return "", "", fmt.Errorf("access not provided")
	}
	if user == nil {
		return "", "", fmt.Errorf("user not provided")
	}
	if opts.Path == "" {
		return "", "", fmt.Errorf("path not provided")
	}
	if opts.Source == "" {
		return "", "", fmt.Errorf("source not provided")
	}

	// Get index
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return "", "", fmt.Errorf("could not get index: %v ", opts.Source)
	}

	// Resolve user scope
	userScope, scopeErr := user.GetScopeForSourcePath(idx.Path)
	if scopeErr != nil || userScope == "" {
		return "", "", fmt.Errorf("user has no access to source: %v", opts.Source)
	}

	safePath, err := utils.SanitizeUserPath(opts.Path)
	if err != nil {
		return "", "", errors.ErrAccessDenied
	}

	// Combine scope + sanitized path
	indexPath := utils.JoinPathAsUnix(userScope, safePath)
	// Layer 1: USER ACCESS CONTROL
	// Quick check: Does THIS user have permission?
	if !access.Permitted(idx.Path, indexPath, user.Username) {
		return "", "", errors.ErrAccessDenied
	}
	return indexPath, userScope, nil
}

func FileInfoFaster(opts utils.FileOptions, access *access.Storage, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
	response := &iteminfo.ExtendedFileInfo{}
	indexPath, userScope, err := CheckPermissions(opts, access, user)
	if err != nil {
		return response, err
	}
	// Get index
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}

	// Layer 2: INDEX RULES (global)
	// Get file info using unified entry point (applies IsViewable/ShouldSkip)
	info, err := idx.GetFileInfo(indexing.FileInfoRequest{
		IndexPath:      indexPath,
		FollowSymlinks: opts.FollowSymlinks,
		ShowHidden:     opts.ShowHidden,
		Expand:         opts.Expand,
		IsRoutineScan:  false, // API call
	})
	if err != nil {
		return response, err // Path excluded by index rules OR doesn't exist
	}

	// Build response
	response.FileInfo = *info
	response.RealPath = filepath.Join(idx.Path, indexPath)
	response.Source = opts.Source

	// Layer 3: FILTER CHILDREN (user access)
	// Remove child items THIS user can't access
	if info.Type == "directory" {
		if err := access.CheckChildItemAccess(response, idx, user.Username); err != nil {
			return response, err
		}
	}

	// Process directory metadata if requested
	if info.Type == "directory" {
		processDirectoryMetadata(response, idx, opts)
	}

	// Process single file content/metadata
	isAudioVideo := strings.HasPrefix(info.Type, "audio") || strings.HasPrefix(info.Type, "video")
	if opts.Content || opts.Metadata || isAudioVideo {
		processContent(response, idx, opts)
	}

	// Finalize response (OnlyOffice ID, scope stripping)
	finalizeResponse(response, info, response.RealPath, user, userScope)

	return response, nil
}

func processContent(info *iteminfo.ExtendedFileInfo, idx *indexing.Index, opts utils.FileOptions) {
	isVideo := strings.HasPrefix(info.Type, "video")
	isAudio := strings.HasPrefix(info.Type, "audio")
	isFolder := info.Type == "directory"
	if isFolder {
		return
	}

	if isVideo {
		// Extract duration for video
		extItem := &iteminfo.ExtendedItemInfo{
			ItemInfo: info.ItemInfo,
		}
		err := extractVideoMetadata(context.Background(), extItem, info.RealPath, nil)
		if err != nil {
			logger.Debugf("failed to extract video metadata for file: "+info.RealPath, info.Name, err)
		} else {
			info.Metadata = extItem.Metadata
		}

		// Handle subtitles if requested
		parentPath := filepath.Dir(info.Path)
		parentInfo, exists := idx.GetMetadataInfo(parentPath, true, false)
		if exists {
			info.GetSubtitles(parentInfo)
		}
		if opts.ExtractEmbeddedSubtitles {
			subtitles := ffmpeg.DetectEmbeddedSubtitles(info.RealPath, info.ModTime)
			info.Subtitles = append(info.Subtitles, subtitles...)
		}

		return
	}

	if isAudio {
		// Create an ExtendedItemInfo to hold the metadata
		extItem := &iteminfo.ExtendedItemInfo{
			ItemInfo: info.ItemInfo,
		}
		err := extractAudioMetadata(context.Background(), extItem, info.RealPath, opts.AlbumArt || opts.Content, opts.Metadata || opts.Content, nil)
		if err != nil {
			logger.Debugf("failed to extract audio metadata for file: "+info.RealPath, info.Name, err)
		} else {
			// Copy metadata to ExtendedFileInfo
			info.Metadata = extItem.Metadata
			info.HasMetadata = true
			info.HasPreview = extItem.Metadata != nil && len(extItem.Metadata.AlbumArt) > 0
		}
		return
	}

	// Process text content for non-video, non-audio files
	if info.Size < 20*1024*1024 { // 20 megabytes in bytes
		content, err := getContent(info.RealPath)
		if err != nil {
			logger.Debugf("could not get content for file: "+info.RealPath, info.Name, err)
			return
		}
		info.Content = content
	} else {
		logger.Debug("skipping large text file contents (20MB limit): "+info.Path, info.Name, info.Type)
	}
}

func generateOfficeId(realPath string) string {
	key, ok := utils.OnlyOfficeCache.Get(realPath)
	if !ok {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		documentKey := utils.HashSHA256(realPath + timestamp)
		utils.OnlyOfficeCache.Set(realPath, documentKey)
		return documentKey
	}
	return key
}

// extractAudioMetadata extracts metadata from an audio file using dhowden/tag
// and optionally extracts duration using the ffmpeg service with concurrency control
// If ffmpegService is nil, a new service will be created (for backward compatibility)
func extractAudioMetadata(ctx context.Context, item *iteminfo.ExtendedItemInfo, realPath string, getArt bool, getDuration bool, ffmpegService *ffmpeg.FFmpegService) error {
	file, err := os.Open(realPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check file size first to prevent reading extremely large files
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Skip files larger than 300MB to prevent memory issues
	maxSize := int64(300)
	if fileInfo.Size() > maxSize*1024*1024 {
		return fmt.Errorf("file with size %d MB exceeds metadata check limit: %d MB", fileInfo.Size()/1024/1024, maxSize)
	}

	m, err := tag.ReadFrom(file)
	if err != nil {
		return err
	}

	item.Metadata = &iteminfo.MediaMetadata{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
		Year:   m.Year(),
		Genre:  m.Genre(),
	}

	// Extract track number
	track, _ := m.Track()
	item.Metadata.Track = track

	// Extract duration ONLY if explicitly requested using the ffmpeg VideoService
	// This respects concurrency limits and gracefully handles missing ffmpeg
	if getDuration {
		// Use provided service or create a new one for backward compatibility
		service := ffmpegService
		if service == nil {
			service = ffmpeg.NewFFmpegService(5, false, "")
		}
		if service != nil {
			if duration, err := service.GetMediaDuration(ctx, realPath); err == nil {
				item.Metadata.Duration = int(duration)
			}
		}
	}

	if !getArt {
		return nil
	}

	// Extract album art if available (stored as raw bytes, auto-encoded to base64 in JSON)
	if picture := m.Picture(); picture != nil && picture.Data != nil {
		// Size limit to prevent memory issues (max 5MB)
		if len(picture.Data) <= 5*1024*1024 {
			item.Metadata.AlbumArt = picture.Data
		} else {
			logger.Debugf("Skipping album art for %s: too large (%d bytes)", realPath, len(picture.Data))
		}
	}

	return nil
}

// extractVideoMetadata extracts duration from video files using ffprobe
// If ffmpegService is nil, a new service will be created (for backward compatibility)
func extractVideoMetadata(ctx context.Context, item *iteminfo.ExtendedItemInfo, realPath string, ffmpegService *ffmpeg.FFmpegService) error {
	// Use provided service or create a new one for backward compatibility
	service := ffmpegService
	if service == nil {
		service = ffmpeg.NewFFmpegService(10, false, "")
	}
	if service != nil {
		duration, err := service.GetMediaDuration(ctx, realPath)
		if err != nil {
			return err
		}
		if duration > 0 {
			item.Metadata = &iteminfo.MediaMetadata{
				Duration: int(duration),
			}
		}
		return nil
	}
	return nil
}

func DeleteFiles(source, absPath string, isDir bool) error {
	index := indexing.GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}

	if !index.Config.DisableIndexing {
		indexPath := index.MakeIndexPath(absPath, isDir)

		// Perform the physical deletion
		err := os.RemoveAll(absPath)
		if err != nil {
			return err
		}

		// Clear cache entries
		indexing.RealPathCache.Delete(absPath)
		indexing.IsDirCache.Delete(absPath + ":isdir")

		// Remove metadata from index
		deleteSuccess := index.DeleteMetadata(indexPath, isDir, isDir)
		if !deleteSuccess {
			logger.Errorf("Failed to delete metadata from index for %s, but filesystem deletion succeeded", indexPath)
		}

		// Refresh the parent directory to recalculate sizes and update counts
		refreshConfig := utils.FileOptions{
			Path:  index.MakeIndexPath(filepath.Dir(absPath), true),
			IsDir: true,
		}
		err = index.RefreshFileInfo(refreshConfig)
		return err
	}

	// Indexing disabled, just delete the file
	return os.RemoveAll(absPath)
}

func RefreshIndex(source string, path string, isDir bool, recursive bool) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	if idx.Config.DisableIndexing {
		return nil
	}
	// Always normalize path using MakeIndexPath
	path = idx.MakeIndexPath(path, isDir)

	// Check if path is a symlink
	realPath, _, _ := idx.GetRealPath(path)
	isSymlink := false
	if realPath != "" {
		if fileInfo, err := os.Lstat(realPath); err == nil {
			isSymlink = fileInfo.Mode()&os.ModeSymlink != 0
		}
	}

	// Skip indexing only for paths that are explicitly excluded (not viewable and should be skipped)
	hidden := indexing.IsHidden(filepath.Join(idx.Path, path))
	if idx.ShouldSkip(isDir, path, hidden, isSymlink, false) {
		return nil
	}

	// For directories, check if the path exists on disk
	// If it doesn't exist, remove it from the index
	if isDir {
		realPath, _, err := idx.GetRealPath(path)
		if err == nil {
			// Check if the directory exists on disk
			if !Exists(realPath) {
				// Directory no longer exists, remove it from the index
				// This clears both Directories and DirectoriesLedger maps
				indexing.RealPathCache.Delete(realPath)
				indexing.IsDirCache.Delete(realPath + ":isdir")
				idx.DeleteMetadata(path, true, false)
				return nil
			}
		}
	}

	err := idx.RefreshFileInfo(utils.FileOptions{Path: path, IsDir: isDir, Recursive: recursive})
	if err != nil {
		logger.Errorf("RefreshFileInfo failed: %v", err)
		return err
	}
	return err
}

// validateMoveDestination checks if a move/rename operation is valid
// It prevents moving a directory into itself or its subdirectories
func validateMoveDestination(src, dst string, isSrcDir bool) error {
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

func MoveResource(isSrcDir bool, sourceIndex, destIndex, realsrc, realdst string, s *share.Storage, a *access.Storage) error {
	// Check if source and destination are the same file
	if realsrc == realdst {
		return fmt.Errorf("cannot move a file to itself: %s", realsrc)
	}

	// Validate the move operation before executing
	if err := validateMoveDestination(realsrc, realdst, isSrcDir); err != nil {
		return err
	}

	// Get indexes for deletion and refresh operations
	srcIdx := indexing.GetIndex(sourceIndex)
	if srcIdx == nil {
		return fmt.Errorf("could not get source index: %v", sourceIndex)
	}
	dstIdx := indexing.GetIndex(destIndex)
	if dstIdx == nil {
		return fmt.Errorf("could not get destination index: %v", destIndex)
	}

	// Prepare paths for index operations
	srcIndexPath := srcIdx.MakeIndexPath(realsrc, isSrcDir)
	srcParentPath := filepath.Dir(realsrc)

	// Perform the physical move
	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}

	// Handle SOURCE cleanup (treat as deletion)
	// Run async to avoid blocking the HTTP response
	if !srcIdx.Config.DisableIndexing {
		go func() {
			if isSrcDir {
				srcIdx.DeleteMetadata(srcIndexPath, true, true)
			} else {
				srcIdx.DeleteMetadata(srcIndexPath, false, false)
			}
			if err := RefreshIndex(sourceIndex, srcParentPath, true, false); err != nil {
				logger.Errorf("Failed to refresh source parent directory %s after move: %v", srcParentPath, err)
			}
		}()
	}

	// Handle DESTINATION indexing
	// Run async to avoid blocking the HTTP response
	if !dstIdx.Config.DisableIndexing {
		if isSrcDir {
			go func() {
				// Recursively index the moved directory tree
				if err := RefreshIndex(destIndex, realdst, true, true); err != nil {
					logger.Errorf("Failed to index moved directory %s: %v", realdst, err)
					return
				}

				// Refresh parent to update its size
				parentDir := filepath.Dir(realdst)
				if err := RefreshIndex(destIndex, parentDir, true, false); err != nil {
					logger.Errorf("Failed to refresh destination parent %s: %v", parentDir, err)
				}
			}()
		} else {
			// For files, refresh parent directory
			parentDir := filepath.Dir(realdst)
			go RefreshIndex(destIndex, parentDir, true, false) //nolint:errcheck
		}
	}

	// Use backend source paths to match how shares are stored
	go s.UpdateShares(srcIdx.Path, srcIdx.MakeIndexPath(realsrc, isSrcDir), dstIdx.Path, dstIdx.MakeIndexPath(realdst, isSrcDir)) //nolint:errcheck

	// Update access rules for the moved path
	if a != nil {
		// If moving within the same source, update the rules
		if srcIdx.Path == dstIdx.Path {
			go a.UpdateRules(srcIdx.Path, srcIdx.MakeIndexPath(realsrc, isSrcDir), dstIdx.MakeIndexPath(realdst, isSrcDir)) //nolint:errcheck
		}
		// Cross-source moves don't preserve access rules (they're source-specific)
	}

	return nil
}

func CopyResource(isSrcDir bool, sourceIndex, destIndex, realsrc, realdst string) error {
	// Check if source and destination are the same file
	if realsrc == realdst {
		return fmt.Errorf("cannot copy a file to itself: %s", realsrc)
	}

	// Validate the copy operation before executing
	if err := validateMoveDestination(realsrc, realdst, isSrcDir); err != nil {
		logger.Errorf("[COPY] Copy validation failed: %v", err)
		return err
	}

	// Perform the physical copy
	err := fileutils.CopyFile(realsrc, realdst)
	if err != nil {
		logger.Errorf("[COPY] Physical copy failed: %v", err)
		return err
	}

	// Refresh source (non-recursive, just metadata)
	srcIdx := indexing.GetIndex(sourceIndex)
	if srcIdx != nil && !srcIdx.Config.DisableIndexing {
		srcRefreshPath := realsrc
		if !isSrcDir {
			srcRefreshPath = filepath.Dir(realsrc)
		}
		go RefreshIndex(sourceIndex, srcRefreshPath, true, false) //nolint:errcheck
	}

	// Refresh destination (RECURSIVE for directories to capture full tree)
	// Run async to avoid blocking the HTTP response
	dstIdx := indexing.GetIndex(destIndex)
	if dstIdx != nil && !dstIdx.Config.DisableIndexing {
		if isSrcDir {
			go func() {
				// Recursively index the copied directory tree
				if err := RefreshIndex(destIndex, realdst, true, true); err != nil {
					logger.Errorf("[COPY] Failed to index copied directory %s: %v", realdst, err)
					return
				}

				// Refresh parent to update its size
				parentDir := filepath.Dir(realdst)
				if err := RefreshIndex(destIndex, parentDir, true, false); err != nil {
					logger.Errorf("[COPY] Failed to refresh parent %s: %v", parentDir, err)
				}
			}()
		} else {
			// For files, refresh parent directory
			dstParent := filepath.Dir(realdst)
			go RefreshIndex(destIndex, dstParent, true, false) //nolint:errcheck
		}
	}
	return nil
}

func WriteDirectory(opts utils.FileOptions) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, _, _ := idx.GetRealPath(opts.Path)

	var stat os.FileInfo
	var err error
	// Check if the destination exists and is a file
	if stat, err = os.Stat(realPath); err == nil && !stat.IsDir() {
		// If it's a file and we're trying to create a directory, remove the file first
		err = os.Remove(realPath)
		if err != nil {
			return fmt.Errorf("could not remove existing file to create directory: %v", err)
		}
	}

	// Ensure the parent directories exist
	// Permissions are set by MkdirAll (subject to umask, which is usually acceptable)
	err = os.MkdirAll(realPath, fileutils.PermDir)
	if err != nil {
		return err
	}

	// Explicitly set directory permissions to bypass umask
	err = os.Chmod(realPath, fileutils.PermDir)
	if err != nil {
		// Handle chmod error gracefully
		logger.Debugf("Could not set file permissions for %s (this may be expected in restricted environments): %v", realPath, err)
	}

	// Refresh the directory itself recursively
	err = RefreshIndex(idx.Name, opts.Path, true, true)
	if err != nil {
		return err
	}

	// Refresh parent directory to update its size
	parentPath := filepath.Dir(opts.Path)
	if parentPath != "." && parentPath != "/" && parentPath != opts.Path {
		if err := RefreshIndex(idx.Name, parentPath, true, false); err != nil {
			logger.Debugf("Could not refresh parent directory %s: %v", parentPath, err)
		}
	}

	return nil
}

func WriteFile(source, path string, in io.Reader) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	realPath, _, _ := idx.GetRealPath(path)
	// Strip trailing slash from realPath if it's meant to be a file
	realPath = strings.TrimRight(realPath, "/")
	// Ensure the parent directories exist
	parentDir := filepath.Dir(realPath)
	err := os.MkdirAll(parentDir, fileutils.PermDir)
	if err != nil {
		return err
	}
	var stat os.FileInfo
	// Check if the destination exists and is a directory
	if stat, err = os.Stat(realPath); err == nil {
		if stat.IsDir() {
			// If it's a directory and we're trying to create a file, remove the directory first
			err = os.RemoveAll(realPath)
			if err != nil {
				return fmt.Errorf("could not remove existing directory to create file: %v", err)
			}
		}
		// If file exists, its permissions will be preserved (O_TRUNC doesn't change permissions)
	}

	// Open the file for writing (create if it doesn't exist, truncate if it does)
	// For new files: permissions are set to fileutils.PermFile (subject to umask, which is usually acceptable)
	// For existing files: permissions are preserved automatically (O_TRUNC doesn't change them)
	file, err := os.OpenFile(realPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy the contents from the reader to the file
	_, err = io.Copy(file, in)
	if err != nil {
		return err
	}

	// Explicitly set file permissions to bypass umask
	err = os.Chmod(realPath, fileutils.PermFile)
	if err != nil {
		// Handle chmod error gracefully
		logger.Debugf("Could not set file permissions for %s (this may be expected in restricted environments): %v", realPath, err)
	}

	// Refresh the file itself
	err = RefreshIndex(source, path, false, false)
	if err != nil {
		return err
	}

	// Refresh parent directory to update its size
	parentPath := filepath.Dir(path)
	if parentPath != "." && parentPath != "/" && parentPath != path {
		if err := RefreshIndex(source, parentPath, true, false); err != nil {
			logger.Debugf("Could not refresh parent directory %s: %v", parentPath, err)
		}
	}

	return nil
}

// getContent reads and returns the file content if it's considered an editable text file.
// This is a wrapper around utils.IsTextFile that also returns the file content.
func getContent(realPath string) (string, error) {
	// First check if it's a text file using the consolidated validation logic
	isText, err := utils.IsTextFile(realPath)
	if err != nil {
		return "", err
	}
	if !isText {
		// Not a text file, return empty string (no error, just not text)
		return "", nil
	}

	// It's a text file, read and return the content
	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}

	// Handle empty file (return specific marker string)
	if len(content) == 0 {
		return "empty-file-x6OlSil", nil
	}

	return string(content), nil
}

func IsNamedPipe(mode os.FileMode) bool {
	return mode&os.ModeNamedPipe != 0
}

func IsSymlink(mode os.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
