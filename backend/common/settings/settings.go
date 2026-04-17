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
// Keep this in sync with [UserDefaults]: every user-facing field there should be copied
// here (except DefaultScopes, which is wired through source config rather than this helper).
func ApplyUserDefaults(u *users.User) {
	d := Config.UserDefaults

	u.DisableSettings = d.DisableSettings
	u.LockPassword = d.LockPassword

	u.EditorQuickSave = d.EditorQuickSave
	u.HideSidebarFileActions = d.HideSidebarFileActions
	u.DisableQuickToggles = d.DisableQuickToggles
	u.DisableSearchOptions = d.DisableSearchOptions
	u.StickySidebar = d.StickySidebar
	u.HideFilesInTree = d.HideFilesInTree
	u.DarkMode = boolValueOrDefault(d.DarkMode, true)
	u.Locale = d.Locale
	u.ViewMode = d.ViewMode
	u.SingleClick = d.SingleClick
	u.ShowHidden = d.ShowHidden
	u.DateFormat = d.DateFormat
	u.GallerySize = d.GallerySize
	u.ThemeColor = d.ThemeColor
	u.QuickDownload = d.QuickDownload
	u.DisablePreviewExt = d.DisablePreviewExt
	u.DisableViewingExt = d.DisableViewingExt
	u.DisableUpdateNotifications = d.DisableUpdateNotifications
	u.DisableOfficePreviewExt = d.DisableOfficePreviewExt
	u.DisableOnlyOfficeExt = d.DisableOnlyOfficeExt
	u.CustomTheme = d.CustomTheme
	u.ShowSelectMultiple = d.ShowSelectMultiple
	u.DebugOffice = d.DebugOffice
	u.DeleteWithoutConfirming = d.DeleteWithoutConfirming
	u.DeleteAfterArchive = d.DeleteAfterArchive
	u.PreferEditorForMarkdown = d.PreferEditorForMarkdown
	u.FileLoading = d.FileLoading

	u.Permissions.Api = d.Permissions.Api
	u.Permissions.Admin = d.Permissions.Admin
	u.Permissions.Modify = d.Permissions.Modify
	u.Permissions.Share = d.Permissions.Share
	u.Permissions.Realtime = d.Permissions.Realtime
	u.Permissions.Delete = d.Permissions.Delete
	u.Permissions.Create = d.Permissions.Create
	u.Permissions.Download = boolValueOrDefault(d.Permissions.Download, true)

	u.Preview.DisableHideSidebar = d.Preview.DisableHideSidebar
	u.Preview.Image = boolValueOrDefault(d.Preview.Image, true)
	u.Preview.Video = boolValueOrDefault(d.Preview.Video, true)
	u.Preview.Audio = boolValueOrDefault(d.Preview.Audio, true)
	u.Preview.MotionVideoPreview = boolValueOrDefault(d.Preview.MotionVideoPreview, true)
	u.Preview.Office = boolValueOrDefault(d.Preview.Office, true)
	u.Preview.PopUp = boolValueOrDefault(d.Preview.PopUp, true)
	u.Preview.AutoplayMedia = boolValueOrDefault(d.Preview.AutoplayMedia, true)
	u.Preview.DefaultMediaPlayer = d.Preview.DefaultMediaPlayer
	u.Preview.Folder = boolValueOrDefault(d.Preview.Folder, true)
	u.Preview.Models = boolValueOrDefault(d.Preview.Models, true)

	if u.LoginMethod == "" && d.LoginMethod != "" {
		u.LoginMethod = users.LoginMethod(d.LoginMethod)
	}

	if len(u.Scopes) == 0 && u.Username != "anonymous" {
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
