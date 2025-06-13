package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

type AllowedMethods string

const (
	ProxyAuth    AllowedMethods = "proxyAuth"
	NoAuth       AllowedMethods = "noAuth"
	PasswordAuth AllowedMethods = "passwordAuth"
)

type Settings struct {
	Server       Server       `json:"server"`
	Auth         Auth         `json:"auth"`
	Frontend     Frontend     `json:"frontend"`
	UserDefaults UserDefaults `json:"userDefaults"`
	Integrations Integrations `json:"integrations"`
}

type Server struct {
	NumImageProcessors           int         `json:"numImageProcessors"`           // number of concurrent image processing jobs used to create previews, default is number of cpu cores available.
	Socket                       string      `json:"socket"`                       // socket to listen on
	TLSKey                       string      `json:"tlsKey"`                       // path to TLS key
	TLSCert                      string      `json:"tlsCert"`                      // path to TLS cert
	DisablePreviews              bool        `json:"disablePreviews"`              // disable all previews thumbnails, simple icons will be used
	DisableResize                bool        `json:"disablePreviewResize"`         // disable resizing of previews for faster loading over slow connections
	DisableTypeDetectionByHeader bool        `json:"disableTypeDetectionByHeader"` // disable type detection by header, useful if filesystem is slow.
	Port                         int         `json:"port"`                         // port to listen on
	BaseURL                      string      `json:"baseURL"`                      // base URL for the server, the subpath that the server is running on.
	Logging                      []LogConfig `json:"logging"`
	DebugMedia                   bool        `json:"debugMedia"` // output ffmpeg stdout for media integration -- careful can produces lots of output!
	Database                     string      `json:"database"`   // path to the database file
	Sources                      []Source    `json:"sources" validate:"required,dive"`
	ExternalUrl                  string      `json:"externalUrl"`    // used by share links if set
	InternalUrl                  string      `json:"internalUrl"`    // used by integrations if set, this is the url that an integration service will use to communicate with filebrowser
	CacheDir                     string      `json:"cacheDir"`       // path to the cache directory, used for thumbnails and other cached files
	MaxArchiveSizeGB             int64       `json:"maxArchiveSize"` // max pre-archive combined size of files/folder that are allowed to be archived (in GB)
	// not exposed to config
	SourceMap      map[string]Source `json:"-" validate:"omitempty"` // uses realpath as key
	NameToSource   map[string]Source `json:"-" validate:"omitempty"` // uses name as key
	DefaultSource  Source            `json:"-" validate:"omitempty"`
	MuPdfAvailable bool              `json:"-"` // used internally if compiled with mupdf support
}

type Integrations struct {
	OnlyOffice OnlyOffice `json:"office" validate:"omitempty"`
	Media      Media      `json:"media" validate:"omitempty"`
}

// onlyoffice secret is stored in the local.json file
// docker exec <containerID> /var/www/onlyoffice/documentserver/npm/json -f /etc/onlyoffice/documentserver/local.json 'services.CoAuthoring.secret.session.string'
type OnlyOffice struct {
	Url         string `json:"url" validate:"required"` // The URL to the OnlyOffice Document Server, needs to be accessible to the user.
	InternalUrl string `json:"internalUrl"`             // An optional internal address that the filebrowser server can use to communicate with the OnlyOffice Document Server, could be useful to bypass proxy.
	Secret      string `json:"secret" validate:"required"`
}

type Media struct {
	FfmpegPath string `json:"ffmpegPath"` // path to ffmpeg directory with ffmpeg and ffprobe (eg. /usr/local/bin)
}

type LogConfig struct {
	Levels    string `json:"levels"`    // separated list of log levels to enable. (eg. "info|warning|error|debug")
	ApiLevels string `json:"apiLevels"` // separated list of log levels to enable for the API. (eg. "info|warning|error")
	Output    string `json:"output"`    // output location. (eg. "stdout" or "path/to/file.log")
	NoColors  bool   `json:"noColors"`  // disable colors in the output
	Json      bool   `json:"json"`      // output in json format, currently not supported
	Utc       bool   `json:"utc"`       // use UTC time in the output instead of local time
}

type Source struct {
	Path   string       `json:"path" validate:"required"` // file system path. (Can be relative)
	Name   string       `json:"name"`                     // display name
	Config SourceConfig `json:"config"`
}

type SourceConfig struct {
	IndexingInterval      uint32      `json:"indexingInterval"`      // optional manual overide interval in seconds to re-index the source
	DisableIndexing       bool        `json:"disableIndexing"`       // disable the indexing of this source
	MaxWatchers           int         `json:"maxWatchers"`           // number of concurrent watchers to use for this source, currently not supported
	NeverWatch            []string    `json:"neverWatchPaths"`       // paths to never watch, relative to the source path (eg. "/folder/file.txt")
	IgnoreHidden          bool        `json:"ignoreHidden"`          // ignore hidden files and folders.
	IgnoreZeroSizeFolders bool        `json:"ignoreZeroSizeFolders"` // ignore folders with 0 size
	Exclude               IndexFilter `json:"exclude"`               // exclude files and folders from indexing, if include is not set
	Include               IndexFilter `json:"include"`               // include files and folders from indexing, if exclude is not set
	DefaultUserScope      string      `json:"defaultUserScope"`      // default "/" should match folders under path
	DefaultEnabled        bool        `json:"defaultEnabled"`        // should be added as a default source for new users?
	CreateUserDir         bool        `json:"createUserDir"`         // create a user directory for each user
}

type IndexFilter struct {
	Files        []string `json:"files"`        // array of file names to include/exclude
	Folders      []string `json:"folders"`      // array of folder names to include/exclude
	FileEndsWith []string `json:"fileEndsWith"` // array of file names to include/exclude (eg "a.jpg")
}

type Frontend struct {
	Name                  string         `json:"name"`                  // display name
	DisableDefaultLinks   bool           `json:"disableDefaultLinks"`   // disable default links in the sidebar
	DisableUsedPercentage bool           `json:"disableUsedPercentage"` // disable used percentage for the sources in the sidebar
	ExternalLinks         []ExternalLink `json:"externalLinks"`
}

type ExternalLink struct {
	Text  string `json:"text" validate:"required"` // the text to display on the link
	Title string `json:"title"`                    // the title to display on hover
	Url   string `json:"url" validate:"required"`  // the url to link to
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	StickySidebar           bool                `json:"stickySidebar"`             // keep sidebar open when navigating
	DarkMode                bool                `json:"darkMode"`                  // should dark mode be enabled
	Locale                  string              `json:"locale"`                    // language to use: eg. de, en, or fr
	ViewMode                string              `json:"viewMode"`                  // view mode to use: eg. normal, list, grid, or compact
	SingleClick             bool                `json:"singleClick"`               // open directory on single click, also enables middle click to open in new tab
	ShowHidden              bool                `json:"showHidden"`                // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	DateFormat              bool                `json:"dateFormat"`                // when false, the date is relative, when true, the date is an exact timestamp
	GallerySize             int                 `json:"gallerySize"`               // 0-9 - the size of the gallery thumbnails
	ThemeColor              string              `json:"themeColor"`                // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	QuickDownload           bool                `json:"quickDownload"`             // show icon to download in one click
	DisableOnlyOfficeExt    string              `json:"disableOnlyOfficeExt"`      // comma separated list of file extensions to disable onlyoffice preview for
	DisableOfficePreviewExt string              `json:"disableOfficePreviewExt"`   // comma separated list of file extensions to disable office preview for
	LockPassword            bool                `json:"lockPassword"`              // disable the user from changing their password
	DisableSettings         bool                `json:"disableSettings,omitempty"` // disable the user from viewing the settings page
	Preview                 users.Preview       `json:"preview"`
	DefaultScopes           []users.SourceScope `json:"-"`
	Permissions             users.Permissions   `json:"permissions"`
	LoginMethod             string              `json:"loginMethod,omitempty"` // login method to use: eg. password, proxy, oidc
}
