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

type PreviewOptions struct {
	HighQuality        bool `json:"highQuality"`
	Image              bool `json:"image"`
	Video              bool `json:"video"`
	MotionVideoPreview bool `json:"motionVideoPreview"`
	LivePhotoPreview   bool `json:"livePhotoPreview"`
	Office             bool `json:"office"`
	PopUp              bool `json:"popup"`
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
	LoginMethod     LoginMethod          `json:"loginMethod"`
	// legacy for migration purposes... og filebrowser has perm attribute
	Perm Permissions `json:"perm,omitzero"`
}

type SourceScope struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type NonAdminEditable struct {
	Preview              PreviewOptions `json:"preview"`
	StickySidebar        bool           `json:"stickySidebar"`
	DarkMode             bool           `json:"darkMode"`
	Password             string         `json:"password,omitempty"`
	Locale               string         `json:"locale"`
	ViewMode             string         `json:"viewMode"`
	SingleClick          bool           `json:"singleClick"`
	Sorting              Sorting        `json:"sorting"`
	ShowHidden           bool           `json:"showHidden"`
	DateFormat           bool           `json:"dateFormat"`
	GallerySize          int            `json:"gallerySize"`
	ThemeColor           string         `json:"themeColor"`
	QuickDownload        bool           `json:"quickDownload"`
	DisableOnlyOfficeExt string         `json:"disableOnlyOfficeExt"`
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
