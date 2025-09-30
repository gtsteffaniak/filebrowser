package files

import (
	"encoding/base64"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
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
		return response, fmt.Errorf("could not get real path for requested path: %v", opts.Path)
	}
	if !strings.HasSuffix(opts.Path, "/") && isDir {
		opts.Path = opts.Path + "/"
	}
	opts.IsDir = isDir
	var info *iteminfo.FileInfo
	var exists bool
	var useFsDirInfo bool
	if isDir {
		err = index.RefreshFileInfo(opts)
		if err != nil {
			if err == errors.ErrNotIndexed && index.Config.DisableIndexing {
				useFsDirInfo = true
			} else if err == errors.ErrNotIndexed {
				return response, fmt.Errorf("could not refresh file info: %v", err)
			}
		}
	}
	if useFsDirInfo {
		info, err = index.GetFsDirInfo(opts.Path)
		if err != nil {
			return response, err
		}
	} else {
		info, exists = index.GetReducedMetadata(opts.Path, opts.IsDir)
		if !exists {
			err = index.RefreshFileInfo(opts)
			if err != nil {
				return response, fmt.Errorf("could not refresh file info: %v", err)
			}
			info, exists = index.GetReducedMetadata(opts.Path, opts.IsDir)
			if !exists {
				return response, fmt.Errorf("could not get metadata for path: %v", opts.Path)
			}
		}
	}

	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source

	if access != nil && !access.Permitted(index.Path, opts.Path, opts.Username) {
		// User doesn't have access to the current folder, but check if they have access to any subitems
		// This allows specific allow rules on subfolders/files to work even when parent is denied
		err := access.CheckChildItemAccess(response, index, opts.Username)
		if err != nil {
			return response, err
		}
	}
	if opts.Content || opts.Metadata {
		processContent(response, index)
	}
	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}

	return response, nil
}

func processContent(info *iteminfo.ExtendedFileInfo, idx *indexing.Index) {
	isVideo := strings.HasPrefix(info.Type, "video")
	isAudio := strings.HasPrefix(info.Type, "audio")
	isFolder := info.Type == "directory"
	if isFolder {
		return
	}

	if isVideo {
		parentPath := filepath.Dir(info.Path)
		parentInfo, exists := idx.GetReducedMetadata(parentPath, true)
		if exists {
			info.DetectSubtitles(parentInfo)
			err := info.LoadSubtitleContent()
			if err != nil {
				logger.Debug("failed to load subtitle content: " + err.Error())
			}
		}
		return
	}

	if isAudio {
		err := extractAudioMetadata(info)
		if err != nil {
			logger.Debugf("failed to extract audio metadata for file: "+info.RealPath, info.Name, err)
		} else {
			info.HasPreview = info.AudioMeta.AlbumArt != ""
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
		logger.Debug("skipping large text file contents (20MB limit): "+info.Path, info.Name)
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
func extractAudioMetadata(item *iteminfo.ExtendedFileInfo) error {
	file, err := os.Open(item.RealPath)
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

	item.AudioMeta = &iteminfo.AudioMetadata{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
		Year:   m.Year(),
		Genre:  m.Genre(),
	}

	// Extract track number
	track, _ := m.Track()
	item.AudioMeta.Track = track

	// Extract album art and encode as base64 with strict size limits
	if picture := m.Picture(); picture != nil && picture.Data != nil {
		// More aggressive size limit to prevent memory issues (max 2MB)
		if len(picture.Data) <= 2*1024*1024 {
			item.AudioMeta.AlbumArt = base64.StdEncoding.EncodeToString(picture.Data)
		} else {
			logger.Debugf("Skipping album art for %s: too large (%d bytes)", item.RealPath, len(picture.Data))
		}
	}

	return nil
}

func DeleteFiles(source, absPath string, absDirPath string) error {
	err := os.RemoveAll(absPath)
	if err != nil {
		return err
	}
	index := indexing.GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	if index.Config.DisableIndexing {
		return nil
	}

	// Clear RealPathCache entries for the deleted path to prevent cache issues
	// when a folder with the same name is created later
	indexPath := index.MakeIndexPath(absPath)
	realPath, _, err := index.GetRealPath(indexPath)
	if err == nil {
		// Clear both the path and the isdir cache entries
		indexing.RealPathCache.Delete(realPath)
		indexing.RealPathCache.Delete(realPath + ":isdir")
	}

	refreshConfig := utils.FileOptions{Path: index.MakeIndexPath(absDirPath), IsDir: true}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return err
	}
	return nil
}

func RefreshIndex(source string, path string, isDir bool, recursive bool) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	if idx.Config.DisableIndexing {
		return nil
	}
	// Only use MakeIndexPath for directory operations to ensure trailing slashes
	if isDir {
		path = idx.MakeIndexPath(path)
	}
	return idx.RefreshFileInfo(utils.FileOptions{Path: path, IsDir: isDir, Recursive: recursive})
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

func MoveResource(isSrcDir, isDestDir bool, sourceIndex, destIndex, realsrc, realdst string, s *share.Storage) error {
	// Validate the move operation before executing
	if err := validateMoveDestination(realsrc, realdst, isSrcDir); err != nil {
		return err
	}

	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}

	// For move operations:
	// 1. Delete the source from the index (recursively if it's a directory)
	// 2. Recursively index the destination to capture the entire moved tree

	// Get indexes for deletion and refresh operations
	srcIdx := indexing.GetIndex(sourceIndex)
	if srcIdx == nil {
		return fmt.Errorf("could not get source index: %v", sourceIndex)
	}
	dstIdx := indexing.GetIndex(destIndex)
	if dstIdx == nil {
		return fmt.Errorf("could not get destination index: %v", destIndex)
	}

	// Delete from source index (recursively for directories)
	go srcIdx.DeleteMetadata(realsrc, isSrcDir, isSrcDir) //nolint:errcheck

	// Clear RealPathCache entries for the moved path to prevent cache issues
	// when a file/folder with the same name is created later
	srcIndexPath := srcIdx.MakeIndexPath(realsrc)
	srcRealPath, _, err := srcIdx.GetRealPath(srcIndexPath)
	if err == nil {
		// Clear both the path and the isdir cache entries
		indexing.RealPathCache.Delete(srcRealPath)
		indexing.RealPathCache.Delete(srcRealPath + ":isdir")
	}

	// For move operations, refresh the parent directory to capture the moved file
	refreshPath := realdst
	refreshIsDir := isDestDir
	if !isSrcDir {
		// If moving a file (regardless of destination), refresh the parent directory
		refreshPath = filepath.Dir(realdst)
		refreshIsDir = true
	}

	go RefreshIndex(destIndex, refreshPath, refreshIsDir, true) //nolint:errcheck

	// Use backend source paths to match how shares are stored
	go s.UpdateShares(srcIdx.Path, srcIdx.MakeIndexPath(realsrc), dstIdx.Path, dstIdx.MakeIndexPath(realdst)) //nolint:errcheck
	return nil
}

func CopyResource(isSrcDir, isDestDir bool, sourceIndex, destIndex, realsrc, realdst string) error {
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
	dstRefreshIsDir := isDestDir
	if !isSrcDir {
		// If copying a file (regardless of destination), refresh the parent directory
		dstRefreshPath = filepath.Dir(realdst)
		dstRefreshIsDir = true
	}

	go RefreshIndex(destIndex, dstRefreshPath, dstRefreshIsDir, true) //nolint:errcheck
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
		// Clear the cache for the removed file
		realPath, _, err = idx.GetRealPath(opts.Path)
		if err == nil {
			indexing.RealPathCache.Delete(realPath)
			indexing.RealPathCache.Delete(realPath + ":isdir")
		}
	}

	// Ensure the parent directories exist
	err = os.MkdirAll(realPath, fileutils.PermDir)
	if err != nil {
		return err
	}

	// Explicitly set directory permissions to bypass umask
	err = os.Chmod(realPath, fileutils.PermDir)
	if err != nil {
		return err
	}

	return RefreshIndex(idx.Name, opts.Path, true, true)
}

func WriteFile(opts utils.FileOptions, in io.Reader) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, _, _ := idx.GetRealPath(opts.Path)
	// Ensure the parent directories exist
	err := os.MkdirAll(filepath.Dir(realPath), fileutils.PermDir)
	if err != nil {
		return err
	}
	var stat os.FileInfo
	// Check if the destination exists and is a directory
	if stat, err = os.Stat(realPath); err == nil && stat.IsDir() {
		// If it's a directory and we're trying to create a file, remove the directory first
		err = os.RemoveAll(realPath)
		if err != nil {
			return fmt.Errorf("could not remove existing directory to create file: %v", err)
		}
		// Clear the cache for the removed directory
		realPath, _, err = idx.GetRealPath(opts.Path)
		if err == nil {
			indexing.RealPathCache.Delete(realPath)
			indexing.RealPathCache.Delete(realPath + ":isdir")
		}
	}

	// Open the file for writing (create if it doesn't exist, truncate if it does)
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
		return err
	}

	return RefreshIndex(opts.Source, opts.Path, false, false)
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
		// 1. Basic Check: Is the header valid UTF-8?
		// If not, it's unlikely an editable UTF-8 text file.
		if !utf8.Valid(actualHeader) {
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
