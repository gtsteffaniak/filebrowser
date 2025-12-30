package files

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dhowden/tag"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

func FileInfoFaster(opts utils.FileOptions, access *access.Storage) (*iteminfo.ExtendedFileInfo, error) {
	response := &iteminfo.ExtendedFileInfo{}
	index := indexing.GetIndex(opts.Source)
	if index == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}
	if !strings.HasPrefix(opts.Path, "/") {
		opts.Path = "/" + opts.Path
	}
	realPath, isDir, err := index.GetRealPath(opts.Path)
	if err != nil {
		return response, fmt.Errorf("could not get real path for requested path: %v, error: %v", opts.Path, err)
	}
	if !strings.HasSuffix(opts.Path, "/") && isDir {
		opts.Path = opts.Path + "/"
	}
	opts.IsDir = isDir
	var info *iteminfo.FileInfo

	// Check if path is viewable (allows filesystem access without indexing)
	isViewable := index.IsViewable(isDir, opts.Path)

	// For non-viewable paths, verify they are indexed
	// Skip this check if indexing is disabled for the entire source
	if !isViewable && !index.Config.DisableIndexing {
		err = index.RefreshFileInfo(opts)
		if err != nil {
			return response, fmt.Errorf("path not accessible: %v", err)
		}
	}

	if isDir {
		info, err = index.GetFsDirInfo(opts.Path)
		if err != nil {
			return response, err
		}
	} else {
		// For files, get info from parent directory to ensure HasPreview is set correctly
		info, err = index.GetFsDirInfo(opts.Path)
		if err != nil {
			return response, err
		}
	}

	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source

	if access != nil {
		err := access.CheckChildItemAccess(response, index, opts.Username)
		if err != nil {
			return response, err
		}
	}

	// For directories, populate metadata for audio/video files ONLY if explicitly requested
	// This avoids expensive ffprobe calls on every directory listing
	if isDir && opts.Metadata {
		startTime := time.Now()
		metadataCount := 0

		// Create a single shared FFmpegService instance for all files to coordinate concurrency
		sharedFFmpegService := ffmpeg.NewFFmpegService(10, false, "")
		if sharedFFmpegService != nil {
			// Process files concurrently using goroutines
			var wg sync.WaitGroup
			var mu sync.Mutex // Protects metadataCount

			for i := range response.Files {
				fileItem := &response.Files[i]
				isItemAudio := strings.HasPrefix(fileItem.Type, "audio")
				isItemVideo := strings.HasPrefix(fileItem.Type, "video")

				if isItemAudio || isItemVideo {
					// Get the real path for this file
					itemRealPath, _, _ := index.GetRealPath(opts.Path, fileItem.Name)

					// Capture loop variables in local copies to avoid closure issues
					item := fileItem
					itemPath := itemRealPath
					isAudio := isItemAudio

					wg.Go(func() {
						// Extract metadata for audio files (without album art for performance)
						if isAudio {
							err := extractAudioMetadata(context.Background(), item, itemPath, opts.AlbumArt || opts.Content, opts.Metadata, sharedFFmpegService)
							if err != nil {
								logger.Debugf("failed to extract metadata for file: "+item.Name, err)
							} else {
								mu.Lock()
								metadataCount++
								mu.Unlock()
							}
						} else {
							// Extract duration for video files
							err := extractVideoMetadata(context.Background(), item, itemPath, sharedFFmpegService)
							if err != nil {
								logger.Debugf("failed to extract video metadata for file: "+item.Name, err)
							} else {
								mu.Lock()
								metadataCount++
								mu.Unlock()
							}
						}
					})
				}
			}

			// Wait for all goroutines to complete
			wg.Wait()
		}

		if metadataCount > 0 {
			elapsed := time.Since(startTime)
			logger.Debugf("Extracted metadata for %d audio/video files concurrently in %v (avg: %v per file)",
				metadataCount, elapsed, elapsed/time.Duration(metadataCount))
		}
	}

	// Extract content/metadata when explicitly requested OR for single file audio/video requests
	isAudioVideo := strings.HasPrefix(info.Type, "audio") || strings.HasPrefix(info.Type, "video")
	if opts.Content || opts.Metadata || (!isDir && isAudioVideo) {
		processContent(response, index, opts)
	}

	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}

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
		if opts.ExtractEmbeddedSubtitles {
			parentPath := filepath.Dir(info.Path)
			parentInfo, exists := idx.GetReducedMetadata(parentPath, true)
			if exists {
				info.DetectSubtitles(parentInfo)
				err := info.LoadSubtitleContent()
				if err != nil {
					logger.Debug("failed to load subtitle content: " + err.Error())
				}
			}
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
			info.HasPreview = extItem.Metadata != nil && extItem.Metadata.AlbumArt != ""
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
			startTime := time.Now()
			if duration, err := service.GetMediaDuration(ctx, realPath); err == nil {
				item.Metadata.Duration = int(duration)
				elapsed := time.Since(startTime)
				if elapsed > 100*time.Millisecond {
					logger.Debugf("Duration extraction took %v for file: %s", elapsed, item.Name)
				}
			}
		}
	}

	if !getArt {
		return nil
	}

	// Extract album art and encode as base64 with strict size limits
	if picture := m.Picture(); picture != nil && picture.Data != nil {
		// More aggressive size limit to prevent memory issues (max 5MB)
		if len(picture.Data) <= 5*1024*1024 {
			item.Metadata.AlbumArt = base64.StdEncoding.EncodeToString(picture.Data)
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

func DeleteFiles(source, absPath string, absDirPath string, isDir bool) error {
	index := indexing.GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}

	if !index.Config.DisableIndexing {
		indexPath := index.MakeIndexPath(absPath)

		// Perform the physical deletion
		err := os.RemoveAll(absPath)
		if err != nil {
			return err
		}

		// Clear cache entries
		indexing.RealPathCache.Delete(absPath)
		indexing.IsDirCache.Delete(absPath + ":isdir")

		// Remove metadata from index
		if isDir {
			// Recursively remove directory and all subdirectories from index
			index.DeleteMetadata(indexPath, true, true)
		} else {
			// Remove file from parent's file list
			index.DeleteMetadata(indexPath, false, false)
		}

		// Refresh the parent directory to recalculate sizes and update counts
		// This will traverse up the tree and update all parent sizes correctly
		refreshConfig := utils.FileOptions{
			Path:  index.MakeIndexPath(absDirPath),
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
	path = idx.MakeIndexPath(path)

	// MakeIndexPath always adds trailing slash, but for files we need to remove it
	if !isDir {
		path = strings.TrimSuffix(path, "/")
	}

	// Skip indexing for viewable paths (viewable: true means don't index, just allow FS access)
	if idx.IsViewable(isDir, path) {
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
	srcIndexPath := srcIdx.MakeIndexPath(realsrc)
	srcParentPath := filepath.Dir(realsrc)

	// Perform the physical move
	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}

	// Handle SOURCE cleanup (treat as deletion)
	if !srcIdx.Config.DisableIndexing {
		if isSrcDir {
			srcIdx.DeleteMetadata(srcIndexPath, true, true)
		} else {
			srcIdx.DeleteMetadata(srcIndexPath, false, false)
		}
		go RefreshIndex(sourceIndex, srcParentPath, true, false) //nolint:errcheck
	}

	// Handle DESTINATION indexing
	if !dstIdx.Config.DisableIndexing {
		if isSrcDir {
			go func() {
				RefreshIndex(destIndex, realdst, true, true) //nolint:errcheck
				parentDir := filepath.Dir(realdst)
				RefreshIndex(destIndex, parentDir, true, false) //nolint:errcheck
			}()
		} else {
			parentDir := filepath.Dir(realdst)
			go RefreshIndex(destIndex, parentDir, true, false) //nolint:errcheck
		}
	}

	// Use backend source paths to match how shares are stored
	go s.UpdateShares(srcIdx.Path, srcIdx.MakeIndexPath(realsrc), dstIdx.Path, dstIdx.MakeIndexPath(realdst)) //nolint:errcheck

	// Update access rules for the moved path
	if a != nil {
		// If moving within the same source, update the rules
		if srcIdx.Path == dstIdx.Path {
			go a.UpdateRules(srcIdx.Path, srcIdx.MakeIndexPath(realsrc), dstIdx.MakeIndexPath(realdst)) //nolint:errcheck
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
		return err
	}

	err := fileutils.CopyFile(realsrc, realdst)
	if err != nil {
		return err
	}

	// For copy operations:
	// 1. Shallow refresh of source (just to update access times if needed)
	// 2. Recursively index the destination to capture the entire copied tree

	// Refresh source (parent directory if it's a file)
	srcRefreshPath := realsrc
	srcRefreshIsDir := isSrcDir
	if !isSrcDir {
		srcRefreshPath = filepath.Dir(realsrc)
		srcRefreshIsDir = true
	}

	go RefreshIndex(sourceIndex, srcRefreshPath, srcRefreshIsDir, false) //nolint:errcheck

	// Refresh destination (parent directory if it's a file)
	dstRefreshPath := realdst
	if !isSrcDir {
		// If copying a file (regardless of destination), refresh the parent directory
		dstRefreshPath = filepath.Dir(realdst)
	}

	go RefreshIndex(destIndex, dstRefreshPath, true, true) //nolint:errcheck

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

	return RefreshIndex(idx.Name, opts.Path, true, true)
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

	// Explicitly set directory permissions to bypass umask
	err = os.Chmod(realPath, fileutils.PermDir)
	if err != nil {
		// Handle chmod error gracefully
		logger.Debugf("Could not set file permissions for %s (this may be expected in restricted environments): %v", realPath, err)
	}

	return RefreshIndex(source, path, false, false)
}

// getContent reads and returns the file content if it's considered an editable text file.
func getContent(realPath string) (string, error) {
	const headerSize = 4096
	// Thresholds for detecting binary-like content (these can be tuned)
	const maxNullBytesInHeaderAbs = 10    // Max absolute null bytes in header
	const maxNullByteRatioInHeader = 0.1  // Max 10% null bytes in header
	const maxNullByteRatioInFile = 0.05   // Max 5% null bytes in the entire file
	const maxNonPrintableRuneRatio = 0.05 // Max 5% non-printable runes in the entire file

	// Open file
	f, err := os.Open(realPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read header
	headerBytes := make([]byte, headerSize)
	n, err := f.Read(headerBytes)
	if err != nil && err != io.EOF {
		return "", err
	}
	actualHeader := headerBytes[:n]

	// --- Start of new heuristic checks ---

	if n > 0 {
		// Trim header to last complete UTF-8 rune to avoid false negatives
		// when the header read cuts off in the middle of a multi-byte sequence.
		// We decode runes from the end until we find a valid one, trimming
		// any incomplete sequences at the end.
		trimmedHeader := actualHeader
		for len(trimmedHeader) > 0 {
			lastRune, size := utf8.DecodeLastRune(trimmedHeader)
			if lastRune != utf8.RuneError {
				// Found a valid complete rune
				break
			}
			// RuneError occurred - this could be an incomplete sequence or invalid byte
			// Trim the last byte and try again
			if size == 1 && len(trimmedHeader) > 0 {
				trimmedHeader = trimmedHeader[:len(trimmedHeader)-1]
			} else {
				// Shouldn't happen, but break to avoid infinite loop
				break
			}
		}

		// 1. Basic Check: Is the header valid UTF-8?
		// If not, it's unlikely an editable UTF-8 text file.
		// Use trimmed header to avoid false negatives from truncated sequences
		if len(trimmedHeader) > 0 && !utf8.Valid(trimmedHeader) {
			return "", nil // Not an error, just not the text file we want
		}

		// 2. Check for excessive null bytes in the header
		nullCountInHeader := 0
		for _, b := range actualHeader {
			if b == 0x00 {
				nullCountInHeader++
			}
		}
		// Reject if too many nulls absolutely or relatively in the header
		if nullCountInHeader > 0 { // Only perform check if there are any nulls
			if nullCountInHeader > maxNullBytesInHeaderAbs ||
				(float64(nullCountInHeader)/float64(n) > maxNullByteRatioInHeader) {
				return "", nil // Too many nulls in header
			}
		}

		// 3. Check for other non-text ASCII control characters in the header
		// (C0 controls excluding \t, \n, \r)
		for _, b := range actualHeader {
			if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
				return "", nil // Found problematic control character
			}
			// C1 control characters (0x80-0x9F) would be caught by utf8.Valid if part of invalid sequences,
			// or by the non-printable rune check later if they form valid (but undesirable) codepoints.
		}

		// Optional: Use http.DetectContentType for an additional check on the header
		// contentType := http.DetectContentType(actualHeader)
		// if !strings.HasPrefix(contentType, "text/") && contentType != "application/octet-stream" {
		//     // If it's clearly a non-text MIME type (e.g., "image/jpeg"), reject it.
		//     // "application/octet-stream" is ambiguous, so we rely on other heuristics.
		//     return "", nil
		// }
	}
	// --- End of new heuristic checks for header ---

	// Now read the full file (original logic)
	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}
	// Handle empty file (original logic - returns specific string)
	if len(content) == 0 {
		return "empty-file-x6OlSil", nil
	}

	stringContent := string(content)

	// 4. Final UTF-8 validation for the entire file
	// (This is crucial as the header might be fine, but the rest of the file isn't)
	if !utf8.ValidString(stringContent) {
		return "", nil
	}

	// 5. Check for excessive null bytes in the entire file content
	if len(content) > 0 { // Check only for non-empty files
		totalNullCount := 0
		for _, b := range content {
			if b == 0x00 {
				totalNullCount++
			}
		}
		if float64(totalNullCount)/float64(len(content)) > maxNullByteRatioInFile {
			return "", nil // Too many nulls in the entire file
		}
	}

	// 6. Check for excessive non-printable runes in the entire file content
	// (Excluding tab, newline, carriage return, which are common in text files)
	if len(stringContent) > 0 { // Check only for non-empty strings
		nonPrintableRuneCount := 0
		totalRuneCount := 0
		for _, r := range stringContent {
			totalRuneCount++
			// unicode.IsPrint includes letters, numbers, punctuation, symbols, and spaces.
			// It excludes control characters. We explicitly allow \t, \n, \r.
			if !unicode.IsPrint(r) && r != '\t' && r != '\n' && r != '\r' {
				nonPrintableRuneCount++
			}
		}

		if totalRuneCount > 0 { // Avoid division by zero
			if float64(nonPrintableRuneCount)/float64(totalRuneCount) > maxNonPrintableRuneRatio {
				return "", nil // Too many non-printable runes
			}
		}
	}

	// The file has passed all checks and is considered editable text.
	return stringContent, nil
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
