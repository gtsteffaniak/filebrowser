package users

import (
	"fmt"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
)

const ()

type LoginMethod string

const (
	LoginMethodPassword LoginMethod = "password"
	LoginMethodProxy    LoginMethod = "proxy"
	LoginMethodOidc     LoginMethod = "oidc"
	LoginMethodLdap     LoginMethod = "ldap"
)

type AuthToken struct {
	MinimalAuthToken
	UC          bool        `json:"uc,omitempty"` // whether the token is a user created token
	Key         string      `json:"key,omitempty"`
	Name        string      `json:"name,omitempty"`
	BelongsTo   uint        `json:"belongsTo,omitempty"`
	Permissions Permissions `json:"Permissions,omitempty"`
}

// MinimalAuthToken is used for tokens that only include JWT standard claims
type MinimalAuthToken struct {
	jwt.RegisteredClaims `swaggerignore:"true"`
}

type Permissions struct {
	Api      bool `json:"api"`      // allow api access
	Admin    bool `json:"admin"`    // allow admin access
	Modify   bool `json:"modify"`   // allow modifying files
	Share    bool `json:"share"`    // allow sharing files
	Realtime bool `json:"realtime"` // allow realtime updates
	Delete   bool `json:"delete"`   // allow deleting files
	Create   bool `json:"create"`   // allow creating or uploading files
	Download bool `json:"download"` // allow downloading files
}

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

type Preview struct {
	DisableHideSidebar bool `json:"disableHideSidebar"` // disable the hide sidebar preview for previews and editors
	Image              bool `json:"image"`              // show thumbnail preview image for image files
	Video              bool `json:"video"`              // show thumbnail preview image for video files
	MotionVideoPreview bool `json:"motionVideoPreview"` // show multiple frames for videos in thumbnail preview when hovering
	Office             bool `json:"office"`             // show thumbnail preview image for office files
	PopUp              bool `json:"popup"`              // show larger popup preview when hovering over thumbnail
	AutoplayMedia      bool `json:"autoplayMedia"`      // autoplay media files in preview
	DefaultMediaPlayer bool `json:"defaultMediaPlayer"` // disable html5 media player and use the default media player
	Folder             bool `json:"folder"`             // show thumbnail preview image for folder files
}

// User describes a user.
type User struct {
	NonAdminEditable
	DisableSettings bool                 `json:"disableSettings"`
	ID              uint                 `storm:"id,increment" json:"id"`
	Username        string               `storm:"unique" json:"username"`
	Scopes          []SourceScope        `json:"scopes"`
	Scope           string               `json:"scope,omitempty"`
	LockPassword    bool                 `json:"lockPassword"`
	Permissions     Permissions          `json:"permissions"`
	ApiKeys         map[string]AuthToken `json:"apiKeys,omitempty"`
	TOTPSecret      string               `json:"totpSecret,omitempty"`
	TOTPNonce       string               `json:"totpNonce,omitempty"`
	LoginMethod     LoginMethod          `json:"loginMethod"`
	OtpEnabled      bool                 `json:"otpEnabled"` // true if TOTP is enabled, false otherwise
	// legacy for migration purposes... og filebrowser has perm attribute
	Perm           Permissions `json:"perm,omitzero"`
	Version        int         `json:"version"`
	ShowFirstLogin bool        `json:"showFirstLogin"`
}

type SourceScope struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// json tags must match variable name with smaller case first letter
type NonAdminEditable struct {
	EditorQuickSave            bool          `json:"editorQuickSave"`         // show quick save button in editor
	HideSidebarFileActions     bool          `json:"hideSidebarFileActions"`  // hide the file actions in the sidebar
	DisableQuickToggles        bool          `json:"disableQuickToggles"`     // disable the quick toggles in the sidebar
	DisableSearchOptions       bool          `json:"disableSearchOptions"`    // disable the search options in the search bar
	DeleteWithoutConfirming    bool          `json:"deleteWithoutConfirming"` // delete files without confirmation
	Preview                    Preview       `json:"preview"`
	StickySidebar              bool          `json:"stickySidebar"` // keep sidebar open when navigating
	DarkMode                   bool          `json:"darkMode"`      // should dark mode be enabled
	Password                   string        `json:"password,omitempty"`
	Locale                     string        `json:"locale"`      // language to use: eg. de, en, or fr
	ViewMode                   string        `json:"viewMode"`    // view mode to use: eg. normal, list, grid, or compact
	SingleClick                bool          `json:"singleClick"` // open directory on single click, also enables middle click to open in new tab
	Sorting                    Sorting       `json:"sorting"`
	ShowHidden                 bool          `json:"showHidden"`                 // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	DateFormat                 bool          `json:"dateFormat"`                 // when false, the date is relative, when true, the date is an exact timestamp
	GallerySize                int           `json:"gallerySize"`                // 0-9 - the size of the gallery thumbnails
	ThemeColor                 string        `json:"themeColor"`                 // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	QuickDownload              bool          `json:"quickDownload"`              // show icon to download in one click
	DisableUpdateNotifications bool          `json:"disableUpdateNotifications"` // disable update notifications
	FileLoading                FileLoading   `json:"fileLoading"`                // upload and download settings
	DisableOfficePreviewExt    string        `json:"disableOfficePreviewExt"`    // deprecated
	DisableOnlyOfficeExt       string        `json:"disableOnlyOfficeExt"`       // deprecated
	DisablePreviewExt          string        `json:"disablePreviewExt"`          // space separated list of file extensions to disable preview for
	DisableViewingExt          string        `json:"disableViewingExt"`          // space separated list of file extensions to disable viewing for
	CustomTheme                string        `json:"customTheme"`                // Name of theme to use chosen from custom themes config.
	ShowSelectMultiple         bool          `json:"showSelectMultiple"`         // show select multiple files on desktop
	DebugOffice                bool          `json:"debugOffice"`                // debug onlyoffice editor
	OtpEnabled                 bool          `json:"otpEnabled"`                 // allow non-admin users to disable their own OTP
	SidebarLinks               []SidebarLink `json:"sidebarLinks"`               // customizable sidebar links
}

type FileLoading struct {
	MaxConcurrent     int  `json:"maxConcurrentUpload"`
	UploadChunkSize   int  `json:"uploadChunkSizeMb"`
	ClearAll          bool `json:"clearAll"`
	DownloadChunkSize int  `json:"downloadChunkSizeMb"`
}

// SidebarLink represents a customizable link in the sidebar.
type SidebarLink struct {
	Name       string `json:"name"`                 // Display name of the link
	Category   string `json:"category"`             // Category type: "source", "source-link", "share", "tool", "custom", etc.
	Target     string `json:"target"`               // Target path/URL for the link (relative for source/share)
	Icon       string `json:"icon"`                 // Material icon name
	SourceName string `json:"sourceName,omitempty"` // Source identifier for source-type links
}

func CleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.Replace(s, "..", "", -1)
	return s
}

// SourceNameResolver is a function type that resolves source names to source paths
// This avoids circular dependencies by allowing the settings package to provide the resolver
type SourceNameResolver func(sourceName string) (sourcePath string, err error)

// SourceInfo represents basic information about a source needed for user operations
type SourceInfo struct {
	Path             string
	Name             string
	DefaultUserScope string
}

// SourceConfigProvider provides access to source configuration
type SourceConfigProvider struct {
	GetSourceByPath  func(path string) (SourceInfo, bool)
	GetSourceByName  func(name string) (SourceInfo, bool)
	GetAllSources    func() []SourceInfo
	GetDefaultScopes func() []SourceScope
}

// Global variables set by the settings package
var (
	sourceNameResolver SourceNameResolver
	sourceConfig       *SourceConfigProvider
)

// SetSourceNameResolver sets the global source name resolver
// This should be called once during initialization by the settings package
func SetSourceNameResolver(resolver SourceNameResolver) {
	sourceNameResolver = resolver
}

// SetSourceConfig sets the global source configuration provider
// This should be called once during initialization by the settings package
func SetSourceConfig(config *SourceConfigProvider) {
	sourceConfig = config
}

// GetScopeForSourcePath returns the scope for a given source path (backend-style)
// This method works with backend-style scopes where Name is the source path
func (u *User) GetScopeForSourcePath(sourcePath string) (string, error) {
	for _, scope := range u.Scopes {
		if scope.Name == sourcePath {
			return scope.Scope, nil
		}
	}
	return "", fmt.Errorf("scope not found for source %v", sourcePath)
}

// GetScopeForSourceName returns the scope for a given source name (frontend-style)
// Uses the global SourceNameResolver to convert source names to paths
func (u *User) GetScopeForSourceName(sourceName string) (string, error) {
	if sourceNameResolver == nil {
		return "", fmt.Errorf("source name resolver not initialized")
	}

	sourcePath, err := sourceNameResolver(sourceName)
	if err != nil {
		return "", err
	}

	return u.GetScopeForSourcePath(sourcePath)
}

// HasSourceByPath checks if the user has access to a given source path
func (u *User) HasSourceByPath(sourcePath string) bool {
	for _, scope := range u.Scopes {
		if scope.Name == sourcePath {
			return true
		}
	}
	return false
}

// GetSourcePaths returns all source paths the user has access to
func (u *User) GetSourcePaths() []string {
	paths := make([]string, 0, len(u.Scopes))
	for _, scope := range u.Scopes {
		paths = append(paths, scope.Name)
	}
	return paths
}
