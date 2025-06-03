package users

import (
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
)

const ()

type LoginMethod string

const (
	LoginMethodPassword LoginMethod = "password"
	LoginMethodProxy    LoginMethod = "proxy"
	LoginMethodOidc     LoginMethod = "oidc"
)

type AuthToken struct {
	Key                  string      `json:"key"`
	Name                 string      `json:"name"`
	Created              int64       `json:"createdAt"`
	Expires              int64       `json:"expiresAt"`
	BelongsTo            uint        `json:"belongsTo"`
	Permissions          Permissions `json:"Permissions"`
	jwt.RegisteredClaims `json:"-"`
}

type Permissions struct {
	Api      bool `json:"api"`
	Admin    bool `json:"admin"`
	Modify   bool `json:"modify"`
	Share    bool `json:"share"`
	Realtime bool `json:"realtime"`
}

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

type Preview struct {
	HighQuality        bool `json:"highQuality"`        // generate high quality preview images
	Image              bool `json:"image"`              // show real image as icon instead of generic photo icon
	Video              bool `json:"video"`              // show preview image for video files
	MotionVideoPreview bool `json:"motionVideoPreview"` // show multiple frames for videos in preview when hovering
	Office             bool `json:"office"`             // show preview image for office files
	PopUp              bool `json:"popup"`              // show larger popup preview when hovering
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
	Perm Permissions `json:"perm,omitzero"`
}

type SourceScope struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// json tags must match variable name with smaller case first letter
type NonAdminEditable struct {
	Preview                 Preview `json:"preview"`
	StickySidebar           bool    `json:"stickySidebar"` // keep sidebar open when navigating
	DarkMode                bool    `json:"darkMode"`      // should dark mode be enabled
	Password                string  `json:"password,omitempty"`
	Locale                  string  `json:"locale"`      // language to use: eg. de, en, or fr
	ViewMode                string  `json:"viewMode"`    // view mode to use: eg. normal, list, grid, or compact
	SingleClick             bool    `json:"singleClick"` // open directory on single click, also enables middle click to open in new tab
	Sorting                 Sorting `json:"sorting"`
	ShowHidden              bool    `json:"showHidden"`              // show hidden files in the UI. On windows this includes files starting with a dot and windows hidden files
	DateFormat              bool    `json:"dateFormat"`              // when false, the date is relative, when true, the date is an exact timestamp
	GallerySize             int     `json:"gallerySize"`             // 0-9 - the size of the gallery thumbnails
	ThemeColor              string  `json:"themeColor"`              // theme color to use: eg. #ff0000, or var(--red), var(--purple), etc
	QuickDownload           bool    `json:"quickDownload"`           // show icon to download in one click
	DisableOnlyOfficeExt    string  `json:"disableOnlyOfficeExt"`    // comma separated list of file extensions to disable onlyoffice preview for
	DisableOfficePreviewExt string  `json:"disableOfficePreviewExt"` // comma separated list of file extensions to disable office preview for
}

var PublicUser = User{
	NonAdminEditable: NonAdminEditable{
		Password: "publicUser", // temp user not registered
		ViewMode: "normal",
	},
	Username:     "publicUser", // temp user not registered
	LockPassword: true,
	Permissions:  Permissions{},
}

func CleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.Replace(s, "..", "", -1)
	return s
}
