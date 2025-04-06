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
	Commands     map[string][]string `json:"commands"`
	Shell        []string            `json:"shell"`
	Server       Server              `json:"server"`
	Auth         Auth                `json:"auth"`
	Frontend     Frontend            `json:"frontend"`
	Users        []UserDefaults      `json:"users,omitempty"`
	UserDefaults UserDefaults        `json:"userDefaults"`
	Integrations Integrations        `json:"integrations"`
}

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"`
	Recaptcha            Recaptcha    `json:"recaptcha"`
	Methods              LoginMethods `json:"methods"`
	Command              string       `json:"command"`
	Signup               bool         `json:"signup"`
	Method               string       `json:"method"`
	Shell                string       `json:"shell"`
	Key                  []byte       `json:"key"`
	AdminUsername        string       `json:"adminUsername"`
	AdminPassword        string       `json:"adminPassword"`
	AuthMethods          []string
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy"`
	NoAuth       bool               `json:"noauth"`
	PasswordAuth PasswordAuthConfig `json:"password"`
}

type PasswordAuthConfig struct {
	Enabled   bool `json:"enabled"`
	MinLength int  `json:"minLength"`
}

type ProxyAuthConfig struct {
	Enabled    bool   `json:"enabled"`
	CreateUser bool   `json:"createUser"`
	Header     string `json:"header"`
}

type Recaptcha struct {
	Host   string `json:"host"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type Server struct {
	NumImageProcessors int         `json:"numImageProcessors"`
	Socket             string      `json:"socket"`
	TLSKey             string      `json:"tlsKey"`
	TLSCert            string      `json:"tlsCert"`
	EnableThumbnails   bool        `json:"enableThumbnails"`
	ResizePreview      bool        `json:"resizePreview"`
	EnableExec         bool        `json:"enableExec"`
	AuthHook           string      `json:"authHook"`
	Port               int         `json:"port"`
	BaseURL            string      `json:"baseURL"`
	Logging            []LogConfig `json:"logging"`
	Database           string      `json:"database"`
	Root               string      `json:"root"` // deprecated, use sources
	UserHomeBasePath   string      `json:"userHomeBasePath"`
	Sources            []Source    `json:"sources"`
	ExternalUrl        string      `json:"externalUrl"`
	InternalUrl        string      `json:"internalUrl"` // used by integrations
	CacheDir           string      `json:"cacheDir"`
	MaxArchiveSizeGB   int64       `json:"maxArchiveSize"`
	// not exposed to config
	SourceMap     map[string]Source // uses realpath as key
	NameToSource  map[string]Source // uses name as key
	DefaultSource Source
}

type Integrations struct {
	OnlyOffice OnlyOffice `json:"office"`
}

// onlyoffice secret is stored in the local.json file
// docker exec <containerID> /var/www/onlyoffice/documentserver/npm/json -f /etc/onlyoffice/documentserver/local.json 'services.CoAuthoring.secret.session.string'
type OnlyOffice struct {
	Url    string `json:"url"`
	Secret string `json:"secret"`
}

type LogConfig struct {
	Levels    string `json:"levels"`
	ApiLevels string `json:"apiLevels"`
	Output    string `json:"output"`
	NoColors  bool   `json:"noColors"`
	Json      bool   `json:"json"`
}

type Source struct {
	Path   string       `json:"path"` // can be relative, filesystem path
	Name   string       `json:"name"` // display name
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
	Files                 string         `json:"files"`
	Color                 string         `json:"color"`
	ExternalLinks         []ExternalLink `json:"externalLinks"`
}

type ExternalLink struct {
	Text  string `json:"text"`
	Title string `json:"title"`
	Url   string `json:"url"`
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	StickySidebar        bool                `json:"stickySidebar"`
	DarkMode             bool                `json:"darkMode"`
	LockPassword         bool                `json:"lockPassword"`
	DisableSettings      bool                `json:"disableSettings,omitempty"`
	Scope                string              `json:"scope"` // deprecated
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
