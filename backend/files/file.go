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
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/filebrowser/backend/cache"
	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

type ItemInfo struct {
	Name    string    `json:"name"`     // name of the file
	Size    int64     `json:"size"`     // length in bytes for regular files
	ModTime time.Time `json:"modified"` // modification time
	Type    string    `json:"type"`     // type of the file, either "directory" or a file mimetype
	Hidden  bool      `json:"hidden"`   // whether the file is hidden
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	ItemInfo
	Files   []ItemInfo `json:"files"`   // files in the directory
	Folders []ItemInfo `json:"folders"` // folders in the directory
	Path    string     `json:"path"`    // path scoped to the associated index
}

// for efficiency, a response will be a pointer to the data
// extra calculated fields can be added here
type ExtendedFileInfo struct {
	FileInfo
	Content      string            `json:"content,omitempty"`      // text content of a file, if requested
	Subtitles    []string          `json:"subtitles,omitempty"`    // subtitles for video files
	Checksums    map[string]string `json:"checksums,omitempty"`    // checksums for the file
	Token        string            `json:"token,omitempty"`        // token for the file -- used for sharing
	OnlyOfficeId string            `json:"onlyOfficeId,omitempty"` // id for onlyoffice files
	Source       string            `json:"source"`                 // associated index source for the file
	RealPath     string            `json:"-"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Path       string // realpath
	Source     string
	IsDir      bool
	Modify     bool
	Expand     bool
	ReadHeader bool
	Content    bool
}

func (f FileOptions) Components() (string, string) {
	return filepath.Dir(f.Path), filepath.Base(f.Path)
}

func FileInfoFaster(opts FileOptions) (ExtendedFileInfo, error) {
	response := ExtendedFileInfo{}
	if opts.Source == "" {
		opts.Source = settings.Config.Server.DefaultSource.Name
	}
	index := GetIndex(opts.Source)
	if index == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, isDir, err := index.GetRealPath(opts.Path)
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
		return response, fmt.Errorf("could not get metadata for path: %v", opts.Path)
	}
	if opts.Content && strings.HasPrefix(info.Type, "text") {
		// Check file size
		if info.Size > 50*1024*1024 { // 50 megabytes in bytes
			logger.Debug(fmt.Sprintf("Reading large text file contents: "+info.Path, info.Name))
		}

		content, err := getContent(realPath)
		if err != nil {
			return response, err
		}
		response.Content = content
	}
	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source
	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && isOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}
	if strings.HasPrefix(info.Type, "video") {
		response.detectSubtitles(realPath)
	}
	return response, nil
}

func generateOfficeId(realPath string) string {
	key, ok := cache.OnlyOffice.Get(realPath).(string)
	if !ok {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		documentKey := utils.HashSHA256(realPath + timestamp)
		cache.OnlyOffice.Set(realPath, documentKey)
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
	index := GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	refreshConfig := FileOptions{Path: index.MakeIndexPath(absDirPath), IsDir: true}
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
	idxSrc := GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	// refresh info for source and dest
	err = idxSrc.RefreshFileInfo(FileOptions{
		Path:  refreshSourceDir,
		IsDir: true,
	})
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	if refreshSourceDir == refreshDestDir {
		return nil
	}
	refreshConfig := FileOptions{Path: refreshDestDir, IsDir: true}
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
	idxSrc := GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	index := GetIndex(sourceIndex)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshConfig := FileOptions{Path: refreshSourceDir, IsDir: true}
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

func WriteDirectory(opts FileOptions) error {
	idx := GetIndex(opts.Source)
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

func WriteFile(opts FileOptions, in io.Reader) error {
	idx := GetIndex(opts.Source)
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

func resolveSymlinks(path string) (string, bool, error) {
	const maxSymlinks = 25
	for i := 0; i < maxSymlinks; i++ {
		info, err := os.Lstat(path)
		if err != nil {
			return path, false, fmt.Errorf("could not stat path: %s, %v", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return path, false, fmt.Errorf("could not read symlink: %s, %v", path, err)
			}
			path = filepath.Clean(filepath.Join(filepath.Dir(path), target))
			if !filepath.IsAbs(path) {
				path, err = filepath.Abs(path)
				if err != nil {
					return path, false, fmt.Errorf("could not resolve absolute path: %s, %v", path, err)
				}
			}
			continue
		}
		return path, info.IsDir(), nil
	}
	return path, false, fmt.Errorf("too many symlink resolutions for path: %s", path)
}

// addContent reads and sets content based on the file type.
func getContent(realPath string) (string, error) {
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
func (i *ItemInfo) DetectType(realPath string, saveContent bool) {
	name := i.Name
	ext := filepath.Ext(name)

	// Attempt MIME detection by file extension
	if ext == ".md" {
		i.Type = "text/markdown"
		return
	}
	i.Type = strings.Split(mime.TypeByExtension(ext), ";")[0]
	if i.Type == "" {
		i.Type = extendedMimeTypeCheck(ext)
	}
	if i.Type == "blob" {
		// Read only the first 512 bytes for efficient MIME detection
		file, err := os.Open(realPath)
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
func (i *ExtendedFileInfo) detectSubtitles(path string) {
	if !strings.HasPrefix(i.Type, "video") {
		logger.Debug("subtitles are not supported for this file : " + path)
		return
	}

	idx := GetIndex(i.Source)
	if idx == nil {
		return
	}

	parentInfo, exists := idx.GetReducedMetadata(filepath.Dir(i.Path), true)
	if !exists {
		return
	}
	base := strings.Split(i.Name, ".")[0]
	for _, f := range parentInfo.Files {
		baseName := strings.Split(f.Name, ".")[0]
		if baseName != base {
			continue
		}

		for _, subtitleExt := range []string{".vtt", ".srt", ".lrc", ".sbv", ".ass", ".ssa", ".sub", ".smi"} {
			if strings.HasSuffix(f.Name, subtitleExt) {
				fullPathBase := strings.Split(i.Path, ".")[0]
				i.Subtitles = append(i.Subtitles, fullPathBase+subtitleExt)
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
		nameWithoutExt := strings.Split(info.Folders[i].Name, ".")[0]
		nameWithoutExt2 := strings.Split(info.Folders[j].Name, ".")[0]
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(nameWithoutExt)
		numJ, errJ := strconv.Atoi(nameWithoutExt2)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Folders[i].Name) < strings.ToLower(info.Folders[j].Name)
	})
	sort.Slice(info.Files, func(i, j int) bool {
		nameWithoutExt := strings.Split(info.Files[i].Name, ".")[0]
		nameWithoutExt2 := strings.Split(info.Files[j].Name, ".")[0]
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(nameWithoutExt)
		numJ, errJ := strconv.Atoi(nameWithoutExt2)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Files[i].Name) < strings.ToLower(info.Files[j].Name)
	})
}
