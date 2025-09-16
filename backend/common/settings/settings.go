//go:generate go run ./tools/yaml.go -input=common/settings/settings.go -output=config.generated.yaml
package settings

import (
	"crypto/rand"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

const DefaultUsersHomeBasePath = "/users"

// AuthMethod describes an authentication method.
type AuthMethod string

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
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
	u.Permissions = Config.UserDefaults.Permissions
	u.Preview = Config.UserDefaults.Preview
	u.ShowHidden = Config.UserDefaults.ShowHidden
	u.DateFormat = Config.UserDefaults.DateFormat
	u.DisableViewingExt = Config.UserDefaults.DisableViewingExt
	u.ThemeColor = Config.UserDefaults.ThemeColor
	u.GallerySize = Config.UserDefaults.GallerySize
	u.QuickDownload = Config.UserDefaults.QuickDownload
	u.LockPassword = Config.UserDefaults.LockPassword
	if len(u.Scopes) == 0 {
		for _, source := range Config.Server.Sources {
			if source.Config.DefaultEnabled {
				u.Scopes = append(u.Scopes, users.SourceScope{
					Name:  source.Path, // backend name is path
					Scope: source.Config.DefaultUserScope,
				})
			}
		}
	}
}
