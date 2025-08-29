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
	ExternalUrl                  string      `json:"externalUrl"`    // used by share links if set (eg. http://mydomain.com)
	InternalUrl                  string      `json:"internalUrl"`    // used by integrations if set, this is the base domain that an integration service will use to communicate with filebrowser (eg. http://localhost:8080)
	CacheDir                     string      `json:"cacheDir"`       // path to the cache directory, used for thumbnails and other cached files
	MaxArchiveSizeGB             int64       `json:"maxArchiveSize"` // max pre-archive combined size of files/folder that are allowed to be archived (in GB)
	// not exposed to config
	SourceMap      map[string]Source `json:"-" validate:"omitempty"` // uses realpath as key
	NameToSource   map[string]Source `json:"-" validate:"omitempty"` // uses name as key
	MuPdfAvailable bool              `json:"-"`                      // used internally if compiled with mupdf support
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
	DenyByDefault    bool               `json:"denyByDefault"`           // deny access unless an "allow" access rule was specifically created.
	Private          bool               `json:"private"`                 // designate as source as private -- currently just means no sharing permitted.
	Disabled         bool               `json:"disabled"`                // disable the source, this is useful so you don't need to remove it from the config file
	IndexingInterval uint32             `json:"indexingIntervalMinutes"` // optional manual overide interval in minutes to re-index the source
	DisableIndexing  bool               `json:"disableIndexing"`         // disable the indexing of this source
	MaxWatchers      int                `json:"maxWatchers"`             // number of concurrent watchers to use for this source, currently not supported
	NeverWatchPaths  []string           `json:"neverWatchPaths"`         // paths that get initially once. Useful for folders that rarely change contents (without source path prefix)
	Exclude          ExcludeIndexFilter `json:"exclude"`                 // exclude files and folders from indexing, if include is not set
	Include          IncludeIndexFilter `json:"include"`                 // include files and folders from indexing, if exclude is not set
	DefaultUserScope string             `json:"defaultUserScope"`        // default "/" should match folders under path
	DefaultEnabled   bool               `json:"defaultEnabled"`          // should be added as a default source for new users?
	CreateUserDir    bool               `json:"createUserDir"`           // create a user directory for each user
}

type IncludeIndexFilter struct {
	RootFolders []string `json:"rootFolders"` // list of root folders to include, relative to the source path (eg. "folder1")
	RootFiles   []string `json:"rootFiles"`   // list of root files to include, relative to the source path (eg. "file1.txt")
}

type ExcludeIndexFilter struct {
	Hidden          bool     `json:"hidden"`                // exclude hidden files and folders.
	ZeroSizeFolders bool     `json:"ignoreZeroSizeFolders"` // ignore folders with 0 size
	FilePaths       []string `json:"filePaths"`             // list of filepaths Eg. "folder1" or "file1.txt" or "folder1/file1.txt" (without source path prefix)
	FolderPaths     []string `json:"folderPaths"`           // (filepath) list of folder names to include/exclude. Eg. "folder1" or "folder1/subfolder" (do not include source path, just the subpaths from the source path)
	FileNames       []string `json:"fileNames"`             // (global) list of file names to include/exclude. Eg. "a.jpg"
	FolderNames     []string `json:"folderNames"`           // (global) list of folder names to include/exclude. Eg. "@eadir" or ".thumbnails"
	FileEndsWith    []string `json:"fileEndsWith"`          // (global) exclude files that end with these suffixes. Eg. ".jpg" or ".txt"
	FolderEndsWith  []string `json:"folderEndsWith"`        // (global) exclude folders that end with these suffixes. Eg. ".thumbnails" or ".git"
}

type Frontend struct {
	Name                  string         `json:"name"`                  // display name
	DisableDefaultLinks   bool           `json:"disableDefaultLinks"`   // disable default links in the sidebar
	DisableUsedPercentage bool           `json:"disableUsedPercentage"` // disable used percentage for the sources in the sidebar
	ExternalLinks         []ExternalLink `json:"externalLinks"`
	DisableNavButtons     bool           `json:"disableNavButtons"` // disable the nav buttons in the sidebar
	Styling               StylingConfig  `json:"styling"`
}

type StylingConfig struct {
	CustomCSS          string                 `json:"customCSS"`       // if a valid path to a css file is provided, it will be applied for all users. (eg. "reduce-rounded-corners.css")
	LightBackground    string                 `json:"lightBackground"` // specify a valid CSS color property value to use as the background color in light mode
	DarkBackground     string                 `json:"darkBackground"`  // Specify a valid CSS color property value to use as the background color in dark mode
	CustomThemes       map[string]CustomTheme `json:"customThemes"`    // A list of custom css files that each user can select to override the default styling. if "default" is key name then it will be the default option.
	CustomThemeOptions map[string]CustomTheme `json:"-"`               // not exposed
}

type CustomTheme struct {
	Description string `json:"description"`   // The description of the theme to display in the UI.
	CSS         string `json:"css,omitempty"` // The css file path and filename to use for the theme.
}

type ExternalLink struct {
	Text  string `json:"text" validate:"required"` // the text to display on the link
	Title string `json:"title"`                    // the title to display on hover
	Url   string `json:"url" validate:"required"`  // the url to link to
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	EditorQuickSave            bool                `json:"editorQuickSave"`           // show quick save button in editor
	HideSidebarFileActions     bool                `json:"hideSidebarFileActions"`    // hide the file actions in the sidebar
	DisableQuickToggles        bool                `json:"disableQuickToggles"`       // disable the quick toggles in the sidebar
	DisableSearchOptions       bool                `json:"disableSearchOptions"`      // disable the search options in the search bar
	StickySidebar              bool                `json:"stickySidebar"`             // keep sidebar open when navigating
	DarkMode                   bool                `json:"darkMode"`                  // should dark mode be enabled
	Locale                     string              `json:"locale"`                    // language to use: eg. de, en, or fr
	ViewMode                   string              `json:"viewMode"`                  // view mode to use: eg. normal, list, grid, or compact
	SingleClick                bool                `json:"singleClick"`               // open directory on single click, also enables middle click to open in new tab
	ShowHidden                 bool                `json:"showHidden"`                // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	DateFormat                 bool                `json:"dateFormat"`                // when false, the date is relative, when true, the date is an exact timestamp
	GallerySize                int                 `json:"gallerySize"`               // 0-9 - the size of the gallery thumbnails
	ThemeColor                 string              `json:"themeColor"`                // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	QuickDownload              bool                `json:"quickDownload"`             // show icon to download in one click
	DisablePreviewExt          string              `json:"disablePreviewExt"`         // comma separated list of file extensions to disable preview for
	DisableViewingExt          string              `json:"disableViewingExt"`         // comma separated list of file extensions to disable viewing for
	LockPassword               bool                `json:"lockPassword"`              // disable the user from changing their password
	DisableSettings            bool                `json:"disableSettings,omitempty"` // disable the user from viewing the settings page
	Preview                    users.Preview       `json:"preview"`
	DefaultScopes              []users.SourceScope `json:"-"`
	Permissions                users.Permissions   `json:"permissions"`
	LoginMethod                string              `json:"loginMethod,omitempty"`      // login method to use: eg. password, proxy, oidc
	DisableUpdateNotifications bool                `json:"disableUpdateNotifications"` // disable update notifications banner for admin users
	DeleteWithoutConfirming    bool                `json:"deleteWithoutConfirming"`    // delete files without confirmation
	FileLoading                users.FileLoading   `json:"fileLoading"`                // upload and download settings
	DisableOfficePreviewExt    string              `json:"disableOfficePreviewExt"`    // deprecated: use disablePreviewExt instead
	DisableOnlyOfficeExt       string              `json:"disableOnlyOfficeExt"`       // list of file extensions to disable onlyoffice editor for
	CustomTheme                string              `json:"customTheme"`                // Name of theme to use chosen from custom themes config.
	ShowSelectMultiple         bool                `json:"showSelectMultiple"`         // show select multiple files on desktop
}
