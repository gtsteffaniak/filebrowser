package share

import "sync"

type CommonShare struct {
	DownloadsLimit           int      `json:"downloadsLimit,omitempty"`
	ShareTheme               string   `json:"shareTheme,omitempty"`
	DisableAnonymous         bool     `json:"disableAnonymous,omitempty"`
	MaxBandwidth             int      `json:"maxBandwidth,omitempty"`
	DisableThumbnails        bool     `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration      bool     `json:"keepAfterExpiration,omitempty"`
	AllowedUsernames         []string `json:"allowedUsernames,omitempty"`
	ThemeColor               string   `json:"themeColor,omitempty"`
	Banner                   string   `json:"banner,omitempty"`
	Title                    string   `json:"title,omitempty"`
	Description              string   `json:"description,omitempty"`
	Favicon                  string   `json:"favicon,omitempty"`
	QuickDownload            bool     `json:"quickDownload,omitempty"`
	HideNavButtons           bool     `json:"hideNavButtons,omitempty"`
	DisableSidebar           bool     `json:"disableSidebar"`
	Source                   string   `json:"source,omitempty"` // backend source is path to maintain between name changes
	Path                     string   `json:"path,omitempty"`
	DownloadURL              string   `json:"downloadURL,omitempty"`
	DisableShareCard         bool     `json:"disableShareCard,omitempty"`
	EnforceDarkLightMode     string   `json:"enforceDarkLightMode,omitempty"` // "dark" or "light"
	ViewMode                 string   `json:"viewMode,omitempty"`             // default view mode for anonymous users: "list", "compact", "normal", "gallery"
	EnableOnlyOffice         bool     `json:"enableOnlyOffice,omitempty"`
	ShareType                string   `json:"shareType"` // type of share: normal, upload, max
	PerUserDownloadLimit     bool     `json:"perUserDownloadLimit,omitempty"`
	ExtractEmbeddedSubtitles bool     `json:"extractEmbeddedSubtitles,omitempty"` // can be io intensive for large files and take 10-30 seconds.
	AllowDelete              bool     `json:"allowDelete,omitempty"`
	AllowCreate              bool     `json:"allowCreate,omitempty"`       // allow creating files
	AllowModify              bool     `json:"allowModify,omitempty"`       // allow modifying files
	DisableFileViewer        bool     `json:"disableFileViewer,omitempty"` // don't allow viewing files
	DisableDownload          bool     `json:"disableDownload,omitempty"`   // don't allow downloading files
	AllowReplacements        bool     `json:"allowReplacements,omitempty"` // allow replacements of files
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
	UserDownloads map[string]int `json:"-"` // Track downloads per username (not persisted)

}
