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

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var OnlyOfficeCache = cache.NewCache(48 * time.Hour)

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
	key, ok := OnlyOfficeCache.Get(realPath).(string)
	if !ok {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		documentKey := utils.HashSHA256(realPath + timestamp)
		OnlyOfficeCache.Set(realPath, documentKey)
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

// getContent reads and returns the file content if it's UTF-8 readable.
func getContent(realPath string) (string, error) {
	// Read the entire file in one go. This is more efficient.
	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}

	// Check if the file content is valid UTF-8.
	// This allows for all characters, including ASCII and emojis.
	if !utf8.Valid(content) {
		// File is not valid text (e.g., a binary image), so we reject it.
		return "", nil
	}

	// Handle the special case for an empty file, as in your original code.
	if len(content) == 0 {
		return "empty-file-x6OlSil", nil
	}

	// The file is valid, so return its string content.
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
