package files

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"mime"
	"net/http"
	"os"
	filepath "path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/spf13/afero"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/users"
)

var (
	bytesInMegabyte int64      = 1000000
	pathMutexes                = make(map[string]*sync.Mutex)
	pathMutexesMu   sync.Mutex // Mutex to protect the pathMutexes map
)

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs        afero.Fs          `json:"-"`
	Path      string            `json:"path,omitempty"`
	Name      string            `json:"name"`
	Size      int64             `json:"size"`
	Extension string            `json:"-"`
	ModTime   time.Time         `json:"modified"`
	CacheTime time.Time         `json:"-"`
	Mode      os.FileMode       `json:"-"`
	IsDir     bool              `json:"isDir,omitempty"`
	IsSymlink bool              `json:"isSymlink,omitempty"`
	Type      string            `json:"type"`
	Subtitles []string          `json:"subtitles,omitempty"`
	Content   string            `json:"content,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
	Token     string            `json:"token,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Fs         afero.Fs
	Path       string
	Modify     bool
	Expand     bool
	ReadHeader bool
	Token      string
	Checker    rules.Checker
	Content    bool
}

// Sorting constants
const (
	SortingByName     = "name"
	SortingBySize     = "size"
	SortingByModified = "modified"
)

// Listing is a collection of files.
type Listing struct {
	Items    []*FileInfo   `json:"items"`
	Path     string        `json:"path"`
	NumDirs  int           `json:"numDirs"`
	NumFiles int           `json:"numFiles"`
	Sorting  users.Sorting `json:"sorting"`
}

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file. If it's a video file, it will also detect any subtitles.
func NewFileInfo(opts FileOptions) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}
	file, err := stat(opts.Path, opts) // Pass opts.Path here
	if err != nil {
		return nil, err
	}
	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.Path, opts.Checker, opts.ReadHeader); err != nil { //nolint:govet
				return nil, err
			}
			return file, nil
		}
		err = file.detectType(opts.Path, opts.Modify, opts.Content, true)
		if err != nil {
			return nil, err
		}
	}
	return file, err
}

func FileInfoFaster(opts FileOptions) (*FileInfo, error) {
	// Lock access for the specific path
	pathMutex := getMutex(opts.Path)
	pathMutex.Lock()
	defer pathMutex.Unlock()
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}
	index := GetIndex(rootPath)
	trimmed := strings.TrimPrefix(opts.Path, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	adjustedPath := makeIndexPath(trimmed, index.Root)
	var info FileInfo
	info, exists := index.GetMetadataInfo(adjustedPath)
	if exists && !opts.Content {
		// Check if the cache time is less than 1 second
		if time.Since(info.CacheTime) > time.Second {
			go refreshFileInfo(opts)
		}
		// refresh cache after
		return &info, nil
	} else {
		// don't bother caching content
		if opts.Content {
			file, err := NewFileInfo(opts)
			return file, err
		}
		updated := refreshFileInfo(opts)
		if !updated {
			file, err := NewFileInfo(opts)
			return file, err
		}
		info, exists = index.GetMetadataInfo(adjustedPath)
		if !exists || info.Name == "" {
			return &FileInfo{}, errors.ErrEmptyKey
		}
		return &info, nil
	}
}

func refreshFileInfo(opts FileOptions) bool {
	if !opts.Checker.Check(opts.Path) {
		return false
	}
	index := GetIndex(rootPath)
	trimmed := strings.TrimPrefix(opts.Path, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	adjustedPath := makeIndexPath(trimmed, index.Root)
	file, err := stat(opts.Path, opts) // Pass opts.Path here
	if err != nil {
		return false
	}
	_ = file.detectType(adjustedPath, true, opts.Content, opts.ReadHeader)
	if file.IsDir {
		err := file.readListing(opts.Path, opts.Checker, opts.ReadHeader)
		if err != nil {
			return false
		}
		//_, exists := index.GetFileMetadata(adjustedPath)

		return index.UpdateFileMetadata(adjustedPath, *file)
	} else {
		//_, exists := index.GetFileMetadata(adjustedPath)
		return index.UpdateFileMetadata(adjustedPath, *file)
	}
}

func stat(path string, opts FileOptions) (*FileInfo, error) {
	var file *FileInfo
	if lstaterFs, ok := opts.Fs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(path)
		if err == nil {
			file = &FileInfo{
				Fs:        opts.Fs,
				Path:      opts.Path,
				Name:      info.Name(),
				ModTime:   info.ModTime(),
				Mode:      info.Mode(),
				Size:      info.Size(),
				Extension: filepath.Ext(info.Name()),
				Token:     opts.Token,
			}
			if info.IsDir() {
				file.IsDir = true
			}
			if info.Mode()&os.ModeSymlink != 0 {
				file.IsSymlink = true
			}
		}
	}
	if file == nil || file.IsSymlink {
		info, err := opts.Fs.Stat(opts.Path)
		if err != nil {
			return nil, err
		}

		if file != nil && file.IsSymlink {
			file.Size = info.Size()
			file.IsDir = info.IsDir()
			return file, nil
		}

		file = &FileInfo{
			Fs:        opts.Fs,
			Path:      opts.Path,
			Name:      info.Name(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			Size:      info.Size(),
			Extension: filepath.Ext(info.Name()),
			Token:     opts.Token,
		}
	}

	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {
	if i.IsDir {
		return errors.ErrIsDirectory
	}

	if i.Checksums == nil {
		i.Checksums = map[string]string{}
	}

	reader, err := i.Fs.Open(i.Path)
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
	if realPathFs, ok := i.Fs.(interface {
		RealPath(name string) (fPath string, err error)
	}); ok {
		realPath, err := realPathFs.RealPath(i.Path)
		if err == nil {
			return realPath
		}
	}

	return i.Path
}

// addContent reads and sets content based on the file type.
func (i *FileInfo) addContent(path string) error {
	if !i.IsDir {
		afs := &afero.Afero{Fs: i.Fs}
		content, err := afs.ReadFile(path)
		if err != nil {
			return err
		}
		c := string(string(content))
		if !utf8.ValidString(c) {
			return nil
		}
		i.Content = string(c)
	}
	return nil
}

// detectType detects the file type.
func (i *FileInfo) detectType(path string, modify, saveContent, readHeader bool) error {
	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		if saveContent {
			return i.addContent(path)
		}
		return nil
	}

	var buffer []byte
	if readHeader {
		buffer = i.readFirstBytes()
		mimetype := mime.TypeByExtension(i.Extension)
		if mimetype == "" {
			http.DetectContentType(buffer)
		}
	}

	ext := filepath.Ext(i.Name)
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
			parentDir := strings.TrimRight(path, i.Name)
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
func (i *FileInfo) readFirstBytes() []byte {
	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		i.Type = "blob"
		return nil
	}
	defer reader.Close()

	buffer := make([]byte, 512) //nolint:gomnd
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		i.Type = "blob"
		return nil
	}

	return buffer[:n]
}

// detectSubtitles detects subtitles for video files.
func (i *FileInfo) detectSubtitles(parentDir string) {
	if i.Type != "video" {
		return
	}
	i.Subtitles = []string{}
	ext := filepath.Ext(i.Name)
	dir, err := os.Open(parentDir)
	if err != nil {
		// Directory must have been deleted, remove it from the index
		return
	}
	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return
	}

	base := strings.TrimSuffix(i.Name, ext)
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
func (i *FileInfo) readListing(path string, checker rules.Checker, readHeader bool) error {
	afs := &afero.Afero{Fs: i.Fs}
	dir, err := afs.ReadDir(i.Path)
	if err != nil {
		return err
	}

	listing := &Listing{
		Items:    []*FileInfo{},
		Path:     i.Path,
		NumDirs:  0,
		NumFiles: 0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := filepath.Join(i.Path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			info, err := i.Fs.Stat(fPath)
			if err == nil {
				f = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Name:    name,
			Size:    f.Size(),
			ModTime: f.ModTime(),
			Mode:    f.Mode(),
		}
		if f.IsDir() {
			file.IsDir = true
		}
		if isSymlink {
			file.IsSymlink = true
		}

		if file.IsDir {
			listing.NumDirs++
		} else {
			listing.NumFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(path, true, false, readHeader)
				if err != nil {
					return err
				}
			}
		}

		listing.Items = append(listing.Items, file)
	}

	i.Listing = listing
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
