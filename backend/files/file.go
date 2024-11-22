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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/fileutils"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/utils"
)

var (
	pathMutexes   = make(map[string]*sync.Mutex)
	pathMutexesMu sync.Mutex // Mutex to protect the pathMutexes map
)

type ReducedItem struct {
	Name    string      `json:"name"`
	Size    int64       `json:"size"`
	ModTime time.Time   `json:"modified"`
	Type    string      `json:"type"`
	Mode    os.FileMode `json:"-"`
	Content string      `json:"content,omitempty"`
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	Files     []ReducedItem     `json:"-"`
	Dirs      []ReducedItem     `json:"-"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Items     []ReducedItem     `json:"items"`
	Size      int64             `json:"size"`
	ModTime   time.Time         `json:"modified"`
	Mode      os.FileMode       `json:"-"`
	Type      string            `json:"type"`
	Subtitles []string          `json:"subtitles,omitempty"`
	Content   string            `json:"content,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Path       string // realpath
	IsDir      bool
	Modify     bool
	Expand     bool
	ReadHeader bool
	Token      string
	Checker    users.Checker
	Content    bool
}

func (f FileOptions) Components() (string, string) {
	return filepath.Dir(f.Path), filepath.Base(f.Path)
}

func FileInfoFaster(opts FileOptions) (*FileInfo, error) {
	index := GetIndex(rootPath)
	opts.Path = index.makeIndexPath(opts.Path)

	// Lock access for the specific path
	pathMutex := getMutex(opts.Path)
	pathMutex.Lock()
	defer pathMutex.Unlock()
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}
	_, isDir, err := GetRealPath(opts.Path)
	if err != nil {
		return nil, err
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
	err = RefreshFileInfo(opts)
	if err != nil {
		return nil, err
	}
	info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	if !exists {
		return nil, err
	}
	if opts.Content {
		content, err := getContent(opts.Path)
		if err != nil {
			return info, err
		}
		info.Content = content
	}
	return info, nil
}

func RefreshFileInfo(opts FileOptions) error {
	refreshOptions := FileOptions{
		Path:  opts.Path,
		IsDir: opts.IsDir,
		Token: opts.Token,
	}
	index := GetIndex(rootPath)

	if !refreshOptions.IsDir {
		refreshOptions.Path = index.makeIndexPath(filepath.Dir(refreshOptions.Path))
		refreshOptions.IsDir = true
	} else {
		refreshOptions.Path = index.makeIndexPath(refreshOptions.Path)
	}

	current, exists := index.GetMetadataInfo(refreshOptions.Path, true)

	file, err := stat(refreshOptions)
	if err != nil {
		return fmt.Errorf("file/folder does not exist to refresh data: %s", refreshOptions.Path)
	}

	//utils.PrintStructFields(*file)
	result := index.UpdateMetadata(file)
	if !result {
		return fmt.Errorf("file/folder does not exist in metadata: %s", refreshOptions.Path)
	}
	if !exists {
		return nil
	}
	if current.Size != file.Size {
		index.recursiveUpdateDirSizes(file, current.Size)
	}
	return nil
}

func stat(opts FileOptions) (*FileInfo, error) {
	realPath, _, err := GetRealPath(rootPath, opts.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(realPath)
	if err != nil {
		return nil, err
	}
	file := &FileInfo{
		Path:    opts.Path,
		Name:    filepath.Base(opts.Path),
		ModTime: info.ModTime(),
		Mode:    info.Mode(),
		Size:    info.Size(),
	}
	if info.IsDir() {
		// Open and read directory contents
		dir, err := os.Open(realPath)
		if err != nil {
			return nil, err
		}
		defer dir.Close()

		// TODO: this is not reliable, because we are not checking the children
		// Check cached metadata to decide if refresh is needed
		//dirInfo, err := dir.Stat()
		//if err != nil {
		//	return nil, err
		//}
		//index := GetIndex(rootPath)
		//// Check cached metadata to decide if refresh is needed
		//cachedParentDir, exists := index.GetMetadataInfo(opts.Path, true)
		//if exists && dirInfo.ModTime().Before(cachedParentDir.CacheTime) {
		//	return cachedParentDir, nil
		//}

		// Read directory contents and process
		files, err := dir.Readdir(-1)
		if err != nil {
			return nil, err
		}

		file.Files = []ReducedItem{}
		file.Dirs = []ReducedItem{}

		var totalSize int64
		for _, item := range files {
			itemPath := filepath.Join(realPath, item.Name())

			if item.IsDir() {
				itemInfo := ReducedItem{
					Name: item.Name(),
				}
				//if exists {
				//// if directory size was already cached use that.
				//cachedDir, ok := cachedParentDir.Dirs[item.Name()]
				//if ok {
				//	itemInfo.Size = cachedDir.Size
				//}
				//}//
				file.Dirs = append(file.Dirs, itemInfo)
				totalSize += itemInfo.Size
			} else {
				itemInfo := ReducedItem{
					Name:    item.Name(),
					Size:    item.Size(),
					ModTime: item.ModTime(),
					Mode:    item.Mode(),
				}
				if IsSymlink(item.Mode()) {
					itemInfo.Type = "symlink"
					info, err := os.Stat(itemPath)
					if err == nil {
						itemInfo.Name = info.Name()
						itemInfo.ModTime = info.ModTime()
						itemInfo.Size = info.Size()
						itemInfo.Mode = info.Mode()
					} else {
						file.Type = "invalid_link"
					}
				}
				if file.Type != "invalid_link" {
					err := itemInfo.detectType(itemPath, true, opts.Content, opts.ReadHeader)
					if err != nil {
						fmt.Printf("failed to detect type for %v: %v \n", itemPath, err)
					}
					file.Files = append(file.Files, itemInfo)
				}
				totalSize += itemInfo.Size

			}
		}

		file.Size = totalSize
	}
	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {

	if i.Checksums == nil {
		i.Checksums = map[string]string{}
	}
	fullpath := filepath.Join(i.Path, i.Name)
	reader, err := os.Open(fullpath)
	if err != nil {
		return err
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
		return errors.ErrInvalidOption
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return err
	}

	i.Checksums[algo] = hex.EncodeToString(h.Sum(nil))
	return nil
}

// RealPath gets the real path for the file, resolving symlinks if supported.
func (i *FileInfo) RealPath() string {
	realPath, err := filepath.EvalSymlinks(i.Path)
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
	err = RefreshFileInfo(opts)
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
	// refresh info for source and dest
	err = RefreshFileInfo(FileOptions{
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
	err = RefreshFileInfo(refreshConfig)
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

	refreshConfig := FileOptions{Path: realdst, IsDir: true}
	if !isSrcDir {
		refreshConfig.Path = filepath.Dir(realdst)
	}
	err = RefreshFileInfo(refreshConfig)
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
	err = RefreshFileInfo(opts)
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
	return RefreshFileInfo(opts)
}

// resolveSymlinks resolves symlinks in the given path
func resolveSymlinks(path string) (string, bool, error) {
	for {
		// Get the file info
		info, err := os.Lstat(path)
		if err != nil {
			return path, false, fmt.Errorf("could not stat path: %v, %s", path, err)
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the symlink target
			target, err := os.Readlink(path)
			if err != nil {
				return path, false, err
			}

			// Resolve the target relative to the symlink's directory
			path = filepath.Join(filepath.Dir(path), target)
		} else {
			// Not a symlink, so return the resolved path and check if it's a directory
			return path, info.IsDir(), nil
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
		return "", fmt.Errorf("file is not utf8 encoded")
	}
	if stringContent == "" {
		return "empty-file-x6OlSil", nil
	}
	return stringContent, nil
}

// detectType detects the file type.
func (i *ReducedItem) detectType(path string, modify, saveContent, readHeader bool) error {
	name := i.Name
	var contentErr error
	var contentString string
	if saveContent {
		contentString, contentErr = getContent(path)
		if contentErr == nil {
			i.Content = contentString
		}
	}

	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		return contentErr
	}

	ext := filepath.Ext(name)
	var buffer []byte
	if readHeader {
		buffer = i.readFirstBytes(path)
		mimetype := mime.TypeByExtension(ext)
		if mimetype == "" {
			http.DetectContentType(buffer)
		}
	}

	for _, fileType := range AllFiletypeOptions {
		if IsMatchingType(ext, fileType) {
			i.Type = fileType
		}
		switch i.Type {
		case "text":
			if !modify {
				i.Type = "textImmutable"
			}
			if saveContent {
				return contentErr
			}
		case "video":
			// TODO add back somewhere else, not during metadata fetch
			//parentDir := strings.TrimRight(path, name)
			//i.detectSubtitles(parentDir)
		case "doc":
			if ext == ".pdf" {
				i.Type = "pdf"
				return nil
			}
			if saveContent {
				return nil
			}
		}
	}
	if i.Type == "" {
		i.Type = "blob"
		if saveContent {
			return contentErr
		}
	}

	return nil
}

// readFirstBytes reads the first bytes of the file.
func (i *ReducedItem) readFirstBytes(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		i.Type = "blob"
		return nil
	}
	defer file.Close()

	buffer := make([]byte, 512) //nolint:gomnd
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		i.Type = "blob"
		return nil
	}

	return buffer[:n]
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
