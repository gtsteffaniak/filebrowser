package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ShareDefaults holds admin-configured default share options.
type ShareDefaults struct {
	ShareTheme               string              `json:"shareTheme,omitempty"`
	DisableAnonymous         bool                `json:"disableAnonymous,omitempty"`
	DisableThumbnails        bool                `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration      bool                `json:"keepAfterExpiration,omitempty"`
	ThemeColor               string              `json:"themeColor,omitempty"`
	Title                    string              `json:"title,omitempty"`
	Description              string              `json:"description,omitempty"`
	Favicon                  string              `json:"favicon,omitempty"`
	QuickDownload            bool                `json:"quickDownload,omitempty"`
	HideNavButtons           bool                `json:"hideNavButtons,omitempty"`
	DisableSidebar           bool                `json:"disableSidebar,omitempty"`
	DisableShareCard         bool                `json:"disableShareCard,omitempty"`
	EnforceDarkLightMode     string              `json:"enforceDarkLightMode,omitempty"`
	ViewMode                 string              `json:"viewMode,omitempty"`
	EnableOnlyOffice         bool                `json:"enableOnlyOffice,omitempty"`
	ShareType                string              `json:"shareType,omitempty"`
	AllowDelete              bool                `json:"allowDelete,omitempty"`
	AllowCreate              bool                `json:"allowCreate,omitempty"`
	AllowModify              bool                `json:"allowModify,omitempty"`
	DisableFileViewer        bool                `json:"disableFileViewer,omitempty"`
	DisableDownload          bool                `json:"disableDownload,omitempty"`
	AllowReplacements        bool                `json:"allowReplacements,omitempty"`
	SidebarLinks             []users.SidebarLink `json:"sidebarLinks,omitempty"`
	ShowHidden               bool                `json:"showHidden,omitempty"`
	DisableLoginOption       bool                `json:"disableLoginOption,omitempty"`
	MaxBandwidth             int                 `json:"maxBandwidth,omitempty"`
	AllowedUsernames         []string            `json:"allowedUsernames,omitempty"`
	PerUserDownloadLimit     bool                `json:"perUserDownloadLimit,omitempty"`
	ExtractEmbeddedSubtitles bool                `json:"extractEmbeddedSubtitles,omitempty"`
	DownloadsLimit           int                 `json:"downloadsLimit,omitempty"`
	QuotaLimitBytes          int64               `json:"quotaLimitBytes,omitempty"`
	HideFileExt              string              `json:"hideFileExt,omitempty"`
	Banner                   string              `json:"banner,omitempty"`
}

// ShareDefaultsEnforcement marks which share default fields are enforced for non-admins.
type ShareDefaultsEnforcement struct {
	ShareTheme               bool `json:"shareTheme,omitempty"`
	DisableAnonymous         bool `json:"disableAnonymous,omitempty"`
	DisableThumbnails        bool `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration      bool `json:"keepAfterExpiration,omitempty"`
	ThemeColor               bool `json:"themeColor,omitempty"`
	Title                    bool `json:"title,omitempty"`
	Description              bool `json:"description,omitempty"`
	Favicon                  bool `json:"favicon,omitempty"`
	QuickDownload            bool `json:"quickDownload,omitempty"`
	HideNavButtons           bool `json:"hideNavButtons,omitempty"`
	DisableSidebar           bool `json:"disableSidebar,omitempty"`
	DisableShareCard         bool `json:"disableShareCard,omitempty"`
	EnforceDarkLightMode     bool `json:"enforceDarkLightMode,omitempty"`
	ViewMode                 bool `json:"viewMode,omitempty"`
	EnableOnlyOffice         bool `json:"enableOnlyOffice,omitempty"`
	ShareType                bool `json:"shareType,omitempty"`
	AllowDelete              bool `json:"allowDelete,omitempty"`
	AllowCreate              bool `json:"allowCreate,omitempty"`
	AllowModify              bool `json:"allowModify,omitempty"`
	DisableFileViewer        bool `json:"disableFileViewer,omitempty"`
	DisableDownload          bool `json:"disableDownload,omitempty"`
	AllowReplacements        bool `json:"allowReplacements,omitempty"`
	SidebarLinks             bool `json:"sidebarLinks,omitempty"`
	ShowHidden               bool `json:"showHidden,omitempty"`
	DisableLoginOption       bool `json:"disableLoginOption,omitempty"`
	MaxBandwidth             bool `json:"maxBandwidth,omitempty"`
	AllowedUsernames         bool `json:"allowedUsernames,omitempty"`
	PerUserDownloadLimit     bool `json:"perUserDownloadLimit,omitempty"`
	ExtractEmbeddedSubtitles bool `json:"extractEmbeddedSubtitles,omitempty"`
	DownloadsLimit           bool `json:"downloadsLimit,omitempty"`
	QuotaLimitBytes          bool `json:"quotaLimitBytes,omitempty"`
	HideFileExt              bool `json:"hideFileExt,omitempty"`
	Banner                   bool `json:"banner,omitempty"`
}
