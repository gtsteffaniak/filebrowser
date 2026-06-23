package share

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// FrontendShareInfo is share presentation and behavior for visitors (stored in share_settings JSON).
type FrontendShareInfo struct {
	ShareTheme           string              `json:"shareTheme,omitempty"`
	DisableAnonymous     bool                `json:"disableAnonymous,omitempty"`
	DisableThumbnails    bool                `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration  bool                `json:"keepAfterExpiration,omitempty"`
	ThemeColor           string              `json:"themeColor,omitempty"`
	Title                string              `json:"title,omitempty"`
	Description          string              `json:"description,omitempty"`
	Favicon              string              `json:"favicon,omitempty"`
	QuickDownload        bool                `json:"quickDownload,omitempty"`
	HideNavButtons       bool                `json:"hideNavButtons,omitempty"`
	DisableSidebar       bool                `json:"disableSidebar"`
	DownloadURL          string              `json:"downloadURL,omitempty"`
	ShareURL             string              `json:"shareURL,omitempty"`
	FaviconUrl           string              `json:"faviconUrl,omitempty"`
	BannerUrl            string              `json:"bannerUrl,omitempty"`
	DisableShareCard     bool                `json:"disableShareCard,omitempty"`
	EnforceDarkLightMode string              `json:"enforceDarkLightMode,omitempty"` // "dark" or "light"
	ViewMode             string              `json:"viewMode,omitempty"`             // default view mode for anonymous users
	EnableOnlyOffice     bool                `json:"enableOnlyOffice,omitempty"`
	ShareType            string              `json:"shareType"`
	AllowDelete          bool                `json:"allowDelete,omitempty"`
	AllowCreate          bool                `json:"allowCreate,omitempty"`
	AllowModify          bool                `json:"allowModify,omitempty"`
	DisableFileViewer    bool                `json:"disableFileViewer,omitempty"`
	DisableDownload      bool                `json:"disableDownload,omitempty"`
	AllowReplacements    bool                `json:"allowReplacements,omitempty"`
	SidebarLinks         []users.SidebarLink `json:"sidebarLinks"`
	HasPassword          bool                `json:"hasPassword,omitempty"`
	ShowHidden           bool                `json:"showHidden,omitempty"`
	DisableLoginOption   bool                `json:"disableLoginOption"`
	SourceURL            string              `json:"sourceURL,omitempty"`
	CanEditShare         bool                `json:"canEditShare,omitempty"`
}

// ShareLimits are bandwidth, access, and banner settings stored in share_settings JSON.
type ShareLimits struct {
	MaxBandwidth             int      `json:"maxBandwidth,omitempty"`
	AllowedUsernames         []string `json:"allowedUsernames,omitempty"`
	PerUserDownloadLimit     bool     `json:"perUserDownloadLimit,omitempty"`
	ExtractEmbeddedSubtitles bool     `json:"extractEmbeddedSubtitles,omitempty"`
	DownloadsLimit           int      `json:"downloadsLimit,omitempty"`
	HideFileExt              string   `json:"hideFileExt,omitempty"` // show hidden files based on extensions in shares
	Banner                   string   `json:"banner,omitempty"`
	SourceName               string   `json:"source,omitempty"` // source display name for API; backend path is Share.SourcePath
}

// ShareExpiryInput is POST body input used to compute ShareColumns.Expire (not persisted).
type ShareExpiryInput struct {
	Expires string `json:"expires,omitempty"`
	Unit    string `json:"unit,omitempty"`
}

// ShareSettings is persisted in the share_settings JSON column.
type ShareSettings struct {
	FrontendShareInfo
	ShareLimits
	PinnedItems PinnedItems `json:"pinnedItems,omitempty"`
}

// ShareColumns are SQL-backed identity and counter fields.
type ShareColumns struct {
	Hash      string `json:"hash,omitempty" storm:"id,index"`
	Path      string `json:"path,omitempty"`
	Expire    int64  `json:"expire"`
	Downloads int    `json:"downloads,omitempty"`
}

// ShareEditable is the client-editable subset used in POST/PATCH bodies.
type ShareEditable struct {
	FrontendShareInfo
	ShareLimits
	ShareExpiryInput
}

// ShareFrontend is the share shape returned by list/get/create/update API responses.
type ShareFrontend struct {
	ShareEditable
	ShareColumns
	Username   string `json:"username,omitempty"`
	PathExists bool   `json:"pathExists,omitempty"`
}

// SharePostBody is POST /api/share JSON. Plaintext password is hashed to Share.PasswordHash before persist.
// Password omitted (nil) on update means keep the existing hash; empty string clears it.
type SharePostBody struct {
	ShareEditable
	Password *string `json:"password,omitempty"`
	Hash     string  `json:"hash,omitempty"`
	Path     string  `json:"path,omitempty"`
}

// ApplyPostBodyUpdate copies client-editable fields onto link.
// Caller must preserve path, sourcePath, pinnedItems, version, download counters, and secrets.
func ApplyPostBodyUpdate(link *Share, req *SharePostBody, expire int64) {
	link.FrontendShareInfo = req.FrontendShareInfo
	link.ShareLimits = req.ShareLimits
	link.Expire = expire
}

// Share is the persisted share: settings JSON, SQL columns, and backend-only fields.
type Share struct {
	ShareSettings
	ShareColumns
	PasswordHash  string         `json:"password_hash,omitempty"`
	UserID        uint64         `json:"userID,omitempty"`
	Token         string         `json:"token,omitempty"`
	UserDownloads map[string]int `json:"userDownloads,omitempty"`
	Version       int            `json:"version,omitempty"`
	SourcePath    string         `json:"sourcePath,omitempty"`
}

// LegacyShare embeds Share for Bolt/Storm. LegacyRoutingSource is the historical Bolt/JSON "source" field
// (path or name); it becomes Share.SourcePath after migration.
type LegacyShare struct {
	Share
	PasswordHash        string `json:"password_hash,omitempty"`
	LegacyRoutingSource string `json:"source,omitempty"`
}

// ToShare builds the SQLite/runtime share from a Bolt/Storm legacy record.
// Legacy JSON "source" (backend filesystem path) → SourcePath; share "path" stays Path.
// Legacy userID (small uint from Bolt users) is kept as-is; owner username is not stored.
func (l *LegacyShare) ToShare() Share {
	s := l.Share
	if l.PasswordHash != "" {
		s.PasswordHash = l.PasswordHash
	}
	s.SourcePath = l.LegacyRoutingSource
	return s
}
