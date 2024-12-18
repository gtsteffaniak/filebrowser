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
	"mime"
	"net/http"

	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

var (
	pathMutexes   = make(map[string]*sync.Mutex)
	pathMutexesMu sync.Mutex // Mutex to protect the pathMutexes map
)

type ItemInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modified"`
	Type    string    `json:"type"`
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	ItemInfo
	Files   []ItemInfo `json:"files"`
	Folders []ItemInfo `json:"folders"`
	Path    string     `json:"path"`
}

// for efficiency, a response will be a pointer to the data
// extra calculated fields can be added here
type ExtendedFileInfo struct {
	*FileInfo
	Content   string            `json:"content,omitempty"`
	Subtitles []string          `json:"subtitles,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
	Token     string            `json:"token,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Path       string // realpath
	IsDir      bool
	Modify     bool
	Expand     bool
	ReadHeader bool
	Checker    users.Checker
	Content    bool
}

func (f FileOptions) Components() (string, string) {
	return filepath.Dir(f.Path), filepath.Base(f.Path)
}

func FileInfoFaster(opts FileOptions) (ExtendedFileInfo, error) {
	index := GetIndex(rootPath)
	opts.Path = index.makeIndexPath(opts.Path)
	response := ExtendedFileInfo{}
	// Lock access for the specific path
	pathMutex := getMutex(opts.Path)
	pathMutex.Lock()
	defer pathMutex.Unlock()
	if !opts.Checker.Check(opts.Path) {
		return response, os.ErrPermission
	}

	_, isDir, err := GetRealPath(opts.Path)
	if err != nil {
		return response, err
	}
	opts.IsDir = isDir
	// TODO : whats the best way to save trips to disk here?
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
		return response, err
	}
	if opts.Content {
		content, err := getContent(opts.Path)
		if err != nil {
			return response, err
		}
		response.Content = content
	}
	response.FileInfo = info
	return response, nil
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

// RealPath gets the real path for the file, resolving symlinks if supported.
func (i *FileInfo) RealPath() string {
	realPath, _, _ := GetRealPath(rootPath, i.Path)
	realPath, err := filepath.EvalSymlinks(realPath)
	if err == nil {
		return realPath
	}
	return i.Path
}

func GetRealPath(relativePath ...string) (string, bool, error) {
	combined := []string{settings.Config.Server.Root}
	for _, path := range relativePath {
		combined = append(combined, strings.TrimPrefix(path, settings.Config.Server.Root))
	}
	joinedPath := filepath.Join(combined...)

	isDir, _ := utils.RealPathCache.Get(joinedPath + ":isdir").(bool)
	cached, ok := utils.RealPathCache.Get(joinedPath).(string)
	if ok && cached != "" {
		return cached, isDir, nil
	}
	// Convert relative path to absolute path
	absolutePath, err := filepath.Abs(joinedPath)
	if err != nil {
		return absolutePath, false, fmt.Errorf("could not get real path: %v, %s", combined, err)
	}
	// Resolve symlinks and get the real path
	realPath, isDir, err := resolveSymlinks(absolutePath)
	if err == nil {
		utils.RealPathCache.Set(joinedPath, realPath)
		utils.RealPathCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func DeleteFiles(absPath string, opts FileOptions) error {
	err := os.RemoveAll(absPath)
	if err != nil {
		return err
	}
	index := GetIndex(rootPath)
	refreshConfig := FileOptions{Path: filepath.Dir(opts.Path), IsDir: true}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return err
	}
	return nil
}

func MoveResource(realsrc, realdst string, isSrcDir bool) error {
	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}
	index := GetIndex(rootPath)
	// refresh info for source and dest
	err = index.RefreshFileInfo(FileOptions{
		Path:  realsrc,
		IsDir: isSrcDir,
	})
	if err != nil {
		return errors.ErrEmptyKey
	}
	refreshConfig := FileOptions{Path: realdst, IsDir: true}
	if !isSrcDir {
		refreshConfig.Path = filepath.Dir(realdst)
	}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func CopyResource(realsrc, realdst string, isSrcDir bool) error {
	err := fileutils.CopyFile(realsrc, realdst)
	if err != nil {
		return err
	}
	index := GetIndex(rootPath)
	refreshConfig := FileOptions{Path: realdst, IsDir: true}
	if !isSrcDir {
		refreshConfig.Path = filepath.Dir(realdst)
	}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteDirectory(opts FileOptions) error {
	realPath, _, _ := GetRealPath(rootPath, opts.Path)
	// Ensure the parent directories exist
	err := os.MkdirAll(realPath, 0775)
	if err != nil {
		return err
	}
	index := GetIndex(rootPath)
	err = index.RefreshFileInfo(opts)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteFile(opts FileOptions, in io.Reader) error {
	dst, _, _ := GetRealPath(rootPath, opts.Path)
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
	opts.Path = parentDir
	opts.IsDir = true
	index := GetIndex(rootPath)
	return index.RefreshFileInfo(opts)
}

// resolveSymlinks resolves symlinks in the given path
func resolveSymlinks(path string) (string, bool, error) {
	for {
		// Get the file info using os.Lstat to handle symlinks
		info, err := os.Lstat(path)
		if err != nil {
			return path, false, fmt.Errorf("could not stat path: %s, %v", path, err)
		}

		// Check if the path is a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the symlink target
			target, err := os.Readlink(path)
			if err != nil {
				return path, false, fmt.Errorf("could not read symlink: %s, %v", path, err)
			}

			// Resolve the symlink's target relative to its directory
			// This ensures the resolved path is absolute and correctly calculated
			path = filepath.Join(filepath.Dir(path), target)
		} else {
			// Not a symlink, so return the resolved path and whether it's a directory
			isDir := info.IsDir()
			return path, isDir, nil
		}
	}
}

// addContent reads and sets content based on the file type.
func getContent(path string) (string, error) {
	realPath, _, err := GetRealPath(rootPath, path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}
	stringContent := string(content)
	if !utf8.ValidString(stringContent) {
		return "", nil
	}
	if stringContent == "" {
		return "empty-file-x6OlSil", nil
	}
	return stringContent, nil
}

// DetectType detects the MIME type of a file and updates the ItemInfo struct.
func (i *ItemInfo) DetectType(path string, saveContent bool) {
	name := i.Name
	ext := filepath.Ext(name)

	// Attempt MIME detection by file extension
	i.Type = strings.Split(mime.TypeByExtension(ext), ";")[0]
	if i.Type == "" {
		i.Type = extendedMimeTypeCheck(ext)
	}
	if i.Type == "blob" {
		realpath, _, _ := GetRealPath(path)
		// Read only the first 512 bytes for efficient MIME detection
		file, err := os.Open(realpath)
		if err != nil {

		} else {
			defer file.Close()
			buffer := make([]byte, 512)
			n, _ := file.Read(buffer) // Ignore errors from Read
			i.Type = strings.Split(http.DetectContentType(buffer[:n]), ";")[0]
		}
	}
}

// TODO add subtitles back
// detectSubtitles detects subtitles for video files.
//func (i *FileInfo) detectSubtitles(path string) {
//	if i.Type != "video" {
//		return
//	}
//	parentDir := filepath.Dir(path)
//	fileName := filepath.Base(path)
//	i.Subtitles = []string{}
//	ext := filepath.Ext(fileName)
//	dir, err := os.Open(parentDir)
//	if err != nil {
//		// Directory must have been deleted, remove it from the index
//		return
//	}
//	defer dir.Close() // Ensure directory handle is closed
//
//	files, err := dir.Readdir(-1)
//	if err != nil {
//		return
//	}
//
//	base := strings.TrimSuffix(fileName, ext)
//	subtitleExts := []string{".vtt", ".txt", ".srt", ".lrc"}
//
//	for _, f := range files {
//		if f.IsDir() || !strings.HasPrefix(f.Name(), base) {
//			continue
//		}
//
//		for _, subtitleExt := range subtitleExts {
//			if strings.HasSuffix(f.Name(), subtitleExt) {
//				i.Subtitles = append(i.Subtitles, filepath.Join(parentDir, f.Name()))
//				break
//			}
//		}
//	}
//}

func IsNamedPipe(mode os.FileMode) bool {
	return mode&os.ModeNamedPipe != 0
}

func IsSymlink(mode os.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

func getMutex(path string) *sync.Mutex {
	// Lock access to pathMutexes map
	pathMutexesMu.Lock()
	defer pathMutexesMu.Unlock()

	// Create a mutex for the path if it doesn't exist
	if pathMutexes[path] == nil {
		pathMutexes[path] = &sync.Mutex{}
	}

	return pathMutexes[path]
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

func (info *FileInfo) SortItems() {
	sort.Slice(info.Folders, func(i, j int) bool {
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(info.Folders[i].Name)
		numJ, errJ := strconv.Atoi(info.Folders[j].Name)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Folders[i].Name) < strings.ToLower(info.Folders[j].Name)
	})
	sort.Slice(info.Files, func(i, j int) bool {
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(info.Files[i].Name)
		numJ, errJ := strconv.Atoi(info.Files[j].Name)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Files[i].Name) < strings.ToLower(info.Files[j].Name)
	})
}
