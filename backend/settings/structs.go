package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

type AllowedMethods string

const (
	ProxyAuth    AllowedMethods = "proxyAuth"
	NoAuth       AllowedMethods = "noAuth"
	PasswordAuth AllowedMethods = "passwordAuth"
)

type Settings struct {
	Server       Server         `json:"server"`
	Auth         Auth           `json:"auth"`
	Frontend     Frontend       `json:"frontend"`
	Users        []UserDefaults `json:"users,omitempty"`
	UserDefaults UserDefaults   `json:"userDefaults"`
	Integrations Integrations   `json:"integrations"`
}

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"`
	Methods              LoginMethods `json:"methods"`
	Signup               bool         `json:"signup"`
	Key                  []byte       `json:"key"`
	AdminUsername        string       `json:"adminUsername"`
	AdminPassword        string       `json:"adminPassword"`
	AuthMethods          []string     `json:"-"`
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy" validate:"omitempty"`
	NoAuth       bool               `json:"noauth" validate:"omitempty"`
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled   bool `json:"enabled" validate:"required"`
	MinLength int  `json:"minLength"`
}

type ProxyAuthConfig struct {
	Enabled    bool   `json:"enabled" validate:"required"`
	CreateUser bool   `json:"createUser"`
	Header     string `json:"header"`
}

type Server struct {
	NumImageProcessors int         `json:"numImageProcessors"`
	Socket             string      `json:"socket"`
	TLSKey             string      `json:"tlsKey"`
	TLSCert            string      `json:"tlsCert"`
	EnableThumbnails   bool        `json:"enableThumbnails"`
	ResizePreview      bool        `json:"resizePreview"`
	Port               int         `json:"port"`
	BaseURL            string      `json:"baseURL"`
	Logging            []LogConfig `json:"logging"`
	Database           string      `json:"database"`
	Sources            []Source    `json:"sources" validate:"required,dive"`
	ExternalUrl        string      `json:"externalUrl"`
	InternalUrl        string      `json:"internalUrl"` // used by integrations
	CacheDir           string      `json:"cacheDir"`
	MaxArchiveSizeGB   int64       `json:"maxArchiveSize"`
	// not exposed to config
	SourceMap     map[string]Source `json:"-" validate:"omitempty"` // uses realpath as key
	NameToSource  map[string]Source `json:"-" validate:"omitempty"` // uses name as key
	DefaultSource Source            `json:"-" validate:"omitempty"`
}

type Integrations struct {
	OnlyOffice OnlyOffice `json:"office" validate:"omitempty"`
}

// onlyoffice secret is stored in the local.json file
// docker exec <containerID> /var/www/onlyoffice/documentserver/npm/json -f /etc/onlyoffice/documentserver/local.json 'services.CoAuthoring.secret.session.string'
type OnlyOffice struct {
	Url    string `json:"url" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

type LogConfig struct {
	Levels    string `json:"levels"`
	ApiLevels string `json:"apiLevels"`
	Output    string `json:"output"`
	NoColors  bool   `json:"noColors"`
	Json      bool   `json:"json"`
}

type Source struct {
	Path   string       `json:"path" validate:"required"` // file system path. (Can be relative)
	Name   string       `json:"name"`                     // display name
	Config SourceConfig `json:"config"`
}

type SourceConfig struct {
	IndexingInterval      uint32      `json:"indexingInterval"`
	Disabled              bool        `json:"disabled"`
	MaxWatchers           int         `json:"maxWatchers"`
	NeverWatch            []string    `json:"neverWatchPaths"`
	IgnoreHidden          bool        `json:"ignoreHidden"`
	IgnoreZeroSizeFolders bool        `json:"ignoreZeroSizeFolders"`
	Exclude               IndexFilter `json:"exclude"`
	Include               IndexFilter `json:"include"`
	DefaultUserScope      string      `json:"defaultUserScope"` // default "" should match folders under path
	DefaultEnabled        bool        `json:"defaultEnabled"`
	CreateUserDir         bool        `json:"createUserDir"`
}

type IndexFilter struct {
	Files        []string `json:"files"`
	Folders      []string `json:"folders"`
	FileEndsWith []string `json:"fileEndsWith"`
}

type Frontend struct {
	Name                  string         `json:"name"`
	DisableDefaultLinks   bool           `json:"disableDefaultLinks"`
	DisableUsedPercentage bool           `json:"disableUsedPercentage"`
	ExternalLinks         []ExternalLink `json:"externalLinks"`
}

type ExternalLink struct {
	Text  string `json:"text" validate:"required"`
	Title string `json:"title"`
	Url   string `json:"url" validate:"required"`
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	StickySidebar        bool                `json:"stickySidebar"`
	DarkMode             bool                `json:"darkMode"`
	LockPassword         bool                `json:"lockPassword"`
	DisableSettings      bool                `json:"disableSettings,omitempty"`
	Locale               string              `json:"locale"`
	ViewMode             string              `json:"viewMode"`
	GallerySize          int                 `json:"gallerySize"`
	SingleClick          bool                `json:"singleClick"`
	Permissions          users.Permissions   `json:"permissions"`
	ShowHidden           bool                `json:"showHidden"`
	DateFormat           bool                `json:"dateFormat"`
	ThemeColor           string              `json:"themeColor"`
	QuickDownload        bool                `json:"quickDownload"`
	DisableOnlyOfficeExt string              `json:"disableOnlyOfficeExt"`
	DefaultScopes        []users.SourceScope `json:"-"`
}
