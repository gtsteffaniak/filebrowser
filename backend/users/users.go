package users

import (
	"regexp"

	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/settings"
)

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

// User describes a user.
type User struct {
	StickySidebar   bool                 `json:"stickySidebar"`
	DarkMode        bool                 `json:"darkMode"`
	DisableSettings bool                 `json:"disableSettings"`
	ID              uint                 `storm:"id,increment" json:"id"`
	Username        string               `storm:"unique" json:"username"`
	Password        string               `json:"password"`
	Scope           string               `json:"scope"`
	Locale          string               `json:"locale"`
	LockPassword    bool                 `json:"lockPassword"`
	ViewMode        string               `json:"viewMode"`
	SingleClick     bool                 `json:"singleClick"`
	Perm            settings.Permissions `json:"perm"`
	Commands        []string             `json:"commands"`
	Sorting         Sorting              `json:"sorting"`
	Rules           []rules.Rule         `json:"rules"`
	HideDotfiles    bool                 `json:"hideDotfiles"`
	DateFormat      bool                 `json:"dateFormat"`
	GallerySize     int                  `json:"gallerySize"`
}

var PublicUser = User{
	Username:     "publicUser", // temp user not registered
	Password:     "publicUser", // temp user not registered
	Scope:        "./",
	ViewMode:     "normal",
	LockPassword: true,
	Perm: settings.Permissions{
		Create:   false,
		Rename:   false,
		Modify:   false,
		Delete:   false,
		Share:    true,
		Download: true,
		Admin:    false,
	},
}

// GetRules implements rules.Provider.
func (u *User) GetRules() []rules.Rule {
	return u.Rules
}

// CanExecute checks if an user can execute a specific command.
func (u *User) CanExecute(command string) bool {
	if !u.Perm.Execute {
		return false
	}

	for _, cmd := range u.Commands {
		if regexp.MustCompile(cmd).MatchString(command) {
			return true
		}
	}

	return false
}

// Apply applies the default options to a user.
func ApplyDefaults(u User) User {
	u.StickySidebar = settings.Config.UserDefaults.StickySidebar
	u.DisableSettings = settings.Config.UserDefaults.DisableSettings
	u.DarkMode = settings.Config.UserDefaults.DarkMode
	u.Scope = settings.Config.UserDefaults.Scope
	u.Locale = settings.Config.UserDefaults.Locale
	u.ViewMode = settings.Config.UserDefaults.ViewMode
	u.SingleClick = settings.Config.UserDefaults.SingleClick
	u.Perm = settings.Config.UserDefaults.Perm
	u.Sorting = settings.Config.UserDefaults.Sorting
	u.Commands = settings.Config.UserDefaults.Commands
	u.HideDotfiles = settings.Config.UserDefaults.HideDotfiles
	u.DateFormat = settings.Config.UserDefaults.DateFormat
	return u
}
