package settings

import (
	"encoding/json"
	"fmt"
	"strings"

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
			PromptRightCloseButton:  u.PromptRightCloseButton,
			DateFormat:              u.DateFormat,
			ShowHidden:              u.ShowHidden,
			QuickDownload:           u.QuickDownload,
			ShowSelectMultiple:      u.ShowSelectMultiple,
			SingleClick:             u.SingleClick,
			HideFileExt:             u.HideFileExt,
			ShowCopyPath:            u.ShowCopyPath,
			ShowTypeColumn:          u.ShowTypeColumn,
			ShowCreationDateColumn:  u.ShowCreationDateColumn,
			DeleteAfterArchive:      u.DeleteAfterArchive,
			ViewMode:                u.ViewMode,
			GallerySize:             u.GallerySize,
			NewFileTemplate:         u.NewFileTemplate,
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
			AutoplayMedia:           boolPtr(u.Preview.AutoplayMedia),
			DisableViewingExt:       u.DisableViewingExt,
			DisableOnlyOfficeExt:    u.DisableOnlyOfficeExt,
			PreferEditorForMarkdown: u.PreferEditorForMarkdown,
			PreferEditorForHtml:     u.PreferEditorForHtml,
			DebugOffice:             u.DebugOffice,
		},
		Search: UserDefaultsSearch{
			DisableOptions: u.DisableSearchOptions,
		},
		UI: UserDefaultsUI{
			DarkMode:    u.DarkMode,
			ThemeColor:  u.ThemeColor,
			CustomTheme: u.CustomTheme,
			Locale:      u.Locale,
		},
		FileLoading: u.FileLoading,
		Account: UserDefaultsAccount{
			LockPassword:               u.LockPassword,
			RequirePasswordChange:      u.RequirePasswordChange,
			DisableSettings:            u.DisableSettings,
			DisableUpdateNotifications: u.DisableUpdateNotifications,
			ShowAdvancedProfile:        u.ShowAdvancedProfile,
			MaxConcurrentTranscodes:    u.MaxConcurrentTranscodes,
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
	u.RequirePasswordChange = p.Account.RequirePasswordChange
	u.DisableUpdateNotifications = p.Account.DisableUpdateNotifications
	u.ShowAdvancedProfile = p.Account.ShowAdvancedProfile
	if p.Account.MaxConcurrentTranscodes > 0 {
		u.MaxConcurrentTranscodes = p.Account.MaxConcurrentTranscodes
	}
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
	u.ShowTypeColumn = p.Listing.ShowTypeColumn
	u.ShowCreationDateColumn = p.Listing.ShowCreationDateColumn
	u.DeleteAfterArchive = p.Listing.DeleteAfterArchive
	u.ViewMode = p.Listing.ViewMode
	u.GallerySize = p.Listing.GallerySize
	u.PromptRightCloseButton = p.Listing.PromptRightCloseButton
	u.NewFileTemplate = p.Listing.NewFileTemplate

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

	u.Preview.AutoplayMedia = boolValueOrDefault(p.FileViewer.AutoplayMedia, true)
	u.DisableViewingExt = p.FileViewer.DisableViewingExt
	u.DisableOnlyOfficeExt = p.FileViewer.DisableOnlyOfficeExt
	u.DisableOfficePreviewExt = p.FileViewer.DisableOnlyOfficeExt
	u.PreferEditorForMarkdown = p.FileViewer.PreferEditorForMarkdown
	u.PreferEditorForHtml = p.FileViewer.PreferEditorForHtml
	u.DebugOffice = p.FileViewer.DebugOffice

	u.DisableSearchOptions = p.Search.DisableOptions

	u.DarkMode = p.UI.DarkMode.Or(users.DarkModeDark)
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
		// Some stored profiles hold ui.darkMode as a string ("dark"/"light") instead of a bool.
		// Accept those so one bad value does not stop startup; retry the decode once normalized.
		normalized, normErr := normalizeLegacyUIDarkMode(profileJSON)
		if normErr != nil {
			return err
		}
		// The failed decode can leave partial values behind (e.g. a *bool allocated before the
		// type error), so start the retry from a zero profile.
		p = UserProfile{}
		if err := json.Unmarshal(normalized, &p); err != nil {
			return err
		}
	}
	ExpandProfileIntoUser(u, p)
	return nil
}

// normalizeLegacyUIDarkMode rewrites a string ui.darkMode into a bool. It returns an error when
// there is no string darkMode to fix, or when the string is not a recognized theme value.
func normalizeLegacyUIDarkMode(profileJSON []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(profileJSON, &top); err != nil {
		return nil, err
	}
	var ui map[string]json.RawMessage
	if err := json.Unmarshal(top["ui"], &ui); err != nil {
		return nil, err
	}
	var s string
	if err := json.Unmarshal(ui["darkMode"], &s); err != nil {
		return nil, err // darkMode is not a string, so this is not the legacy case
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dark", "true":
		ui["darkMode"], _ = json.Marshal(true)
	case "light", "false":
		ui["darkMode"], _ = json.Marshal(false)
	case "auto":
		// v2.2.0 "follow system" has no equivalent here. Drop the key so the default applies.
		delete(ui, "darkMode")
	default:
		return nil, fmt.Errorf("unrecognized ui.darkMode value %q", s)
	}
	top["ui"], _ = json.Marshal(ui)
	return json.Marshal(top)
}

// ProfileJSONFromUser returns JSON for SQLite user_data.profile.
func ProfileJSONFromUser(u *users.User) ([]byte, error) {
	return json.Marshal(ProfileFromUser(u))
}
