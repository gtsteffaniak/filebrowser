package settings

import (
	"crypto/rand"

	"github.com/gtsteffaniak/filebrowser/backend/users"
)

const DefaultUsersHomeBasePath = "/users"

// AuthMethod describes an authentication method.
type AuthMethod string

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64) //nolint:gomnd
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GetSettingsConfig(nameType string, Value string) string {
	return nameType + Value
}

func AdminPerms() users.Permissions {
	return users.Permissions{
		Create:   true,
		Rename:   true,
		Modify:   true,
		Delete:   true,
		Share:    true,
		Download: true,
		Admin:    true,
		Api:      true,
	}
}

// Apply applies the default options to a user.
func ApplyUserDefaults(u users.User) users.User {
	u.StickySidebar = Config.UserDefaults.StickySidebar
	u.DisableSettings = Config.UserDefaults.DisableSettings
	u.DarkMode = Config.UserDefaults.DarkMode
	u.Scope = Config.UserDefaults.Scope
	u.Locale = Config.UserDefaults.Locale
	u.ViewMode = Config.UserDefaults.ViewMode
	u.SingleClick = Config.UserDefaults.SingleClick
	u.Perm = Config.UserDefaults.Perm
	u.Sorting = Config.UserDefaults.Sorting
	u.Commands = Config.UserDefaults.Commands
	u.HideDotfiles = Config.UserDefaults.HideDotfiles
	u.DateFormat = Config.UserDefaults.DateFormat
	return u
}
