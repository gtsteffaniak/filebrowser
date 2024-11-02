package users

import (
	"regexp"
)

type Permissions struct {
	Api      bool `json:"api"`
	Admin    bool `json:"admin"`
	Execute  bool `json:"execute"`
	Create   bool `json:"create"`
	Rename   bool `json:"rename"`
	Modify   bool `json:"modify"`
	Delete   bool `json:"delete"`
	Share    bool `json:"share"`
	Download bool `json:"download"`
}

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

// User describes a user.
type User struct {
	StickySidebar   bool        `json:"stickySidebar"`
	DarkMode        bool        `json:"darkMode"`
	DisableSettings bool        `json:"disableSettings"`
	ID              uint        `storm:"id,increment" json:"id"`
	Username        string      `storm:"unique" json:"username"`
	Password        string      `json:"password"`
	Scope           string      `json:"scope"`
	Locale          string      `json:"locale"`
	LockPassword    bool        `json:"lockPassword"`
	ViewMode        string      `json:"viewMode"`
	SingleClick     bool        `json:"singleClick"`
	Sorting         Sorting     `json:"sorting"`
	Perm            Permissions `json:"perm"`
	Commands        []string    `json:"commands"`
	Rules           []Rule      `json:"rules"`
	HideDotfiles    bool        `json:"hideDotfiles"`
	DateFormat      bool        `json:"dateFormat"`
	GallerySize     int         `json:"gallerySize"`
}

var PublicUser = User{
	Username:     "publicUser", // temp user not registered
	Password:     "publicUser", // temp user not registered
	Scope:        "./",
	ViewMode:     "normal",
	LockPassword: true,
	Perm: Permissions{
		Create:   false,
		Rename:   false,
		Modify:   false,
		Delete:   false,
		Share:    false,
		Download: true,
		Admin:    false,
		Api:      false,
	},
}

// GetRules implements rules.Provider.
func (u *User) GetRules() []Rule {
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
