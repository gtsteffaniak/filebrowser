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
func ConvertPermissionsToUsers(p UserDefaultsAccountPermissions) users.Permissions {
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
		Modify:   true,
		Share:    true,
		Admin:    true,
		Api:      true,
		Download: true,
		Delete:   true,
		Create:   true,
		Realtime: false,
	}
}

// Apply applies the default options to a user.
// Keep this in sync with [UserDefaults]: every user-facing field there should be copied
// here (except DefaultScopes, which is wired through source config rather than this helper).
func ApplyUserDefaults(u *users.User) {
	d := Config.UserDefaults

	// Account settings
	u.DisableSettings = d.Account.DisableSettings
	u.LockPassword = d.Account.LockPassword
	u.DisableUpdateNotifications = d.Account.DisableUpdateNotifications

	// Sidebar settings
	u.DisableQuickToggles = d.Sidebar.DisableQuickToggles
	u.HideSidebarFileActions = d.Sidebar.HideFileActions
	u.StickySidebar = d.Sidebar.Sticky
	u.HideFilesInTree = d.Sidebar.HideFiles
	u.ShowToolsInSidebar = boolValueOrDefault(d.Sidebar.ShowTools, true)

	// Listing settings
	u.DeleteWithoutConfirming = d.Listing.DeleteWithoutConfirming
	u.DateFormat = d.Listing.DateFormat
	u.ShowHidden = d.Listing.ShowHidden
	u.QuickDownload = d.Listing.QuickDownload
	u.ShowSelectMultiple = d.Listing.ShowSelectMultiple
	u.SingleClick = d.Listing.SingleClick
	u.HideFileExt = d.Listing.HideFileExt
	u.ShowCopyPath = d.Listing.ShowCopyPath
	u.DeleteAfterArchive = d.Listing.DeleteAfterArchive
	u.ViewMode = d.Listing.ViewMode
	u.GallerySize = d.Listing.GallerySize

	// Preview settings
	u.Preview.DisableHideSidebar = d.Sidebar.DisableHideOnPreview
	u.Preview.Image = boolValueOrDefault(d.Preview.Image, true)
	u.Preview.Video = boolValueOrDefault(d.Preview.Video, true)
	u.Preview.Audio = boolValueOrDefault(d.Preview.Audio, true)
	u.Preview.MotionVideoPreview = boolValueOrDefault(d.Preview.MotionVideoPreview, true)
	u.Preview.Office = boolValueOrDefault(d.Preview.Office, true)
	u.Preview.PopUp = boolValueOrDefault(d.Preview.PopUp, true)
	u.Preview.Folder = boolValueOrDefault(d.Preview.Folder, true)
	u.Preview.Models = boolValueOrDefault(d.Preview.Models, true)
	u.DisablePreviewExt = d.Preview.DisablePreviewExt

	// FileViewer settings
	u.EditorQuickSave = d.FileViewer.EditorQuickSave
	u.Preview.AutoplayMedia = boolValueOrDefault(d.FileViewer.AutoplayMedia, true)
	u.Preview.DefaultMediaPlayer = d.FileViewer.DefaultMediaPlayer
	u.DisableViewingExt = d.FileViewer.DisableViewingExt
	u.DisableOnlyOfficeExt = d.FileViewer.DisableOnlyOfficeExt
	u.DisableOfficePreviewExt = d.FileViewer.DisableOnlyOfficeExt // deprecated field, map to same source
	u.PreferEditorForMarkdown = d.FileViewer.PreferEditorForMarkdown
	u.DebugOffice = d.FileViewer.DebugOffice

	// Search settings
	u.DisableSearchOptions = d.Search.DisableOptions

	// UI settings
	u.DarkMode = boolValueOrDefault(d.UI.DarkMode, true)
	u.ThemeColor = d.UI.ThemeColor
	u.CustomTheme = d.UI.CustomTheme
	u.Locale = d.UI.Locale

	// FileLoading settings
	u.FileLoading = d.FileLoading

	// Permissions
	u.Permissions.Api = d.Account.Permissions.Api
	u.Permissions.Admin = d.Account.Permissions.Admin
	u.Permissions.Modify = d.Account.Permissions.Modify
	u.Permissions.Share = d.Account.Permissions.Share
	u.Permissions.Realtime = d.Account.Permissions.Realtime
	u.Permissions.Delete = d.Account.Permissions.Delete
	u.Permissions.Create = d.Account.Permissions.Create
	u.Permissions.Download = boolValueOrDefault(d.Account.Permissions.Download, true)

	if u.LoginMethod == "" && d.Account.LoginMethod != "" {
		u.LoginMethod = users.LoginMethod(d.Account.LoginMethod)
	}

	if len(u.Scopes) == 0 && u.Username != "anonymous" {
		for _, source := range Config.Server.Sources {
			if source.Config.DefaultEnabled {
				u.Scopes = append(u.Scopes, users.SourceScope{
					Name:  source.Path, // backend name is path
					Scope: source.Config.DefaultUserScope,
				})
				u.SidebarLinks = append(u.SidebarLinks, users.SidebarLink{
					Name:       source.Name,
					Category:   "source",
					Target:     "/",
					Icon:       "",
					SourceName: source.Path,
				})
			}
		}
	}

	u.Version = users.CurrentUserMigrationVersion
}
