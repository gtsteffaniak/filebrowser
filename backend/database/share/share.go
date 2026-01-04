package share

import (
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

type CommonShare struct {
	DownloadsLimit           int                 `json:"downloadsLimit,omitempty"`
	ShareTheme               string              `json:"shareTheme,omitempty"`
	DisableAnonymous         bool                `json:"disableAnonymous,omitempty"`
	MaxBandwidth             int                 `json:"maxBandwidth,omitempty"`
	DisableThumbnails        bool                `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration      bool                `json:"keepAfterExpiration,omitempty"`
	AllowedUsernames         []string            `json:"allowedUsernames,omitempty"`
	ThemeColor               string              `json:"themeColor,omitempty"`
	Banner                   string              `json:"banner,omitempty"`
	Title                    string              `json:"title,omitempty"`
	Description              string              `json:"description,omitempty"`
	Favicon                  string              `json:"favicon,omitempty"`
	QuickDownload            bool                `json:"quickDownload,omitempty"`
	HideNavButtons           bool                `json:"hideNavButtons,omitempty"`
	DisableSidebar           bool                `json:"disableSidebar"`
	Source                   string              `json:"source,omitempty"` // backend source is path to maintain between name changes
	Path                     string              `json:"path,omitempty"`
	DownloadURL              string              `json:"downloadURL,omitempty"`
	ShareURL                 string              `json:"shareURL,omitempty"`
	DisableShareCard         bool                `json:"disableShareCard,omitempty"`
	EnforceDarkLightMode     string              `json:"enforceDarkLightMode,omitempty"` // "dark" or "light"
	ViewMode                 string              `json:"viewMode,omitempty"`             // default view mode for anonymous users: "list", "compact", "normal", "gallery"
	EnableOnlyOffice         bool                `json:"enableOnlyOffice,omitempty"`
	ShareType                string              `json:"shareType"` // type of share: normal, upload, max
	PerUserDownloadLimit     bool                `json:"perUserDownloadLimit,omitempty"`
	ExtractEmbeddedSubtitles bool                `json:"extractEmbeddedSubtitles,omitempty"` // can be io intensive for large files and take 10-30 seconds.
	AllowDelete              bool                `json:"allowDelete,omitempty"`
	AllowCreate              bool                `json:"allowCreate,omitempty"`       // allow creating files
	AllowModify              bool                `json:"allowModify,omitempty"`       // allow modifying files
	DisableFileViewer        bool                `json:"disableFileViewer,omitempty"` // don't allow viewing files
	DisableDownload          bool                `json:"disableDownload,omitempty"`   // don't allow downloading files
	AllowReplacements        bool                `json:"allowReplacements,omitempty"` // allow replacements of files
	SidebarLinks             []users.SidebarLink `json:"sidebarLinks"`                // customizable sidebar links
	HasPassword              bool                `json:"hasPassword,omitempty"`
	// Preview settings - if not set, will use defaults
	PreviewVideo              bool `json:"previewVideo,omitempty"`              // show thumbnail preview image for video files
	PreviewImage              bool `json:"previewImage,omitempty"`              // show thumbnail preview image for image files
	PreviewOffice             bool `json:"previewOffice,omitempty"`             // show thumbnail preview image for office files
	PreviewFolder             bool `json:"previewFolder,omitempty"`             // show thumbnail preview image for folder files
	PreviewPopup              bool `json:"previewPopup,omitempty"`              // show larger popup preview when hovering over thumbnail
	PreviewHighQuality        bool `json:"previewHighQuality,omitempty"`        // generate high quality thumbnail preview images
	PreviewMotionVideo        bool `json:"previewMotionVideo,omitempty"`        // show multiple frames for videos in thumbnail preview when hovering
	PreviewDisableHideSidebar bool `json:"previewDisableHideSidebar,omitempty"` // disable the hide sidebar preview for previews and editors
	PreviewAutoplayMedia      bool `json:"previewAutoplayMedia,omitempty"`      // autoplay media files in preview
	PreviewDefaultMediaPlayer bool `json:"previewDefaultMediaPlayer,omitempty"` // disable html5 media player and use the default media player
	ShowHidden                bool `json:"showHidden,omitempty"`                // show hidden files in share (true = show, false = hide)
}
type CreateBody struct {
	CommonShare
	Hash     string `json:"hash,omitempty"`
	Password string `json:"password"`
	Expires  string `json:"expires"`
	Unit     string `json:"unit"`
}

// Link is the information needed to build a shareable link.
type Link struct {
	CommonShare
	Downloads    int    `json:"downloads"`
	Hash         string `json:"hash" storm:"id,index"`
	UserID       uint   `json:"userID"`
	Expire       int64  `json:"expire"`
	PasswordHash string `json:"password_hash,omitempty"`
	// Token is a random value that will only be set when PasswordHash is set. It is
	// URL-Safe and is used to download links in password-protected shares via a
	// query arg.
	Token string `json:"token,omitempty"`

	Mu            sync.Mutex     `json:"-"`
	UserDownloads map[string]int `json:"userDownloads,omitempty"` // Track downloads per username
	Version       int            `json:"version,omitempty"`
}
