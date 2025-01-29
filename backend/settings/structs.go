package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

type Settings struct {
	Commands     map[string][]string `json:"commands"`
	Shell        []string            `json:"shell"`
	Rules        []users.Rule        `json:"rules"`
	Server       Server              `json:"server"`
	Auth         Auth                `json:"auth"`
	Frontend     Frontend            `json:"frontend"`
	Users        []UserDefaults      `json:"users,omitempty"`
	UserDefaults UserDefaults        `json:"userDefaults"`
	Integrations Integrations        `json:"integrations"`
}

type Auth struct {
	TokenExpirationHours int       `json:"tokenExpirationHours"`
	Recaptcha            Recaptcha `json:"recaptcha"`
	Header               string    `json:"header"`
	Method               string    `json:"method"`
	Command              string    `json:"command"`
	Signup               bool      `json:"signup"`
	Shell                string    `json:"shell"`
	AdminUsername        string    `json:"adminUsername"`
	AdminPassword        string    `json:"adminPassword"`
	Key                  []byte    `json:"key"`
	CreateUser           bool      `json:"createUser"`
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
	Root               string      `json:"root"`
	UserHomeBasePath   string      `json:"userHomeBasePath"`
	CreateUserDir      bool        `json:"createUserDir"`
	Sources            []Source    `json:"sources"`
	ExternalUrl        string      `json:"externalUrl"`
	InternalUrl        string      `json:"internalUrl"` // used by integrations
	CacheDir           string      `json:"cacheDir"`
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
	Path   string      `json:"path"`
	Name   string      `json:"name"`
	Config IndexConfig `json:"config"`
}

type IndexConfig struct {
	IndexingInterval      uint32      `json:"indexingInterval"`
	Disabled              bool        `json:"disabled"`
	MaxWatchers           int         `json:"maxWatchers"`
	NeverWatch            []string    `json:"neverWatchPaths"`
	IgnoreHidden          bool        `json:"ignoreHidden"`
	IgnoreZeroSizeFolders bool        `json:"ignoreZeroSizeFolders"`
	Exclude               IndexFilter `json:"exclude"`
	Include               IndexFilter `json:"include"`
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
	StickySidebar   bool         `json:"stickySidebar"`
	DarkMode        bool         `json:"darkMode"`
	LockPassword    bool         `json:"lockPassword"`
	DisableSettings bool         `json:"disableSettings,omitempty"`
	Scope           string       `json:"scope"`
	Locale          string       `json:"locale"`
	ViewMode        string       `json:"viewMode"`
	GallerySize     int          `json:"gallerySize"`
	SingleClick     bool         `json:"singleClick"`
	Rules           []users.Rule `json:"rules"`
	Sorting         struct {
		By  string `json:"by"`
		Asc bool   `json:"asc"`
	} `json:"sorting"`
	Perm        users.Permissions `json:"perm"`
	Permissions users.Permissions `json:"permissions"`
	Commands    []string          `json:"commands,omitempty"`
	ShowHidden  bool              `json:"showHidden"`
	DateFormat  bool              `json:"dateFormat"`
	ThemeColor  string            `json:"themeColor"`
}
