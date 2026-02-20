package iteminfo

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

type ItemInfo struct {
	Name       string    `json:"name"`               // name of the file
	Size       int64     `json:"size"`               // length in bytes for regular files
	ModTime    time.Time `json:"modified"`           // modification time
	Type       string    `json:"type"`               // type of the file, either "directory" or a file mimetype
	Hidden     bool      `json:"hidden"`             // whether the file is hidden
	HasPreview bool      `json:"hasPreview"`         // whether the file has a thumbnail preview
	IsShared   bool      `json:"isShared,omitempty"` // whether the file or folder is shared
}

// ExtendedItemInfo extends ItemInfo with optional metadata that's only populated on-demand
// This avoids adding memory overhead to indexed items
type ExtendedItemInfo struct {
	ItemInfo
	Metadata *MediaMetadata `json:"metadata,omitempty"` // optional media metadata (audio/video only)
}

// FileInfo describes a file.
// reduced item is non-recursive reduced "Items", used to pass flat items array
type FileInfo struct {
	ItemInfo
	Files   []ExtendedItemInfo `json:"files,omitempty"`   // files in the directory with optional metadata
	Folders []ItemInfo         `json:"folders,omitempty"` // folders in the directory
	Path    string             `json:"path,omitempty"`    // path scoped to the associated index
}

// MediaMetadata contains metadata extracted from audio and video files
type MediaMetadata struct {
	Title    string `json:"title,omitempty"`    // track/video title
	Artist   string `json:"artist,omitempty"`   // track artist
	Album    string `json:"album,omitempty"`    // album name
	Year     int    `json:"year,omitempty"`     // release year
	Genre    string `json:"genre,omitempty"`    // music/video genre
	Track    int    `json:"track,omitempty"`    // track number
	Duration int    `json:"duration,omitempty"` // duration in seconds
	AlbumArt []byte `json:"albumArt,omitempty"` // album art image data (automatically base64-encoded in JSON)
}

// for efficiency, a response will be a pointer to the data
// extra calculated fields can be added here
type ExtendedFileInfo struct {
	FileInfo
	Content      string                `json:"content,omitempty"`      // text content of a file, if requested
	Subtitles    []utils.SubtitleTrack `json:"subtitles,omitempty"`    // subtitles for video files
	Metadata     *MediaMetadata        `json:"metadata,omitempty"`     // media metadata for audio/video files (includes duration)
	Checksums    map[string]string     `json:"checksums,omitempty"`    // checksums for the file
	Token        string                `json:"token,omitempty"`        // token for the file -- used for sharing
	OnlyOfficeId string                `json:"onlyOfficeId,omitempty"` // id for onlyoffice files
	Source       string                `json:"source,omitempty"`       // associated index source for the file
	Hash         string                `json:"hash,omitempty"`         // hash for the file -- used for sharing
	HasMetadata  bool                  `json:"hasMetadata"`            // whether the file or folder has metadata
	RealPath     string                `json:"-"`
}
