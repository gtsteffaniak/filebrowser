package settings

import (
	"encoding/json"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ProfileFromUser builds nested profile from runtime user fields (flat + Preview).
func ProfileFromUser(u *users.User) UserProfile {
	if u == nil {
		return UserProfile{}
	}
	return UserProfile{
		Sidebar: UserDefaultsSidebar{
			DisableQuickToggles:  u.DisableQuickToggles,
			HideFileActions:      u.HideSidebarFileActions,
			DisableHideOnPreview: u.Preview.DisableHideSidebar,
			Sticky:               u.StickySidebar,
			HideFiles:            u.HideFilesInTree,
			ShowTools:            boolPtr(u.ShowToolsInSidebar),
		},
		Listing: UserDefaultsListing{
			DeleteWithoutConfirming: u.DeleteWithoutConfirming,
			DateFormat:              u.DateFormat,
			ShowHidden:              u.ShowHidden,
			QuickDownload:           u.QuickDownload,
			ShowSelectMultiple:      u.ShowSelectMultiple,
			SingleClick:             u.SingleClick,
			HideFileExt:             u.HideFileExt,
			ShowCopyPath:            u.ShowCopyPath,
			DeleteAfterArchive:      u.DeleteAfterArchive,
			ViewMode:                u.ViewMode,
			GallerySize:             u.GallerySize,
		},
		Preview: UserDefaultsPreview{
			Image:              boolPtr(u.Preview.Image),
			Video:              boolPtr(u.Preview.Video),
			Audio:              boolPtr(u.Preview.Audio),
			MotionVideoPreview: boolPtr(u.Preview.MotionVideoPreview),
			Office:             boolPtr(u.Preview.Office),
			PopUp:              boolPtr(u.Preview.PopUp),
			Folder:             boolPtr(u.Preview.Folder),
			Models:             boolPtr(u.Preview.Models),
			DisablePreviewExt:  u.DisablePreviewExt,
		},
		FileViewer: UserDefaultsFileViewer{
			EditorQuickSave:         u.EditorQuickSave,
			AutoplayMedia:           boolPtr(u.Preview.AutoplayMedia),
			DefaultMediaPlayer:      u.Preview.DefaultMediaPlayer,
			DisableViewingExt:       u.DisableViewingExt,
			DisableOnlyOfficeExt:    u.DisableOnlyOfficeExt,
			PreferEditorForMarkdown: u.PreferEditorForMarkdown,
			DebugOffice:             u.DebugOffice,
		},
		Search: UserDefaultsSearch{
			DisableOptions: u.DisableSearchOptions,
		},
		UI: UserDefaultsUI{
			DarkMode:    boolPtr(u.DarkMode),
			ThemeColor:  u.ThemeColor,
			CustomTheme: u.CustomTheme,
			Locale:      u.Locale,
		},
		FileLoading: u.FileLoading,
		Account: UserDefaultsAccount{
			LockPassword:               u.LockPassword,
			DisableSettings:            u.DisableSettings,
			DisableUpdateNotifications: u.DisableUpdateNotifications,
			Permissions: UserDefaultsAccountPermissions{
				Api:      u.Permissions.Api,
				Admin:    u.Permissions.Admin,
				Share:    u.Permissions.Share,
				Realtime: u.Permissions.Realtime,
			},
		},
	}
}

// ExpandProfileIntoUser copies nested profile values onto u (flat API/runtime fields).
func ExpandProfileIntoUser(u *users.User, p UserProfile) {
	if u == nil {
		return
	}
	u.DisableSettings = p.Account.DisableSettings
	u.LockPassword = p.Account.LockPassword
	u.DisableUpdateNotifications = p.Account.DisableUpdateNotifications
	u.Permissions.Api = p.Account.Permissions.Api
	u.Permissions.Admin = p.Account.Permissions.Admin
	u.Permissions.Share = p.Account.Permissions.Share
	u.Permissions.Realtime = p.Account.Permissions.Realtime

	u.DisableQuickToggles = p.Sidebar.DisableQuickToggles
	u.HideSidebarFileActions = p.Sidebar.HideFileActions
	u.StickySidebar = p.Sidebar.Sticky
	u.HideFilesInTree = p.Sidebar.HideFiles
	u.ShowToolsInSidebar = boolValueOrDefault(p.Sidebar.ShowTools, true)

	u.DeleteWithoutConfirming = p.Listing.DeleteWithoutConfirming
	u.DateFormat = p.Listing.DateFormat
	u.ShowHidden = p.Listing.ShowHidden
	u.QuickDownload = p.Listing.QuickDownload
	u.ShowSelectMultiple = p.Listing.ShowSelectMultiple
	u.SingleClick = p.Listing.SingleClick
	u.HideFileExt = p.Listing.HideFileExt
	u.ShowCopyPath = p.Listing.ShowCopyPath
	u.DeleteAfterArchive = p.Listing.DeleteAfterArchive
	u.ViewMode = p.Listing.ViewMode
	u.GallerySize = p.Listing.GallerySize

	u.Preview.DisableHideSidebar = p.Sidebar.DisableHideOnPreview
	u.Preview.Image = boolValueOrDefault(p.Preview.Image, true)
	u.Preview.Video = boolValueOrDefault(p.Preview.Video, true)
	u.Preview.Audio = boolValueOrDefault(p.Preview.Audio, true)
	u.Preview.MotionVideoPreview = boolValueOrDefault(p.Preview.MotionVideoPreview, true)
	u.Preview.Office = boolValueOrDefault(p.Preview.Office, true)
	u.Preview.PopUp = boolValueOrDefault(p.Preview.PopUp, true)
	u.Preview.Folder = boolValueOrDefault(p.Preview.Folder, true)
	u.Preview.Models = boolValueOrDefault(p.Preview.Models, true)
	u.DisablePreviewExt = p.Preview.DisablePreviewExt

	u.EditorQuickSave = p.FileViewer.EditorQuickSave
	u.Preview.AutoplayMedia = boolValueOrDefault(p.FileViewer.AutoplayMedia, true)
	u.Preview.DefaultMediaPlayer = p.FileViewer.DefaultMediaPlayer
	u.DisableViewingExt = p.FileViewer.DisableViewingExt
	u.DisableOnlyOfficeExt = p.FileViewer.DisableOnlyOfficeExt
	u.DisableOfficePreviewExt = p.FileViewer.DisableOnlyOfficeExt
	u.PreferEditorForMarkdown = p.FileViewer.PreferEditorForMarkdown
	u.DebugOffice = p.FileViewer.DebugOffice

	u.DisableSearchOptions = p.Search.DisableOptions

	u.DarkMode = boolValueOrDefault(p.UI.DarkMode, true)
	u.ThemeColor = p.UI.ThemeColor
	u.CustomTheme = p.UI.CustomTheme
	u.Locale = p.UI.Locale

	u.FileLoading = p.FileLoading
}

// ProfileFromLegacyUser converts a Bolt/flat user into nested profile (one-time migration).
func ProfileFromLegacyUser(u *users.User) UserProfile {
	return ProfileFromUser(u)
}

// ApplyProfileToUser unmarshals profile JSON and expands onto u.
func ApplyProfileToUser(u *users.User, profileJSON []byte) error {
	if u == nil || len(profileJSON) == 0 {
		return nil
	}
	var p UserProfile
	if err := json.Unmarshal(profileJSON, &p); err != nil {
		return err
	}
	ExpandProfileIntoUser(u, p)
	return nil
}

// ProfileJSONFromUser returns JSON for SQLite user_data.profile.
func ProfileJSONFromUser(u *users.User) ([]byte, error) {
	return json.Marshal(ProfileFromUser(u))
}
