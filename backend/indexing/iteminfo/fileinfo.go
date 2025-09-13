package iteminfo

import (
	"path/filepath"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
)

type ItemInfo struct {
	Name       string    `json:"name"`       // name of the file
	Size       int64     `json:"size"`       // length in bytes for regular files
	ModTime    time.Time `json:"modified"`   // modification time
	Type       string    `json:"type"`       // type of the file, either "directory" or a file mimetype
	Hidden     bool      `json:"hidden"`     // whether the file is hidden
	HasPreview bool      `json:"hasPreview"` // whether the file has a thumbnail preview
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	ItemInfo
	Files   []ItemInfo `json:"files"`   // files in the directory
	Folders []ItemInfo `json:"folders"` // folders in the directory
	Path    string     `json:"path"`    // path scoped to the associated index
}

// AudioMetadata contains metadata extracted from audio files
type AudioMetadata struct {
	Title    string `json:"title,omitempty"`    // track title
	Artist   string `json:"artist,omitempty"`   // track artist
	Album    string `json:"album,omitempty"`    // album name
	Year     int    `json:"year,omitempty"`     // release year
	Genre    string `json:"genre,omitempty"`    // music genre
	Track    int    `json:"track,omitempty"`    // track number
	Duration int    `json:"duration,omitempty"` // duration in seconds
	AlbumArt string `json:"albumArt,omitempty"` // base64 encoded album art
}

// for efficiency, a response will be a pointer to the data
// extra calculated fields can be added here
type ExtendedFileInfo struct {
	FileInfo
	Content      string                 `json:"content,omitempty"`      // text content of a file, if requested
	Subtitles    []ffmpeg.SubtitleTrack `json:"subtitles,omitempty"`    // subtitles for video files
	AudioMeta    *AudioMetadata         `json:"audioMeta,omitempty"`    // audio metadata for audio files
	Checksums    map[string]string      `json:"checksums,omitempty"`    // checksums for the file
	Token        string                 `json:"token,omitempty"`        // token for the file -- used for sharing
	OnlyOfficeId string                 `json:"onlyOfficeId,omitempty"` // id for onlyoffice files
	Source       string                 `json:"source,omitempty"`       // associated index source for the file
	Hash         string                 `json:"hash,omitempty"`         // hash for the file -- used for sharing
	RealPath     string                 `json:"-"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Access     *access.Storage
	Username   string // username for access control
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
