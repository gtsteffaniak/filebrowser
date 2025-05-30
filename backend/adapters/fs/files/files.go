package files

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"unicode"
	"unicode/utf8"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

func FileInfoFaster(opts iteminfo.FileOptions) (iteminfo.ExtendedFileInfo, error) {
	response := iteminfo.ExtendedFileInfo{}
	if opts.Source == "" {
		opts.Source = settings.Config.Server.DefaultSource.Name
	}
	index := indexing.GetIndex(opts.Source)
	if index == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, isDir, err := index.GetRealPath(opts.Path)
	if err != nil {
		return response, err
	}
	opts.IsDir = isDir
	// TODO: whats the best way to save trips to disk here?
	// disabled using cache because its not clear if this is helping or hurting
	// check if the file exists in the index
	//info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	//if exists {
	//	err := RefreshFileInfo(opts)
	//	if err != nil {
	//		return info, err
	//	}
	//	if opts.Content {
	//		content := ""
	//		content, err = getContent(opts.Path)
	//		if err != nil {
	//			return info, err
	//		}
	//		info.Content = content
	//	}
	//	return info, nil
	//}
	err = index.RefreshFileInfo(opts)
	if err != nil {
		return response, err
	}
	info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	if !exists {
		return response, fmt.Errorf("could not get metadata for path: %v", opts.Path)
	}
	if opts.Content {
		if info.Size < 20*1024*1024 { // 20 megabytes in bytes
			content, err := getContent(realPath)
			if err != nil {
				logger.Debugf("could not get content for file: "+info.Path, info.Name, err)
				return response, err
			}
			response.Content = content
		} else {
			logger.Debug("skipping large text file contents (20MB limit): "+info.Path, info.Name)
		}
	}
	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source
	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}
	if strings.HasPrefix(info.Type, "video") {
		parentInfo, exists := index.GetReducedMetadata(filepath.Dir(info.Path), true)
		if exists {
			response.DetectSubtitles(parentInfo)
		}
	}
	return response, nil
}

func generateOfficeId(realPath string) string {
	key, ok := utils.OnlyOfficeCache.Get(realPath).(string)
	if !ok {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		documentKey := utils.HashSHA256(realPath + timestamp)
		utils.OnlyOfficeCache.Set(realPath, documentKey)
		return documentKey
	}
	return key
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func GetChecksum(fullPath, algo string) (map[string]string, error) {
	subs := map[string]string{}
	reader, err := os.Open(fullPath)
	if err != nil {
		return subs, err
	}
	defer reader.Close()

	hashFuncs := map[string]hash.Hash{
		"md5":    md5.New(),
		"sha1":   sha1.New(),
		"sha256": sha256.New(),
		"sha512": sha512.New(),
	}

	h, ok := hashFuncs[algo]
	if !ok {
		return subs, errors.ErrInvalidOption
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return subs, err
	}
	subs[algo] = hex.EncodeToString(h.Sum(nil))
	return subs, nil
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
	refreshConfig := iteminfo.FileOptions{Path: index.MakeIndexPath(absDirPath), IsDir: true}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return err
	}
	return nil
}

func MoveResource(sourceIndex, destIndex, realsrc, realdst string) error {
	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}
	idxSrc := indexing.GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := indexing.GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	// refresh info for source and dest
	err = idxSrc.RefreshFileInfo(iteminfo.FileOptions{
		Path:  refreshSourceDir,
		IsDir: true,
	})
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	if refreshSourceDir == refreshDestDir {
		return nil
	}
	refreshConfig := iteminfo.FileOptions{Path: refreshDestDir, IsDir: true}
	err = idxDst.RefreshFileInfo(refreshConfig)
	if err != nil {
		return fmt.Errorf("could not refresh index for dest: %v", err)
	}
	return nil
}

func CopyResource(sourceIndex, destIndex, realsrc, realdst string) error {
	err := fileutils.CopyFile(realsrc, realdst)
	if err != nil {
		return err
	}
	idxSrc := indexing.GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := indexing.GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	index := indexing.GetIndex(sourceIndex)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshConfig := iteminfo.FileOptions{Path: refreshSourceDir, IsDir: true}
	// refresh info for source and dest
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	refreshConfig.Path = refreshDestDir
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return errors.ErrEmptyKey
	}

	return nil
}

func WriteDirectory(opts iteminfo.FileOptions) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, _, _ := idx.GetRealPath(opts.Path)
	// Ensure the parent directories exist
	err := os.MkdirAll(realPath, 0775)
	if err != nil {
		return err
	}
	err = idx.RefreshFileInfo(opts)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteFile(opts iteminfo.FileOptions, in io.Reader) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	dst, _, _ := idx.GetRealPath(opts.Path)
	parentDir := filepath.Dir(dst)
	// Create the directory and all necessary parents
	err := os.MkdirAll(parentDir, 0775)
	if err != nil {
		return err
	}

	// Open the file for writing (create if it doesn't exist, truncate if it does)
	file, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy the contents from the reader to the file
	_, err = io.Copy(file, in)
	if err != nil {
		return err
	}
	opts.Path = idx.MakeIndexPath(parentDir)
	opts.IsDir = true
	return idx.RefreshFileInfo(opts)
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
