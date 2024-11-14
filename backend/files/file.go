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
	Files     map[string]*FileInfo `json:"-"`
	Dirs      map[string]*FileInfo `json:"-"`
	Path      string               `json:"path"`
	Name      string               `json:"name"`
	Items     []ReducedItem        `json:"items"`
	Size      int64                `json:"size"`
	Extension string               `json:"-"`
	ModTime   time.Time            `json:"modified"`
	CacheTime time.Time            `json:"-"`
	Mode      os.FileMode          `json:"-"`
	IsSymlink bool                 `json:"isSymlink,omitempty"`
	Type      string               `json:"type"`
	Subtitles []string             `json:"subtitles,omitempty"`
	Content   string               `json:"content,omitempty"`
	Checksums map[string]string    `json:"checksums,omitempty"`
	Token     string               `json:"token,omitempty"`
	NumDirs   int                  `json:"numDirs"`
	NumFiles  int                  `json:"numFiles"`
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

// Legacy file info method, only called on non-indexed directories.
// Once indexing completes for the first time, NewFileInfo is never called.
func NewFileInfo(opts FileOptions) (FileInfo, error) {
	index := GetIndex(rootPath)
	if !opts.Checker.Check(opts.Path) {
		return FileInfo{}, os.ErrPermission
	}

	file, err := stat(opts)
	if err != nil {
		return FileInfo{}, err
	}
	if opts.Expand {
		if file.Type == "directory" {
			if err = file.readListing(opts.Path, opts.Checker, opts.ReadHeader); err != nil {
				return FileInfo{}, err
			}
			metadataInfo, exists := index.GetMetadataInfo(opts.Path, opts.IsDir)
			if !exists {
				return FileInfo{}, errors.ErrNotExist
			}
			return metadataInfo, nil
		}
		err = file.detectType(opts.Path, opts.Modify, opts.Content, true)
		if err != nil {
			return FileInfo{}, err
		}
	}

	return *file, err
}

func FileInfoFaster(opts FileOptions) (FileInfo, error) {
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
	index := GetIndex(rootPath)
	// check if the file exists in the index
	info, exists := index.GetMetadataInfo(opts.Path, opts.IsDir)
	if exists {
		// Let's not refresh if less than a second has passed
		if time.Since(info.CacheTime) > time.Second {
			go RefreshFileInfo(opts) //nolint:errcheck
		}
		// refresh cache after
		return info, nil
	}
	// don't bother caching content
	if opts.Content {
		file, err := NewFileInfo(opts)
		return file, err
	}
	err = RefreshFileInfo(opts)
	if err != nil {
		file, err := NewFileInfo(opts)
		return file, err
	}
	info, exists = index.GetMetadataInfo(opts.Path, opts.IsDir)
	if !exists {
		return NewFileInfo(opts)
	}
	return info, nil
}

func RefreshFileInfo(opts FileOptions) error {
	if !opts.Checker.Check(opts.Path) {
		return fmt.Errorf("permission denied: %s", opts.Path)
	}
	index := GetIndex(rootPath)
	file, err := stat(opts)
	if err != nil {
		return fmt.Errorf("File/folder does not exist to refresh data: %s", opts.Path)
	}
	_ = file.detectType(opts.Path, true, opts.Content, opts.ReadHeader)
	if file.Type == "directory" {
		err := file.readListing(opts.Path, opts.Checker, opts.ReadHeader)
		if err != nil {
			return fmt.Errorf("Dir info could not be read: %s", opts.Path)
		}
	}
	result := index.UpdateFileMetadata(opts.Path, file)
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
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		Size:      info.Size(),
		Extension: filepath.Ext(info.Name()),
		Token:     opts.Token,
	}
	file.Name = filepath.Base(realPath)

	if info.IsDir() {
		file.Type = "directory"

		// Open the directory
		dir, err := os.Open(realPath)
		if err != nil {
			return nil, err
		}
		defer dir.Close()
		dirInfo, err := dir.Stat()
		if err != nil {
			return nil, err
		}

		info, exists := index.GetMetadataInfo(opts.Path, info.IsDir())
		if exists {
			// Check if the directory is already up-to-date
			if dirInfo.ModTime().Before(info.CacheTime) {
				return &info, nil
			}
		}

		// Read directory contents
		files, err := dir.Readdir(-1)
		if err != nil {
			return nil, err
		}
		// Recursively process files and directories
		fileInfos := map[string]*FileInfo{}
		dirInfos := map[string]*FileInfo{}

		var totalSize int64
		var numDirs, numFiles int
		for _, item := range files {
			newInfo := &FileInfo{
				Name:    item.Name(),
				Size:    item.Size(),
				ModTime: item.ModTime(),
			}
			if item.IsDir() {
				newInfo.Type = "directory"
				dirInfos[item.Name()] = newInfo
				numDirs++
			} else {
				_ = newInfo.detectType(opts.Path, true, false, false)
				fileInfos[item.Name()] = newInfo
				numFiles++
			}
			totalSize += newInfo.Size
		}
		file.NumDirs = numDirs
		file.NumFiles = numFiles
		file.Size = totalSize
	}
	if info.Mode()&os.ModeSymlink != 0 {
		file.IsSymlink = true
		targetInfo, err := os.Stat(opts.Path)
		if err == nil {
			file.Size = targetInfo.Size()
		}
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
		return "", false, fmt.Errorf("could not get real path: %v, %s", combined, err)
	}
	// Resolve symlinks and get the real path
	return resolveSymlinks(absolutePath)
}

func DeleteFiles(absPath string, opts FileOptions) error {
	err := os.RemoveAll(absPath)
	if err != nil {
		return err
	}
	opts.Path = filepath.Dir(absPath)
	err = RefreshFileInfo(opts)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteDirectory(opts FileOptions) error {
	// Ensure the parent directories exist
	err := os.MkdirAll(opts.Path, 0775)
	if err != nil {
		return err
	}
	opts.Path = filepath.Dir(opts.Path)
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
			return "", false, fmt.Errorf("could not stat path: %v, %s", path, err)
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the symlink target
			target, err := os.Readlink(path)
			if err != nil {
				return "", false, err
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
	if i.Type != "directory" {
		content, err := os.ReadFile(path)
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

// readListing reads the contents of a directory and fills the listing.
func (i *FileInfo) readListing(path string, checker users.Checker, readHeader bool) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	listing := &FileInfo{
		Files:    map[string]*FileInfo{},
		Dirs:     map[string]*FileInfo{},
		NumDirs:  0,
		NumFiles: 0,
	}

	for _, f := range files {
		name := f.Name()
		fPath := filepath.Join(path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			info, err := os.Stat(fPath)
			if err == nil {
				f = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Size:    f.Size(),
			ModTime: f.ModTime(),
			Mode:    f.Mode(),
		}
		if f.IsDir() {

		}
		if isSymlink {
			file.IsSymlink = true
		}

		if file.Type == "directory" {
			file.Type = "directory"
			listing.Dirs[name] = file
			listing.NumDirs++
		} else {
			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(path, true, false, readHeader)
				if err != nil {
					fmt.Errorf("could not detect type: %s", err)
				}
			}
			listing.Dirs[name] = file
			listing.NumFiles++
		}
	}
	return nil
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
