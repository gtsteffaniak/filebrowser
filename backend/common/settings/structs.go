package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

type AllowedMethods string

var Env Environment

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

type Environment struct {
	IsPlaywright          bool   `json:"-"`
	IsDevMode             bool   `json:"-"`
	IsFirstLoad           bool   `json:"-"` // used internally to track if this is the first load of the application
	MuPdfAvailable        bool   `json:"-"` // used internally if compiled with mupdf support
	EmbeddedFs            bool   `json:"-"` // used internally if compiled with embedded fs support
	FFmpegPath            string `json:"-"`
	FFprobePath           string `json:"-"`
	LoginIconPath         string `json:"-"` // resolved login icon path (filesystem or embedded)
	LoginIconIsCustom     bool   `json:"-"` // true if login icon is from custom filesystem path
	LoginIconEmbeddedPath string `json:"-"` // embedded asset path for default icon
	FaviconPath           string `json:"-"` // resolved favicon path (filesystem or embedded)
	FaviconIsCustom       bool   `json:"-"` // true if favicon is from custom filesystem path
	FaviconEmbeddedPath   string `json:"-"` // embedded asset path for default favicon
	PWAIconsDir           string `json:"-"` // directory where generated PWA icons are stored
	PWAIcon192            string `json:"-"` // path to 192x192 PWA icon
	PWAIcon256            string `json:"-"` // path to 256x256 PWA icon
	PWAIcon512            string `json:"-"` // path to 512x512 PWA icon
}

type Server struct {
	MinSearchLength              int            `json:"minSearchLength" yaml:"minSearchLength"` // minimum length of search query to begin searching (default: 3)
	DisableUpdateCheck           bool           `json:"disableUpdateCheck"`                     // disables backend update check service
	NumImageProcessors           int            `json:"numImageProcessors"`                     // number of concurrent image processing jobs used to create previews, default is number of cpu cores available.
	Socket                       string         `json:"socket"`                                 // socket to listen on
	TLSKey                       string         `json:"tlsKey"`                                 // path to TLS key
	TLSCert                      string         `json:"tlsCert"`                                // path to TLS cert
	DisablePreviews              bool           `json:"disablePreviews"`                        // disable all previews thumbnails, simple icons will be used
	DisableResize                bool           `json:"disablePreviewResize"`                   // disable resizing of previews for faster loading over slow connections
	DisableTypeDetectionByHeader bool           `json:"disableTypeDetectionByHeader"`           // disable type detection by header, useful if filesystem is slow.
	Port                         int            `json:"port"`                                   // port to listen on
	ListenAddress                string         `json:"listen"`                                 // address to listen on (default: 0.0.0.0)
	BaseURL                      string         `json:"baseURL"`                                // base URL for the server, the subpath that the server is running on.
	Logging                      []LogConfig    `json:"logging" yaml:"logging"`
	Database                     string         `json:"database"` // path to the database file
	Sources                      []*Source      `json:"sources" validate:"required,dive"`
	ExternalUrl                  string         `json:"externalUrl"`     // used by share links if set (eg. http://mydomain.com)
	InternalUrl                  string         `json:"internalUrl"`     // used by integrations if set, this is the base domain that an integration service will use to communicate with filebrowser (eg. http://localhost:8080)
	CacheDir                     string         `json:"cacheDir"`        // path to the cache directory, used for thumbnails and other cached files
	CacheDirCleanup              bool           `json:"cacheDirCleanup"` // whether to automatically cleanup the cache directory. Note: docker must also mount a persistent volume to persist the cache (default: false)
	MaxArchiveSizeGB             int64          `json:"maxArchiveSize"`  // max pre-archive combined size of files/folder that are allowed to be archived (in GB)
	Filesystem                   Filesystem     `json:"filesystem"`      // filesystem settings
	IndexSqlConfig               IndexSqlConfig `json:"indexSqlConfig"`  // Index database SQL configuration
	DisableWebDAV                bool           `json:"disableWebDAV"`   // disable webdav support (default: false)
	// not exposed to config
	SourceMap    map[string]*Source `json:"-" validate:"omitempty"` // uses realpath as key
	NameToSource map[string]*Source `json:"-" validate:"omitempty"` // uses name as key
}

type Filesystem struct {
	CreateFilePermission      string `json:"createFilePermission" validate:"required,file_permission"`      // Unix permissions like 644, 755, 2755 (default: 644)
	CreateDirectoryPermission string `json:"createDirectoryPermission" validate:"required,file_permission"` // Unix permissions like 755, 2755, 1777 (default: 755)
}

type IndexSqlConfig struct {
	BatchSize    int  `json:"batchSize"`    // number of items to batch in a single transaction, typically 500-5000. higher = faster but could use more memory.
	CacheSizeMB  int  `json:"cacheSizeMB"`  // size of the SQLite cache in MB
	WalMode      bool `json:"walMode"`      // enable the more complex WAL journaling mode. Slower, more memory usage, but better for deployments with constant user activity.
	DisableReuse bool `json:"disableReuse"` // enable to always create a new indexing database on startup.
}

type Integrations struct {
	OnlyOffice OnlyOffice `json:"office" validate:"omitempty"`
	Media      Media      `json:"media" validate:"omitempty"`
}

// onlyoffice secret is stored in the local.json file
// docker exec <containerID> /var/www/onlyoffice/documentserver/npm/json -f /etc/onlyoffice/documentserver/local.json 'services.CoAuthoring.secret.session.string'
type OnlyOffice struct {
	Url         string `json:"url" validate:"required"`    // The URL to the OnlyOffice Document Server, needs to be accessible to the user.
	InternalUrl string `json:"internalUrl"`                // An optional internal address that the filebrowser server can use to communicate with the OnlyOffice Document Server, could be useful to bypass proxy.
	Secret      string `json:"secret" validate:"required"` // secret: authentication key for OnlyOffice integration
	ViewOnly    bool   `json:"viewOnly"`                   // view only mode for OnlyOffice
}

type Media struct {
	FfmpegPath               string        `json:"ffmpegPath"`               // path to ffmpeg directory with ffmpeg and ffprobe (eg. /usr/local/bin)
	Convert                  FfmpegConvert `json:"convert"`                  // config for ffmpeg conversion settings
	Debug                    bool          `json:"debug"`                    // output ffmpeg stdout for media integration -- careful can produces lots of output!
	ExtractEmbeddedSubtitles bool          `json:"extractEmbeddedSubtitles"` // extract embedded subtitles from media files
}

type FfmpegConvert struct {
	ImagePreview map[ImagePreviewType]*bool `json:"imagePreview"` // supported image preview formats. defaults vary by type (see individual type docs)
	VideoPreview map[VideoPreviewType]*bool `json:"videoPreview"` // supported video preview formats. defaults to true for all types unless explicitly disabled.
}

type ImagePreviewType string

const (
	HEICImagePreview ImagePreviewType = "heic"
	JPEGImagePreview ImagePreviewType = "jpeg" // only used as fallback for JPEG formats that can't otherwise be decoded
	//RAWImagePreview  ImagePreviewType = "raw"
)

func (i ImagePreviewType) String() string {
	return string(i)
}

// AllImagePreviewTypes contains all supported image preview types.
var AllImagePreviewTypes = []ImagePreviewType{
	HEICImagePreview,
	JPEGImagePreview,
	//RAWImagePreview,
}

type VideoPreviewType string

const (
	MP4VideoPreview      VideoPreviewType = "mp4"
	WebMVideoPreview     VideoPreviewType = "webm"
	MOVVideoPreview      VideoPreviewType = "mov"
	AVIVideoPreview      VideoPreviewType = "avi"
	MKVVideoPreview      VideoPreviewType = "mkv"
	FLVVideoPreview      VideoPreviewType = "flv"
	WMVVideoPreview      VideoPreviewType = "wmv"
	M4VVideoPreview      VideoPreviewType = "m4v"
	ThreeGPVideoPreview  VideoPreviewType = "3gp"
	ThreeGP2VideoPreview VideoPreviewType = "3g2"
	TSVideoPreview       VideoPreviewType = "ts"
	M2TSVideoPreview     VideoPreviewType = "m2ts"
	VOBVideoPreview      VideoPreviewType = "vob"
	ASFVideoPreview      VideoPreviewType = "asf"
	MPGVideoPreview      VideoPreviewType = "mpg"
	MPEGVideoPreview     VideoPreviewType = "mpeg"
	F4VVideoPreview      VideoPreviewType = "f4v"
	OGVVideoPreview      VideoPreviewType = "ogv"
)

func (v VideoPreviewType) String() string {
	return string(v)
}

// AllVideoPreviewTypes contains all supported video preview types.
var AllVideoPreviewTypes = []VideoPreviewType{
	MP4VideoPreview,
	WebMVideoPreview,
	MOVVideoPreview,
	AVIVideoPreview,
	MKVVideoPreview,
	FLVVideoPreview,
	WMVVideoPreview,
	M4VVideoPreview,
	ThreeGPVideoPreview,
	ThreeGP2VideoPreview,
	TSVideoPreview,
	M2TSVideoPreview,
	VOBVideoPreview,
	ASFVideoPreview,
	MPGVideoPreview,
	MPEGVideoPreview,
	F4VVideoPreview,
	OGVVideoPreview,
}

type LogConfig struct {
	Levels    string `json:"levels" yaml:"levels"`       // separated list of log levels to enable. (eg. "info|warning|error|debug")
	ApiLevels string `json:"apiLevels" yaml:"apiLevels"` // separated list of log levels to enable for the API. (eg. "info|warning|error")
	Output    string `json:"output" yaml:"output"`       // output location. (eg. "stdout" or "path/to/file.log")
	NoColors  bool   `json:"noColors" yaml:"noColors"`   // disable colors in the output
	Json      bool   `json:"json" yaml:"json"`           // output in json format
	Utc       bool   `json:"utc" yaml:"utc"`             // use UTC time in the output instead of local time
}

type Source struct {
	Path   string       `json:"path" validate:"required"` // file system path. (Can be relative)
	Name   string       `json:"name"`                     // display name
	Config SourceConfig `json:"config,omitempty"`
}

type SourceConfig struct {
	DenyByDefault    bool              `json:"denyByDefault,omitempty"`           // deny access unless an "allow" access rule was specifically created.
	Private          bool              `json:"private"`                           // designate as source as private -- currently just means no sharing permitted.
	Disabled         bool              `json:"disabled,omitempty"`                // disable the source, this is useful so you don't need to remove it from the config file
	IndexingInterval uint32            `json:"indexingIntervalMinutes,omitempty"` // deprecated: create a rule with indexingIntervalMinutes to set the indexing interval for this source
	DisableIndexing  bool              `json:"disableIndexing,omitempty"`         // deprecated: use indexingDisabled instead to disable the indexing of this source
	Conditionals     ConditionalFilter `json:"conditionals"`                      // deprecated: use source.rules instead
	Rules            []ConditionalRule `json:"rules"`                             // list of item rules to apply to specific paths
	DefaultUserScope string            `json:"defaultUserScope"`                  // defaults to root of index "/" should match folders under path
	DefaultEnabled   bool              `json:"defaultEnabled"`                    // should be added as a default source for new users?
	CreateUserDir    bool              `json:"createUserDir"`                     // create a user directory for each user under defaultUserScope + username
	UseLogicalSize   bool              `json:"useLogicalSize"`                    // calculate sizes based on logical size instead of disk utilization (du -sh), folders will be 0 bytes when empty.
	// hidden but used internally - optimized map lookups for conditional rules
	ResolvedRules ResolvedRulesConfig `json:"-"`
}

type ConditionalFilter struct {
	Hidden          bool              `json:"hidden"`                // deprecated: use ignoreHidden instead. eg, FolderPath: "/" and ignoreHidden: true will exclude hidden files and folders under the root folder.
	IgnoreHidden    bool              `json:"ignoreHidden"`          // deprecated: use ignoreHidden instead. eg, FolderPath: "/" and ignoreHidden: true will exclude hidden files and folders under the root folder.
	ZeroSizeFolders bool              `json:"ignoreZeroSizeFolders"` // deprecated: use ignoreZeroSizeFolders instead. eg, FolderPath: "/" and ignoreZeroSizeFolders: true will ignore folders with 0 size under the root folder.
	ItemRules       []ConditionalRule `json:"rules"`                 // list of item rules to apply to specific paths
}

type ConditionalRule struct {
	NeverWatchPath   string `json:"neverWatchPath"`   // index the folder in the first pass to get included in search, but never re-indexed.
	IncludeRootItem  string `json:"includeRootItem"`  // include only these items at root folder level
	FileStartsWith   string `json:"fileStartsWith"`   // (global) exclude files that start with these prefixes. Eg. "archive-" or "backup-"
	FolderStartsWith string `json:"folderStartsWith"` // (global) exclude folders that start with these prefixes. Eg. "archive-" or "backup-"
	FileEndsWith     string `json:"fileEndsWith"`     // (global) exclude files that end with these suffixes. Eg. ".jpg" or ".txt"
	FolderEndsWith   string `json:"folderEndsWith"`   // (global) exclude folders that end with these suffixes. Eg. ".thumbnails" or ".git"
	FolderPath       string `json:"folderPath"`       // (global) exclude folders that match this path. Eg. "/path/to/folder" or "/path/to/folder/subfolder"
	FilePath         string `json:"filePath"`         // (global) exclude files that match this path. Eg. "/path/to/file.txt" or "/path/to/file.txt/subfile.txt"
	FileNames        string `json:"fileNames"`        // deprecated: exclude files that match these names. Eg. "file.txt" or "test.csv"
	FolderNames      string `json:"folderNames"`      // deprecated: exclude folders that match these names. Eg. "folder" or "subfolder"
	FileName         string `json:"fileName"`         // (global) exclude files that match these names. Eg. "file.txt" or "test.csv"
	FolderName       string `json:"folderName"`       // (global) exclude folders that match these names. Eg. "folder" or "subfolder"

	Viewable              bool `json:"viewable"`              // Enable viewing in UI but exclude from indexing
	IgnoreHidden          bool `json:"ignoreHidden"`          // Excludes only hidden files and folders
	IgnoreZeroSizeFolders bool `json:"ignoreZeroSizeFolders"` // Excludes only folders with 0 size
	IgnoreSymlinks        bool `json:"ignoreSymlinks"`        // Excludes symbolic links
}

// ConditionalMaps provides O(1) lookup performance for conditional rules
// Maps are built from ConditionalFilter during initialization
type ResolvedRulesConfig struct {
	// Exact match maps - O(1) lookup (only for names, not StartsWith/EndsWith)
	FileNames   map[string]ConditionalRule // key: file name
	FolderNames map[string]ConditionalRule // key: folder name

	// exact match for paths
	FilePaths   map[string]ConditionalRule // key: file path
	FolderPaths map[string]ConditionalRule // key: folder path

	FileEndsWith     []ConditionalRule // list of item rules that have been resolved for specific paths
	FolderEndsWith   []ConditionalRule // list of item rules that have been resolved for specific paths
	FileStartsWith   []ConditionalRule // list of item rules that have been resolved for specific paths
	FolderStartsWith []ConditionalRule // list of item rules that have been resolved for specific paths

	// NeverWatch paths map - O(1) lookup for all paths with neverWatch: true
	// This replaces the old NeverWatchPaths slice
	NeverWatchPaths  map[string]struct{} // key: full folder path
	IncludeRootItems map[string]struct{} // key: inclusive root item

	IgnoreAllHidden          bool // Excludes all hidden files and folders
	IgnoreAllZeroSizeFolders bool // Excludes all folders with 0 size
	IgnoreAllSymlinks        bool // Excludes all symbolic links
	IndexingDisabled         bool // Excludes all files and folders from indexing
}

type Frontend struct {
	Name                  string         `json:"name"`                  // display name
	DisableDefaultLinks   bool           `json:"disableDefaultLinks"`   // disable default links in the sidebar
	DisableUsedPercentage bool           `json:"disableUsedPercentage"` // disable used percentage for the sources in the sidebar
	ExternalLinks         []ExternalLink `json:"externalLinks"`
	DisableNavButtons     bool           `json:"disableNavButtons"` // disable the nav buttons in the sidebar
	Styling               StylingConfig  `json:"styling"`
	Favicon               string         `json:"favicon"`             // path to a favicon to use for the frontend
	Description           string         `json:"description"`         // description that shows up in html head meta description
	LoginIcon             string         `json:"loginIcon"`           // path to an image file for the login page icon
	LoginButtonText       string         `json:"loginButtonText"`     // text to display on the login button
	OIDCLoginButtonText   string         `json:"oidcLoginButtonText"` // text to display on the OIDC login button
}

type StylingConfig struct {
	DisableEventBasedThemes bool                   `json:"disableEventThemes"` // disable the event based themes,
	CustomCSS               string                 `json:"customCSS"`          // if a valid path to a css file is provided, it will be applied for all users. (eg. "reduce-rounded-corners.css")
	CustomCSSRaw            string                 `json:"-"`                  // The css raw content to use for the custom css.
	LightBackground         string                 `json:"lightBackground"`    // specify a valid CSS color property value to use as the background color in light mode
	DarkBackground          string                 `json:"darkBackground"`     // Specify a valid CSS color property value to use as the background color in dark mode
	CustomThemes            map[string]CustomTheme `json:"customThemes"`       // A list of custom css files that each user can select to override the default styling. if "default" is key name then it will be the default option.
	// In-memory (not exposed to config)
	CustomThemeOptions map[string]CustomTheme `json:"-"` // not exposed
}

type CustomTheme struct {
	Description string `json:"description"`   // The description of the theme to display in the UI.
	CSS         string `json:"css,omitempty"` // The css file path and filename to use for the theme.
	CssRaw      string `json:"-"`             // The css raw content to use for the theme.
}

type ExternalLink struct {
	Text  string `json:"text" validate:"required"` // the text to display on the link
	Title string `json:"title"`                    // the title to display on hover
	Url   string `json:"url" validate:"required"`  // the url to link to
}

// UserDefaultsPreview holds preview settings with pointer types for defaults
type UserDefaultsPreview struct {
	DisableHideSidebar bool  `json:"disableHideSidebar"` // keep sidebar open when previewing files
	HighQuality        *bool `json:"highQuality"`        // use high quality thumbnails
	Image              *bool `json:"image"`              // show thumbnails for image files
	Video              *bool `json:"video"`              // show thumbnails for video files
	MotionVideoPreview *bool `json:"motionVideoPreview"` // show multiple frames for videos in thumbnail preview when hovering
	Office             *bool `json:"office"`             // show thumbnails for office files
	PopUp              *bool `json:"popup"`              // show larger popup preview when hovering over thumbnail
	AutoplayMedia      *bool `json:"autoplayMedia"`      // autoplay media files in preview
	DefaultMediaPlayer bool  `json:"defaultMediaPlayer"` // disable the styled feature-rich media player for browser default
	Folder             *bool `json:"folder"`             // show thumbnails for folders that have previewable contents
}

// UserDefaultsPermissions holds permission settings with pointer types for defaults
type UserDefaultsPermissions struct {
	Api      bool  `json:"api"`      // allow api access
	Admin    bool  `json:"admin"`    // allow admin access
	Modify   bool  `json:"modify"`   // allow modifying files
	Share    bool  `json:"share"`    // allow sharing files
	Realtime bool  `json:"realtime"` // allow realtime updates
	Delete   bool  `json:"delete"`   // allow deleting files
	Create   bool  `json:"create"`   // allow creating or uploading files
	Download *bool `json:"download"` // allow downloading files
	Archive  bool  `json:"archive"`  // allow creating archives
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	EditorQuickSave            bool                    `json:"editorQuickSave"`           // show quick save button in editor
	HideSidebarFileActions     bool                    `json:"hideSidebarFileActions"`    // hide the file actions in the sidebar
	DisableQuickToggles        bool                    `json:"disableQuickToggles"`       // disable the quick toggles in the sidebar
	DisableSearchOptions       bool                    `json:"disableSearchOptions"`      // disable the search options in the search bar
	StickySidebar              bool                    `json:"stickySidebar"`             // keep sidebar open when navigating
	DarkMode                   *bool                   `json:"darkMode"`                  // should dark mode be enabled
	Locale                     string                  `json:"locale"`                    // language to use: eg. de, en, or fr
	ViewMode                   string                  `json:"viewMode"`                  // view mode to use: eg. normal, list, grid, or compact
	SingleClick                bool                    `json:"singleClick"`               // open directory on single click, also enables middle click to open in new tab
	ShowHidden                 bool                    `json:"showHidden"`                // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	DateFormat                 bool                    `json:"dateFormat"`                // when false, the date is relative, when true, the date is an exact timestamp
	GallerySize                int                     `json:"gallerySize"`               // 0-9 - the size of the gallery thumbnails
	ThemeColor                 string                  `json:"themeColor"`                // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	QuickDownload              bool                    `json:"quickDownload"`             // show icon to download in one click
	DisablePreviewExt          string                  `json:"disablePreviewExt"`         // space separated list of file extensions to disable preview for
	DisableViewingExt          string                  `json:"disableViewingExt"`         // space separated list of file extensions to disable viewing for
	LockPassword               bool                    `json:"lockPassword"`              // disable the user from changing their password
	DisableSettings            bool                    `json:"disableSettings,omitempty"` // disable the user from viewing the settings page
	Preview                    UserDefaultsPreview     `json:"preview"`
	DefaultScopes              []users.SourceScope     `json:"-"`
	Permissions                UserDefaultsPermissions `json:"permissions"`
	LoginMethod                string                  `json:"loginMethod,omitempty"`      // login method to use: eg. password, proxy, oidc
	DisableUpdateNotifications bool                    `json:"disableUpdateNotifications"` // disable update notifications banner for admin users
	DeleteWithoutConfirming    bool                    `json:"deleteWithoutConfirming"`    // delete files without confirmation
	FileLoading                users.FileLoading       `json:"fileLoading"`                // upload and download settings
	DisableOfficePreviewExt    string                  `json:"disableOfficePreviewExt"`    // deprecated: use disablePreviewExt instead
	DisableOnlyOfficeExt       string                  `json:"disableOnlyOfficeExt"`       // list of file extensions to disable onlyoffice editor for
	CustomTheme                string                  `json:"customTheme"`                // Name of theme to use chosen from custom themes config.
	ShowSelectMultiple         bool                    `json:"showSelectMultiple"`         // show select multiple files on desktop
	DebugOffice                bool                    `json:"debugOffice"`                // debug onlyoffice editor
}
