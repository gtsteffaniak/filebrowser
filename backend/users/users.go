package users

import (
	jwt "github.com/golang-jwt/jwt/v4"
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
	Api    bool `json:"api"`
	Admin  bool `json:"admin"`
	Modify bool `json:"modify"`
	Share  bool `json:"share"`
}

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

// User describes a user.
type User struct {
	NonAdminEditable
	StickySidebar   bool          `json:"stickySidebar"`
	DisableSettings bool          `json:"disableSettings"`
	ID              uint          `storm:"id,increment" json:"id"`
	Username        string        `storm:"unique" json:"username"`
	Scopes          []SourceScope `json:"scopes"`
	Scope           string        `json:"scope"`
	LockPassword    bool          `json:"lockPassword"`
	Perm            Permissions   `json:"perm"`
}

type SourceScope struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type NonAdminEditable struct {
	DarkMode             bool                 `json:"darkMode"`
	Password             string               `json:"password,omitempty"`
	Locale               string               `json:"locale"`
	ViewMode             string               `json:"viewMode"`
	SingleClick          bool                 `json:"singleClick"`
	Sorting              Sorting              `json:"sorting"`
	ApiKeys              map[string]AuthToken `json:"apiKeys,omitempty"`
	ShowHidden           bool                 `json:"showHidden"`
	DateFormat           bool                 `json:"dateFormat"`
	GallerySize          int                  `json:"gallerySize"`
	ThemeColor           string               `json:"themeColor"`
	QuickDownload        bool                 `json:"quickDownload"`
	DisableOnlyOfficeExt string               `json:"disableOnlyOfficeExt"`
}

var PublicUser = User{
	NonAdminEditable: NonAdminEditable{
		Password: "publicUser", // temp user not registered
		ViewMode: "normal",
	},
	Username:     "publicUser", // temp user not registered
	LockPassword: true,
	Perm: Permissions{
		Modify: false,
		Share:  false,
		Admin:  false,
		Api:    false,
	},
}
