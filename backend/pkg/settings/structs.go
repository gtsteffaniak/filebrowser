package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
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
	UserDefaults UserDefaults `json:"userDefaults"` // optional signup/CLI defaults; per-user values are managed in the UI
	Integrations Integrations `json:"integrations"`
	Http         Http         `json:"http"`
}

type Http struct {
	TrustedHeadersArray []string `json:"trustedHeaders"`   // list of headers to trust, useful when behind a reverse proxy.
	DisableRateLimit    bool     `json:"disableRateLimit"` // turns off built-in auth route rate limiting and failed-login lockout (default false).

	// internal map of trusted headers
	TrustedHeaders map[string]bool `json:"-"`
}

type Environment struct {
	IsPlaywright                bool   `json:"-"`
	IsDevMode                   bool   `json:"-"`
	IsFirstLoad                 bool   `json:"-"` // used internally to track if this is the first load of the application
	ConfigUserDefaultsSpecified bool   `json:"-"` // true when the config file contained a userDefaults section
	MuPdfAvailable              bool   `json:"-"` // used internally if compiled with mupdf support
	EmbeddedFs                  bool   `json:"-"` // used internally if compiled with embedded fs support
	FFmpegPath                  string `json:"-"`
	FFprobePath                 string `json:"-"`
	FFmpegAvailable             bool   `json:"-"`
	LoginIconPath               string `json:"-"` // resolved login icon path (filesystem or embedded)
	LoginIconIsCustom           bool   `json:"-"` // true if login icon is from custom filesystem path
	LoginIconEmbeddedPath       string `json:"-"` // embedded asset path for default icon
	FaviconPath                 string `json:"-"` // resolved favicon path (filesystem or embedded)
	FaviconIsCustom             bool   `json:"-"` // true if favicon is from custom filesystem path
	FaviconEmbeddedPath         string `json:"-"` // embedded asset path for default favicon
}

type Server struct {
	MinSearchLength              int            `json:"minSearchLength" yaml:"minSearchLength"` // minimum length of search query to begin searching (default: 3)
	DisableUpdateCheck           bool           `json:"disableUpdateCheck"`                     // disables backend update check service
	NumImageProcessors           int            `json:"numImageProcessors"`                     // number of concurrent image processing jobs used to create previews, default is 4.
	Socket                       string         `json:"socket"`                                 // socket to listen on - eg. /var/run/filebrowser.sock
	TLSKey                       string         `json:"tlsKey"`                                 // path to TLS key
	TLSCert                      string         `json:"tlsCert"`                                // path to TLS cert
	DisablePreviews              bool           `json:"disablePreviews"`                        // disable all previews thumbnails, simple icons will be used
	DisableResize                bool           `json:"disablePreviewResize"`                   // disable resizing of previews for faster loading over slow connections
	DisableTypeDetectionByHeader bool           `json:"disableTypeDetectionByHeader"`           // disable type detection by header, useful if filesystem is slow.
	Port                         int            `json:"port"`                                   // port to listen on
	ListenAddress                string         `json:"listen"`                                 // address to listen on (default: 0.0.0.0)
	BaseURL                      string         `json:"baseURL"`                                // base URL for the server, the subpath that the server is running on.
	Logging                      []LogConfig    `json:"logging" yaml:"logging"`
	Sources                      []*Source      `json:"sources" validate:"required,dive"`
	ExternalUrl                  string         `json:"externalUrl"`     // used by share links if set (eg. http://mydomain.com)
	InternalUrl                  string         `json:"internalUrl"`     // used by integrations if set, this is the base domain that an integration service will use to communicate with filebrowser (eg. http://localhost:8080)
	CacheDir                     string         `json:"cacheDir"`        // path to the cache directory, used for thumbnails and other cached files
	CacheDirCleanup              bool           `json:"cacheDirCleanup"` // whether to automatically cleanup the cache directory. Note: docker must also mount a persistent volume to persist the cache (default: false)
	MaxArchiveSizeGB             int64          `json:"maxArchiveSize"`  // maximum archive/unarchive size in GB. 0 means no limit. (default: 20)
	Filesystem                   Filesystem     `json:"filesystem"`      // filesystem settings
	IndexSqlConfig               IndexSqlConfig `json:"indexSqlConfig"`  // Index database SQL configuration
	DisableWebDAV                bool           `json:"disableWebDAV"`   // disable webdav support (default: false)
	// not exposed to config
	SourceMap    map[string]*Source `json:"-" validate:"omitempty"` // uses realpath as key
	NameToSource map[string]*Source `json:"-" validate:"omitempty"` // uses name as key
	DatabaseV2   Database           `json:"database"`               // SQLite database configuration
}

type ActivityConfig struct {
	Disabled             bool `json:"disabled"`             // disable semantic activity audit logging (default: false)
	RetentionDays        int  `json:"retentionDays"`        // purge activity rows older than this many days (default 30)
	FlushIntervalSeconds int  `json:"flushIntervalSeconds"` // buffer flush interval in seconds (default 10)
	MaxBufferSize        int  `json:"maxBufferSize"`        // max in-memory buffer before immediate flush (default 10000)
}

type Database struct {
	Path        string         `json:"path"`        // path to SQLite database file
	MigrateFrom string         `json:"migrateFrom"` // path to old BoltDB database file for migration (optional)
	Activity    ActivityConfig `json:"activity"`    // activity audit logging configuration
}

type Filesystem struct {
	CreateFilePermission      string `json:"createFilePermission" validate:"required,file_permission"`      // Unix permissions like 644, 755, 2755 (default: 644)
	CreateDirectoryPermission string `json:"createDirectoryPermission" validate:"required,file_permission"` // Unix permissions like 755, 2755, 1777 (default: 755)
}

// Index SQL startup integrity modes (IndexSqlConfig.StartupIntegrityCheck).
const (
	// IndexStartupIntegrityQuickCheck runs PRAGMA quick_check (default). Slower on very large DBs but thorough.
	IndexStartupIntegrityQuickCheck = "quickCheck"
	// IndexStartupIntegrityProbe verifies sqlite_master and optionally one row read only; fast for large indexes.
	IndexStartupIntegrityProbe = "probe"
	// IndexStartupIntegrityOff skips startup checks beyond sql.Open Ping; least safe, fastest boot.
	IndexStartupIntegrityOff = "off"
)

type IndexSqlConfig struct {
	BatchSize             int    `json:"batchSize"`                                                                       // number of items to batch in a single transaction, typically 500-5000. higher = faster but could use more memory.
	CacheSizeMB           int    `json:"cacheSizeMB"`                                                                     // size of the SQLite cache in MB
	WalMode               bool   `json:"walMode"`                                                                         // enable the more complex WAL journaling mode. Slower, more memory usage, but better for deployments with constant user activity.
	DisableReuse          bool   `json:"disableReuse"`                                                                    // enable to always create a new indexing database on startup.
	StartupIntegrityCheck string `json:"startupIntegrityCheck,omitempty" validate:"omitempty,oneof=quickCheck probe off"` // the method used to check the integrity of the index database on startup (default: quickCheck)
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
	HardwareAcceleration     bool          `json:"hardwareAcceleration"`     // enable hardware acceleration for ffmpeg if available
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
	Utc       bool   `json:"utc" yaml:"utc"`             // use UTC time in the output instead of local time.
	ApiFilter string `json:"apiFilter" yaml:"apiFilter"` // regex filter that excludes matching full api paths from being logged. (eg. '/user\?id\=self') Defaults to '^/health|^/favicon.ico|^/static|^/public/static'
}
type Source struct {
	Path   string       `json:"path" validate:"required"` // file system path. (Can be relative)
	Name   string       `json:"name"`                     // display name
	Config SourceConfig `json:"config,omitempty"`
}

type SourceConfig struct {
	DenyByDefault    bool              `json:"denyByDefault,omitempty"` // deny access unless an "allow" access rule was specifically created.
	Private          bool              `json:"private"`                 // designate as source as private -- currently just means no sharing permitted.
	ReadOnly         bool              `json:"readOnly,omitempty"`      // read-only source, changes from the UI, webdav, and API will be disabled.
	Disabled         bool              `json:"disabled,omitempty"`      // disable the source, this is useful so you don't need to remove it from the config file
	Rules            []ConditionalRule `json:"rules"`                   // list of item rules to apply to specific paths
	DefaultUserScope string            `json:"defaultUserScope"`        // defaults to root of index "/" should match folders under path
	DefaultEnabled   bool              `json:"defaultEnabled"`          // should be added as a default source for new users?
	CreateUserDir    bool              `json:"createUserDir"`           // create a user directory for each user under defaultUserScope + username
	UseLogicalSize   bool              `json:"useLogicalSize"`          // calculate sizes based on logical size instead of disk utilization (du -sh), folders will be 0 bytes when empty.
	// DefaultFilePermissions is the template for new user scopes on this source (also synced globally via Access settings).
	DefaultFilePermissions users.SourceFilePermissions `json:"defaultFilePermissions,omitempty" yaml:"defaultFilePermissions,omitempty"`
	// hidden but used internally - optimized map lookups for conditional rules
	ResolvedRules ResolvedRulesConfig `json:"-"`
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
	FileName         string `json:"fileName"`         // (global) exclude files that match these names. Eg. "file.txt" or "test.csv"
	FolderName       string `json:"folderName"`       // (global) exclude folders that match these names. Eg. "folder" or "subfolder"

	Viewable              bool `json:"viewable"`              // Enable viewing in UI but exclude from indexing
	IgnoreHidden          bool `json:"ignoreHidden"`          // Excludes only hidden files and folders
	IgnoreZeroSizeFolders bool `json:"ignoreZeroSizeFolders"` // Excludes only folders with 0 size
	IgnoreSymlinks        bool `json:"ignoreSymlinks"`        // Excludes symbolic links
}

// ResolvedRulesConfig provides O(1) lookup performance for conditional rules.
// Maps are built from Rules during initialization.
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
	NoRules                  bool // No rules are configured
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

// UserDefaultsPermissions holds permission settings with pointer types for defaults
type UserDefaultsPermissions struct {
	Api      bool  `json:"api"`      // deprecated: use account.permissions.api instead. allow api access
	Admin    bool  `json:"admin"`    // deprecated: use account.permissions.admin instead. allow admin access
	Modify   bool  `json:"modify"`   // deprecated: use account.permissions.modify instead. allow modifying files
	Share    bool  `json:"share"`    // deprecated: use account.permissions.share instead. allow sharing files
	Realtime bool  `json:"realtime"` // deprecated: use account.permissions.realtime instead. allow realtime updates
	Delete   bool  `json:"delete"`   // deprecated: use account.permissions.delete instead. allow deleting files
	Create   bool  `json:"create"`   // deprecated: use account.permissions.create instead. allow creating or uploading files
	Download *bool `json:"download"` // deprecated: use account.permissions.download instead. allow downloading files
}

// New organized structures for UserDefaults

// UserDefaultsSidebar holds sidebar-related settings
type UserDefaultsSidebar struct {
	DisableQuickToggles  bool  `json:"disableQuickToggles"`  // disable the quick toggles in the sidebar
	HideFileActions      bool  `json:"hideFileActions"`      // hide the file actions in the sidebar
	DisableHideOnPreview bool  `json:"disableHideOnPreview"` // keep sidebar open when previewing files (was preview.disableHideSidebar)
	Sticky               bool  `json:"sticky"`               // keep sidebar open when navigating
	HideFiles            bool  `json:"hideFiles"`            // hide files in the sidebar tree navigation, when true, will show only directories
	ShowTools            *bool `json:"showTools"`            // show sidebar links with category "tool"; default is true
}

// UserDefaultsListing holds file listing display settings
type UserDefaultsListing struct {
	DeleteWithoutConfirming bool   `json:"deleteWithoutConfirming"` // delete files without confirmation
	DateFormat              bool   `json:"dateFormat"`              // when false, the date is relative, when true, the date is an exact timestamp
	ShowHidden              bool   `json:"showHidden"`              // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	QuickDownload           bool   `json:"quickDownload"`           // show icon to download in one click
	ShowSelectMultiple      bool   `json:"showSelectMultiple"`      // show select multiple files on desktop
	SingleClick             bool   `json:"singleClick"`             // open directory on single click, also enables middle click to open in new tab
	HideFileExt             string `json:"hideFileExt"`             // space separated list of file extensions to hide in UI
	ShowCopyPath            bool   `json:"showCopyPath"`            // show copy path button in the context menu
	DeleteAfterArchive      bool   `json:"deleteAfterArchive"`      // delete source files after successful creation/extraction of archives
	ViewMode                string `json:"viewMode"`                // view mode to use: eg. normal, list, grid, or compact
	GallerySize             int    `json:"gallerySize"`             // 0-9 - the size of the gallery thumbnails
}

// userDefaultsPreviewLegacy holds v1.x flat preview keys migrated on config load (YAML only).
type UserDefaultsPreviewLegacy struct {
	DisableHideSidebar bool `yaml:"disableHideSidebar,omitempty" json:"-"`
	DefaultMediaPlayer bool `yaml:"defaultMediaPlayer,omitempty" json:"-"`
	AutoplayMedia      bool `yaml:"autoplayMedia,omitempty" json:"-"`
}

// UserDefaultsPreview holds preview-related settings
type UserDefaultsPreview struct {
	Image                     *bool  `json:"image"`              // show thumbnails for image files
	Video                     *bool  `json:"video"`              // show thumbnails for video files
	Audio                     *bool  `json:"audio"`              // show thumbnails for audio files
	MotionVideoPreview        *bool  `json:"motionVideoPreview"` // show multiple frames for videos in thumbnail preview when hovering
	Office                    *bool  `json:"office"`             // show thumbnails for office files
	PopUp                     *bool  `json:"popup"`              // show larger popup preview when hovering over thumbnail
	DisablePreviewExt         string `json:"disablePreviewExt"`  // comma separated list of file extensions to disable preview for
	HighQuality               *bool  `json:"highQuality"`        // high quality preview thumbnails
	Folder                    *bool  `json:"folder"`             // show thumbnails for folders that have previewable contents
	Models                    *bool  `json:"models"`             // show live thumbnails for 3D models files
	UserDefaultsPreviewLegacy `yaml:",inline"`
}

// UserDefaultsFileViewer holds file viewer/editor settings
type UserDefaultsFileViewer struct {
	EditorQuickSave         bool   `json:"editorQuickSave"`         // show quick save button in editor
	AutoplayMedia           *bool  `json:"autoplayMedia"`           // autoplay media files in preview
	DisableViewingExt       string `json:"disableViewingExt"`       // comma separated list of file extensions to disable viewing for
	DisableOnlyOfficeExt    string `json:"disableOnlyOfficeExt"`    // list of file extensions to disable onlyoffice editor for
	PreferEditorForMarkdown bool   `json:"preferEditorForMarkdown"` // prefer editor first for markdown files instead of the Markdown Viewer
	DebugOffice             bool   `json:"debugOffice"`             // debug onlyoffice editor
	DefaultMediaPlayer      bool   `json:"defaultMediaPlayer"`      // disable the styled feature-rich media player for browser default
}

// UserDefaultsSearch holds search-related settings
type UserDefaultsSearch struct {
	DisableOptions bool `json:"disableOptions"` // disable the search options in the search bar
}

// UserDefaultsUI holds UI/appearance settings
type UserDefaultsUI struct {
	DarkMode    *bool  `json:"darkMode"`    // should dark mode be enabled
	ThemeColor  string `json:"themeColor"`  // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	CustomTheme string `json:"customTheme"` // Name of theme to use chosen from custom themes config
	Locale      string `json:"locale"`      // language to use: eg. de, en, or fr

}

// UserDefaultsAccount holds account/security settings
type UserDefaultsAccount struct {
	Permissions                UserDefaultsAccountPermissions `json:"permissions"`
	LockPassword               bool                           `json:"lockPassword"`               // disable the user from changing their password
	DisableSettings            bool                           `json:"disableSettings"`            // disable the user from viewing the settings page
	LoginMethod                string                         `json:"loginMethod,omitempty"`      // login method to use: eg. password, proxy, oidc
	DisableUpdateNotifications bool                           `json:"disableUpdateNotifications"` // disable update notifications banner for admin users
}

// UserDefaultsAccountPermissions holds global permission settings (not per-source file operations).
type UserDefaultsAccountPermissions struct {
	Api      bool `json:"api"`      // allow api access
	Admin    bool `json:"admin"`    // allow admin access
	Share    bool `json:"share"`    // allow sharing files
	Realtime bool `json:"realtime"` // allow realtime updates
}

// UserDefaultsLegacy holds v1.x flat userDefaults keys migrated on config load (YAML only).
type UserDefaultsLegacy struct {
	EditorQuickSave            bool                    `yaml:"editorQuickSave,omitempty" json:"-"`
	HideSidebarFileActions     bool                    `yaml:"hideSidebarFileActions,omitempty" json:"-"`
	DisableQuickToggles        bool                    `yaml:"disableQuickToggles,omitempty" json:"-"`
	DisableSearchOptions       bool                    `yaml:"disableSearchOptions,omitempty" json:"-"`
	StickySidebar              bool                    `yaml:"stickySidebar,omitempty" json:"-"`
	HideFilesInTree            bool                    `yaml:"hideFilesInTree,omitempty" json:"-"`
	DarkMode                   *bool                   `yaml:"darkMode,omitempty" json:"-"`
	Locale                     string                  `yaml:"locale,omitempty" json:"-"`
	ViewMode                   string                  `yaml:"viewMode,omitempty" json:"-"`
	SingleClick                bool                    `yaml:"singleClick,omitempty" json:"-"`
	ShowHidden                 bool                    `yaml:"showHidden,omitempty" json:"-"`
	HideFileExt                string                  `yaml:"hideFileExt,omitempty" json:"-"`
	DateFormat                 bool                    `yaml:"dateFormat,omitempty" json:"-"`
	GallerySize                int                     `yaml:"gallerySize,omitempty" json:"-"`
	ThemeColor                 string                  `yaml:"themeColor,omitempty" json:"-"`
	QuickDownload              bool                    `yaml:"quickDownload,omitempty" json:"-"`
	DisablePreviewExt          string                  `yaml:"disablePreviewExt,omitempty" json:"-"`
	DisableViewingExt          string                  `yaml:"disableViewingExt,omitempty" json:"-"`
	LockPassword               bool                    `yaml:"lockPassword,omitempty" json:"-"`
	DisableSettings            bool                    `yaml:"disableSettings,omitempty" json:"-"`
	Permissions                UserDefaultsPermissions `yaml:"permissions,omitempty" json:"-"`
	LoginMethod                string                  `yaml:"loginMethod,omitempty" json:"-"`
	DisableUpdateNotifications bool                    `yaml:"disableUpdateNotifications,omitempty" json:"-"`
	DeleteWithoutConfirming    bool                    `yaml:"deleteWithoutConfirming,omitempty" json:"-"`
	DeleteAfterArchive         bool                    `yaml:"deleteAfterArchive,omitempty" json:"-"`
	DisableOfficePreviewExt    string                  `yaml:"disableOfficePreviewExt,omitempty" json:"-"`
	DisableOnlyOfficeExt       string                  `yaml:"disableOnlyOfficeExt,omitempty" json:"-"`
	CustomTheme                string                  `yaml:"customTheme,omitempty" json:"-"`
	ShowSelectMultiple         bool                    `yaml:"showSelectMultiple,omitempty" json:"-"`
	ShowToolsInSidebar         *bool                   `yaml:"showToolsInSidebar,omitempty" json:"-"`
	DebugOffice                bool                    `yaml:"debugOffice,omitempty" json:"-"`
	PreferEditorForMarkdown    bool                    `yaml:"preferEditorForMarkdown,omitempty" json:"-"`
	ShowCopyPath               bool                    `yaml:"showCopyPath,omitempty" json:"-"`
}

// UserDefaults is a type that holds the default values for some fields on User.
type UserDefaults struct {
	// New organized structure
	Sidebar            UserDefaultsSidebar    `json:"sidebar,omitempty"`
	Listing            UserDefaultsListing    `json:"listing,omitempty"`
	Preview            UserDefaultsPreview    `json:"preview,omitempty"`
	FileViewer         UserDefaultsFileViewer `json:"fileViewer,omitempty"`
	Search             UserDefaultsSearch     `json:"search,omitempty"`
	UI                 UserDefaultsUI         `json:"ui,omitempty"`
	FileLoading        users.FileLoading      `json:"fileLoading,omitempty"`
	Account            UserDefaultsAccount    `json:"account,omitempty"`
	DefaultScopes      []users.BackendScope   `json:"-"`
	UserDefaultsLegacy `yaml:",inline"`
}
