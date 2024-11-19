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
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

var (
	pathMutexes   = make(map[string]*sync.Mutex)
	pathMutexesMu sync.Mutex // Mutex to protect the pathMutexes map
)

type ReducedItem struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modified"`
	Type    string    `json:"type"`
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	Files     map[string]FileInfo `json:"-"`
	Dirs      map[string]FileInfo `json:"-"`
	Path      string              `json:"path"`
	Name      string              `json:"name"`
	Items     []ReducedItem       `json:"items"`
	Size      int64               `json:"size"`
	Extension string              `json:"-"`
	ModTime   time.Time           `json:"modified"`
	CacheTime time.Time           `json:"-"`
	Mode      os.FileMode         `json:"-"`
	IsSymlink bool                `json:"isSymlink,omitempty"`
	Type      string              `json:"type"`
	Subtitles []string            `json:"subtitles,omitempty"`
	Content   string              `json:"content,omitempty"`
	Checksums map[string]string   `json:"checksums,omitempty"`
	Token     string              `json:"token,omitempty"`
	NumDirs   int                 `json:"numDirs"`
	NumFiles  int                 `json:"numFiles"`
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

func FileInfoFaster(opts FileOptions) (FileInfo, error) {
	index := GetIndex(rootPath)
	opts.Path = index.makeIndexPath(opts.Path)

	// Lock access for the specific path
	pathMutex := getMutex(opts.Path)
	pathMutex.Lock()
	defer pathMutex.Unlock()
	if !opts.Checker.Check(opts.Path) {
		return FileInfo{}, os.ErrPermission
	}
	_, isDir, err := GetRealPath(opts.Path)
	if err != nil {
		return FileInfo{}, err
	}
	opts.IsDir = isDir
	// check if the file exists in the index
	info, exists := index.GetMetadataInfo(opts.Path, opts.IsDir)
	if exists {
		// Let's not refresh if less than a second has passed
		if time.Since(info.CacheTime) > time.Second {
			go RefreshFileInfo(opts) //nolint:errcheck
		}
		if info.Path == "" {
			info.Path = "/"
		}
		if opts.Content {
			err = info.addContent(opts.Path)
			if err != nil {
				return info, err
			}
		}
		// refresh cache after
		return info, nil
	}
	err = RefreshFileInfo(opts)
	if err != nil {
		return FileInfo{}, err
	}
	info, exists = index.GetMetadataInfo(opts.Path, opts.IsDir)
	if !exists {
		return FileInfo{}, err
	}
	if opts.Content {
		err = info.addContent(opts.Path)
		if err != nil {
			return info, err
		}
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
	}

	file, err := stat(refreshOptions)
	if err != nil {
		return fmt.Errorf("File/folder does not exist to refresh data: %s", opts.Path)
	}
	fmt.Println("newly refreshed : ", refreshOptions.Path, file.Path)
	result := index.UpdateFileMetadata(opts.Path, *file)
	if !result {
		return fmt.Errorf("File/folder does not exist in metadata: %s", opts.Path)
	}
	return nil
}

func stat(opts FileOptions) (*FileInfo, error) {
	index := GetIndex(rootPath)
	realPath, _, err := GetRealPath(rootPath, opts.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(realPath)
	if err != nil {
		return nil, err
	}
	file := &FileInfo{
		Path:      opts.Path,
		Name:      filepath.Base(opts.Path),
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		Size:      info.Size(),
		Extension: filepath.Ext(info.Name()),
		Token:     opts.Token,
		CacheTime: time.Now(),
	}

	if info.IsDir() {
		file.Type = "directory"

		// Open and read directory contents
		dir, err := os.Open(realPath)
		if err != nil {
			return nil, err
		}
		defer dir.Close()

		dirInfo, err := dir.Stat()
		if err != nil {
			return nil, err
		}

		// Check cached metadata to decide if refresh is needed
		cachedInfo, exists := index.GetMetadataInfo(opts.Path, true)
		if exists && dirInfo.ModTime().Before(cachedInfo.CacheTime) {
			return &cachedInfo, nil
		}

		// Read directory contents and process
		files, err := dir.Readdir(-1)
		if err != nil {
			return nil, err
		}

		file.Files = map[string]FileInfo{}
		file.Dirs = map[string]FileInfo{}

		var totalSize int64
		for _, item := range files {
			itemPath := filepath.Join(realPath, item.Name())
			itemInfo := FileInfo{
				Name:      item.Name(),
				Size:      item.Size(),
				ModTime:   item.ModTime(),
				Mode:      item.Mode(),
				CacheTime: time.Now(),
			}
			isInvalidLink := false
			if IsSymlink(item.Mode()) {
				fmt.Println("is sym link?")
				itemInfo.IsSymlink = true
				info, err := os.Stat(itemPath)
				if err == nil {
					item = info
				} else {
					isInvalidLink = true
				}
			}

			if item.IsDir() {
				itemInfo.Type = "directory"
				file.Dirs[item.Name()] = itemInfo
				file.NumDirs++
			} else {
				if isInvalidLink {
					file.Type = "invalid_link"
				} else {
					err := itemInfo.detectType(itemPath, true, opts.Content, opts.ReadHeader)
					if err != nil {
						fmt.Printf("failed to detect type for %v: %v \n", itemPath, err)
					}
				}
				file.Files[item.Name()] = itemInfo
				file.NumFiles++
			}

			totalSize += itemInfo.Size
		}

		file.Size = totalSize
	}

	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {
	if i.Type == "directory" {
		return errors.ErrIsDirectory
	}

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
	// Convert relative path to absolute path
	absolutePath, err := filepath.Abs(joinedPath)
	if err != nil {
		return absolutePath, false, fmt.Errorf("could not get real path: %v, %s", combined, err)
	}
	// Resolve symlinks and get the real path
	return resolveSymlinks(absolutePath)
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
	dst := opts.Path
	parentDir := filepath.Dir(dst)
	// Split the directory from the destination path
	dir := filepath.Dir(dst)

	// Create the directory and all necessary parents
	err := os.MkdirAll(dir, 0775)
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
	err = RefreshFileInfo(opts)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
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
func (i *FileInfo) addContent(path string) error {
	realPath, _, err := GetRealPath(rootPath, path)
	if err != nil {
		return err
	}

	if i.Type != "directory" {
		fmt.Println("getting content", realPath)
		content, err := os.ReadFile(realPath)
		if err != nil {
			return err
		}
		stringContent := string(content)
		if !utf8.ValidString(stringContent) {
			return nil
		}
		if stringContent == "" {
			i.Content = "empty-file-x6OlSil"
			return nil
		}
		i.Content = stringContent
	}
	return nil
}

// detectType detects the file type.
func (i *FileInfo) detectType(path string, modify, saveContent, readHeader bool) error {
	if i.Type == "directory" {
		return nil
	}
	name := filepath.Base(path)

	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		if saveContent {
			return i.addContent(path)
		}
		return nil
	}

	var buffer []byte
	if readHeader {
		buffer = i.readFirstBytes(path)
		mimetype := mime.TypeByExtension(i.Extension)
		if mimetype == "" {
			http.DetectContentType(buffer)
		}
	}

	ext := filepath.Ext(name)
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
				return i.addContent(path)
			}
		case "video":
			parentDir := strings.TrimRight(path, name)
			i.detectSubtitles(parentDir)
		case "doc":
			if ext == ".pdf" {
				i.Type = "pdf"
				return nil
			}
			if saveContent {
				return i.addContent(path)
			}
		}
	}
	if i.Type == "" {
		i.Type = "blob"
		if saveContent {
			return i.addContent(path)
		}
	}

	return nil
}

// readFirstBytes reads the first bytes of the file.
func (i *FileInfo) readFirstBytes(path string) []byte {
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

// detectSubtitles detects subtitles for video files.
func (i *FileInfo) detectSubtitles(path string) {
	if i.Type != "video" {
		return
	}
	parentDir := filepath.Dir(path)
	fileName := filepath.Base(path)
	i.Subtitles = []string{}
	ext := filepath.Ext(fileName)
	dir, err := os.Open(parentDir)
	if err != nil {
		// Directory must have been deleted, remove it from the index
		return
	}
	defer dir.Close() // Ensure directory handle is closed

	files, err := dir.Readdir(-1)
	if err != nil {
		return
	}

	base := strings.TrimSuffix(fileName, ext)
	subtitleExts := []string{".vtt", ".txt", ".srt", ".lrc"}

	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), base) {
			continue
		}

		for _, subtitleExt := range subtitleExts {
			if strings.HasSuffix(f.Name(), subtitleExt) {
				i.Subtitles = append(i.Subtitles, filepath.Join(parentDir, f.Name()))
				break
			}
		}
	}
}

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
