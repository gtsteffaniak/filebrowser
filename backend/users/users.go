package users

import (
	"path/filepath"
	"regexp"

	"github.com/spf13/afero"

	"github.com/gtsteffaniak/filebrowser/rules"
)

type Permissions struct {
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
	DarkMode        bool         `json:"darkMode"`
	DisableSettings bool         `json:"disableSettings"`
	ID              uint         `storm:"id,increment" json:"id"`
	Username        string       `storm:"unique" json:"username"`
	Password        string       `json:"password"`
	Scope           string       `json:"scope"`
	Locale          string       `json:"locale"`
	LockPassword    bool         `json:"lockPassword"`
	ViewMode        string       `json:"viewMode"`
	SingleClick     bool         `json:"singleClick"`
	Perm            Permissions  `json:"perm"`
	Commands        []string     `json:"commands"`
	Sorting         Sorting      `json:"sorting"`
	Fs              afero.Fs     `json:"-" yaml:"-"`
	Rules           []rules.Rule `json:"rules"`
	HideDotfiles    bool         `json:"hideDotfiles"`
	DateFormat      bool         `json:"dateFormat"`
}

var PublicUser = User{
	Username:     "publicUser", // temp user not registered
	Password:     "publicUser", // temp user not registered
	Scope:        "./",
	ViewMode:     "normal",
	LockPassword: true,
	Fs:           afero.NewMemMapFs(),
	Perm: Permissions{
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

// Clean cleans up a user and verifies if all its fields
// are alright to be saved.
//
//nolint:gocyclo
func (u *User) Clean(baseScope string) error {

	if u.Fs == nil {
		scope := u.Scope
		scope = filepath.Join(baseScope, filepath.Join("/", scope)) //nolint:gocritic
		u.Fs = afero.NewBasePathFs(afero.NewOsFs(), scope)
	}

	return nil
}

// FullPath gets the full path for a user's relative path.
func (u *User) FullPath(path string) string {
	return afero.FullBaseFsPath(u.Fs.(*afero.BasePathFs), path)
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
