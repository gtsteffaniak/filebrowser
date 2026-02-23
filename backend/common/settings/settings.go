//go:generate go run ./tools/yaml.go -input=common/settings/settings.go -output=config.generated.yaml
package settings

import (
	"crypto/rand"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// boolValueOrDefault returns the value of a bool pointer, or the default if nil
func boolValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// ConvertPermissionsToUsers converts UserDefaultsPermissions to users.Permissions
func ConvertPermissionsToUsers(p UserDefaultsPermissions) users.Permissions {
	return users.Permissions{
		Api:      p.Api,
		Admin:    p.Admin,
		Modify:   p.Modify,
		Share:    p.Share,
		Realtime: p.Realtime,
		Delete:   p.Delete,
		Create:   p.Create,
		Download: boolValueOrDefault(p.Download, true),
	}
}

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

	// Handle DarkMode with default - dereference pointer from config
	u.DarkMode = boolValueOrDefault(Config.UserDefaults.DarkMode, true)

	u.Locale = Config.UserDefaults.Locale
	u.ViewMode = Config.UserDefaults.ViewMode
	u.SingleClick = Config.UserDefaults.SingleClick

	// Handle Permissions - convert from pointer-based defaults to regular bools
	u.Permissions.Api = Config.UserDefaults.Permissions.Api
	u.Permissions.Admin = Config.UserDefaults.Permissions.Admin
	u.Permissions.Modify = Config.UserDefaults.Permissions.Modify
	u.Permissions.Share = Config.UserDefaults.Permissions.Share
	u.Permissions.Realtime = Config.UserDefaults.Permissions.Realtime
	u.Permissions.Delete = Config.UserDefaults.Permissions.Delete
	u.Permissions.Create = Config.UserDefaults.Permissions.Create
	u.Permissions.Download = boolValueOrDefault(Config.UserDefaults.Permissions.Download, true)

	// Handle Preview - convert from pointer-based defaults to regular bools
	u.Preview.DisableHideSidebar = Config.UserDefaults.Preview.DisableHideSidebar
	u.Preview.Image = boolValueOrDefault(Config.UserDefaults.Preview.Image, true)
	u.Preview.Video = boolValueOrDefault(Config.UserDefaults.Preview.Video, true)
	u.Preview.MotionVideoPreview = boolValueOrDefault(Config.UserDefaults.Preview.MotionVideoPreview, true)
	u.Preview.Office = boolValueOrDefault(Config.UserDefaults.Preview.Office, true)
	u.Preview.PopUp = boolValueOrDefault(Config.UserDefaults.Preview.PopUp, true)
	u.Preview.AutoplayMedia = boolValueOrDefault(Config.UserDefaults.Preview.AutoplayMedia, true)
	u.Preview.DefaultMediaPlayer = Config.UserDefaults.Preview.DefaultMediaPlayer
	u.Preview.Folder = boolValueOrDefault(Config.UserDefaults.Preview.Folder, true)

	u.ShowHidden = Config.UserDefaults.ShowHidden
	u.DateFormat = Config.UserDefaults.DateFormat
	u.DisableViewingExt = Config.UserDefaults.DisableViewingExt
	u.ThemeColor = Config.UserDefaults.ThemeColor
	u.GallerySize = Config.UserDefaults.GallerySize
	u.QuickDownload = Config.UserDefaults.QuickDownload
	u.LockPassword = Config.UserDefaults.LockPassword
	u.DisableOnlyOfficeExt = Config.UserDefaults.DisableOnlyOfficeExt
	u.FileLoading = Config.UserDefaults.FileLoading
	u.DisableOfficePreviewExt = Config.UserDefaults.DisableOfficePreviewExt
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
