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
		Modify: true,
		Share:  true,
		Admin:  true,
		Api:    true,
	}
}

// Apply applies the default options to a user.
func ApplyUserDefaults(u *users.User) {
	u.StickySidebar = Config.UserDefaults.StickySidebar
	u.DisableSettings = Config.UserDefaults.DisableSettings
	u.DarkMode = Config.UserDefaults.DarkMode
	u.Locale = Config.UserDefaults.Locale
	u.ViewMode = Config.UserDefaults.ViewMode
	u.SingleClick = Config.UserDefaults.SingleClick
	u.Perm = Config.UserDefaults.Perm
	u.ShowHidden = Config.UserDefaults.ShowHidden
	u.DateFormat = Config.UserDefaults.DateFormat
	if len(u.Scopes) == 0 {
		u.Scopes = []users.SourceScope{
			{
				Scope: Config.Server.DefaultSource.Config.DefaultUserScope,
				Name:  Config.Server.DefaultSource.Path,
			},
		}
	}

}
