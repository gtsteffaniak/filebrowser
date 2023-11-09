package files

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/rules"
)

var (
	bytesInMegabyte int64 = 1000000
)

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs        afero.Fs          `json:"-"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Size      int64             `json:"size"`
	Extension string            `json:"extension"`
	ModTime   time.Time         `json:"modified"`
	Mode      os.FileMode       `json:"mode"`
	IsDir     bool              `json:"isDir"`
	IsSymlink bool              `json:"isSymlink"`
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

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file. If it's a video file, it will also detect any subtitles.
func NewFileInfo(opts FileOptions) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}
	index := GetIndex(rootPath)
	trimmed := strings.TrimPrefix(opts.Path, "/")
	adjustedPath := makeIndexPath(trimmed, index.Root)
	if dir, exists := index.Directories[adjustedPath]; exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
			index.Directories[adjustedPath] = dir
		}
		info, metadataExists := dir.Metadata[adjustedPath]
		if metadataExists && info.Path == trimmed {
			return &info, nil // Return the pointer directly
		}
	}

	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.Checker, opts.ReadHeader); err != nil {
				return nil, err
			}
		} else {
			err = file.detectType(opts.Modify, opts.Content, true)
		}
	}

	if file.IsDir {
		if _, exists := index.Directories[adjustedPath]; exists {
			if file.Path == trimmed {
				index.Directories[adjustedPath].Metadata[adjustedPath] = *file
				newInfo := index.Directories[adjustedPath].Metadata[adjustedPath]
				return &newInfo, nil
			}
		}
	}

	return file, err
}

func stat(opts FileOptions) (*FileInfo, error) {
	var file *FileInfo
	if lstaterFs, ok := opts.Fs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(opts.Path)
		if err == nil {
			file = &FileInfo{
				Fs:        opts.Fs,
				Path:      opts.Path,
				Name:      info.Name(),
				ModTime:   info.ModTime(),
				Mode:      info.Mode(),
				IsDir:     info.IsDir(),
				IsSymlink: IsSymlink(info.Mode()),
				Size:      info.Size(),
				Extension: filepath.Ext(info.Name()),
				Token:     opts.Token,
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

	var h hash.Hash

	//nolint:gosec
	switch algo {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
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

// detectType detects the file type.
func (i *FileInfo) detectType(modify, saveContent, readHeader bool) error {
	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		return nil
	}
	mimetype := mime.TypeByExtension(i.Extension)
	var buffer []byte
	if readHeader {
		buffer = i.readFirstBytes()
		if mimetype == "" {
			http.DetectContentType(buffer)
		}
	}
	switch {
	case IsMatchingType(i.Extension, "video"):
		i.Type = "video"
		i.detectSubtitles()
	case IsMatchingType(i.Extension, "audio"):
		i.Type = "audio"
	case IsMatchingType(i.Extension, "image"):
		i.Type = "image"
	case IsMatchingType(i.Extension, "pdf"):
		i.Type = "pdf"
	case (IsMatchingType(i.Extension, "text") || !isBinary(buffer)) && i.Size <= 10*bytesInMegabyte: // 10 MB
		i.Type = "text"

		if !modify {
			i.Type = "textImmutable"
		}

		if saveContent {
			afs := &afero.Afero{Fs: i.Fs}
			content, err := afs.ReadFile(i.Path)
			if err != nil {
				return err
			}

			i.Content = string(content)
		}
	default:
		i.Type = "blob"
	}
	return nil
}

// readFirstBytes reads the first bytes of the file.
func (i *FileInfo) readFirstBytes() []byte {
	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		log.Print(err)
		i.Type = "blob"
		return nil
	}
	defer reader.Close()

	buffer := make([]byte, 512) //nolint:gomnd
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		log.Print(err)
		i.Type = "blob"
		return nil
	}

	return buffer[:n]
}

// detectSubtitles detects subtitles for video files.
func (i *FileInfo) detectSubtitles() {
	if i.Type != "video" {
		return
	}

	i.Subtitles = []string{}
	ext := filepath.Ext(i.Path)

	parentDir := strings.TrimRight(i.Path, i.Name)
	dir, err := afero.ReadDir(i.Fs, parentDir)
	if err != nil {
		return
	}

	base := strings.TrimSuffix(i.Name, ext)
	subtitleExts := []string{".vtt", ".txt", ".srt", ".lrc"}

	for _, f := range dir {
		if f.IsDir() || !strings.HasPrefix(f.Name(), base) {
			continue
		}

		for _, subtitleExt := range subtitleExts {
			if strings.HasSuffix(f.Name(), subtitleExt) {
				i.Subtitles = append(i.Subtitles, path.Join(parentDir, f.Name()))
				break
			}
		}
	}
}

// readListing reads the contents of a directory and fills the listing.
func (i *FileInfo) readListing(checker rules.Checker, readHeader bool) error {
	afs := &afero.Afero{Fs: i.Fs}
	dir, err := afs.ReadDir(i.Path)
	if err != nil {
		return err
	}

	listing := &Listing{
		Items:    []*FileInfo{},
		NumDirs:  0,
		NumFiles: 0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := path.Join(i.Path, name)

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
			Fs:        i.Fs,
			Name:      name,
			Size:      f.Size(),
			ModTime:   f.ModTime(),
			Mode:      f.Mode(),
			IsDir:     f.IsDir(),
			IsSymlink: isSymlink,
			Extension: filepath.Ext(name),
			Path:      fPath,
		}

		if file.IsDir {
			listing.NumDirs++
		} else {
			listing.NumFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(true, false, readHeader)
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
