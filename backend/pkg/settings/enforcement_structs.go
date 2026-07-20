package settings

// UserDefaultsEnforcement marks which default fields are enforced for all users (policy mask).
type UserDefaultsEnforcement struct {
	Sidebar     UserDefaultsSidebarEnforcement     `json:"sidebar,omitempty"`
	Listing     UserDefaultsListingEnforcement     `json:"listing,omitempty"`
	Preview     UserDefaultsPreviewEnforcement     `json:"preview,omitempty"`
	FileViewer  UserDefaultsFileViewerEnforcement  `json:"fileViewer,omitempty"`
	Search      UserDefaultsSearchEnforcement      `json:"search,omitempty"`
	UI          UserDefaultsUIEnforcement          `json:"ui,omitempty"`
	FileLoading UserDefaultsFileLoadingEnforcement `json:"fileLoading,omitempty"`
	Account     UserDefaultsAccountEnforcement     `json:"account,omitempty"`
}

type UserDefaultsSidebarEnforcement struct {
	DisableQuickToggles  bool `json:"disableQuickToggles,omitempty"`
	HideFileActions      bool `json:"hideFileActions,omitempty"`
	DisableHideOnPreview bool `json:"disableHideOnPreview,omitempty"`
	Sticky               bool `json:"sticky,omitempty"`
	HideFiles            bool `json:"hideFiles,omitempty"`
	ShowTools            bool `json:"showTools,omitempty"`
}

type UserDefaultsListingEnforcement struct {
	DeleteWithoutConfirming bool `json:"deleteWithoutConfirming,omitempty"`
	DateFormat              bool `json:"dateFormat,omitempty"`
	ShowHidden              bool `json:"showHidden,omitempty"`
	QuickDownload           bool `json:"quickDownload,omitempty"`
	ShowSelectMultiple      bool `json:"showSelectMultiple,omitempty"`
	SingleClick             bool `json:"singleClick,omitempty"`
	HideFileExt             bool `json:"hideFileExt,omitempty"`
	ShowCopyPath            bool `json:"showCopyPath,omitempty"`
	DeleteAfterArchive      bool `json:"deleteAfterArchive,omitempty"`
	ViewMode                bool `json:"viewMode,omitempty"`
	GallerySize             bool `json:"gallerySize,omitempty"`
}

type UserDefaultsPreviewEnforcement struct {
	Image              bool `json:"image,omitempty"`
	Video              bool `json:"video,omitempty"`
	Audio              bool `json:"audio,omitempty"`
	MotionVideoPreview bool `json:"motionVideoPreview,omitempty"`
	Office             bool `json:"office,omitempty"`
	PopUp              bool `json:"popup,omitempty"`
	DisablePreviewExt  bool `json:"disablePreviewExt,omitempty"`
	Folder             bool `json:"folder,omitempty"`
	Models             bool `json:"models,omitempty"`
}

type UserDefaultsFileViewerEnforcement struct {
	EditorQuickSave         bool `json:"editorQuickSave,omitempty"`
	AutoplayMedia           bool `json:"autoplayMedia,omitempty"`
	DisableViewingExt       bool `json:"disableViewingExt,omitempty"`
	DisableOnlyOfficeExt    bool `json:"disableOnlyOfficeExt,omitempty"`
	PreferEditorForMarkdown bool `json:"preferEditorForMarkdown,omitempty"`
	DebugOffice             bool `json:"debugOffice,omitempty"`
	DefaultMediaPlayer      bool `json:"defaultMediaPlayer,omitempty"`
}

type UserDefaultsSearchEnforcement struct {
	DisableOptions bool `json:"disableOptions,omitempty"`
}

type UserDefaultsUIEnforcement struct {
	DarkMode    bool `json:"darkMode,omitempty"`
	ThemeColor  bool `json:"themeColor,omitempty"`
	CustomTheme bool `json:"customTheme,omitempty"`
	Locale      bool `json:"locale,omitempty"`
}

type UserDefaultsFileLoadingEnforcement struct {
	MaxConcurrent     bool `json:"maxConcurrentUpload,omitempty"`
	UploadChunkSize   bool `json:"uploadChunkSizeMb,omitempty"`
	DownloadChunkSize bool `json:"downloadChunkSizeMb,omitempty"`
}

type UserDefaultsAccountEnforcement struct {
	LockPassword               bool                                  `json:"lockPassword,omitempty"`
	DisableSettings            bool                                  `json:"disableSettings,omitempty"`
	DisableUpdateNotifications bool                                  `json:"disableUpdateNotifications,omitempty"`
	Permissions                UserDefaultsAccountPermissionsEnforcement `json:"permissions,omitempty"`
}

type UserDefaultsAccountPermissionsEnforcement struct {
	Admin    bool `json:"admin,omitempty"`
	Api      bool `json:"api,omitempty"`
	Share    bool `json:"share,omitempty"`
	Realtime bool `json:"realtime,omitempty"`
}
