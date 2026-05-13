package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// TestMigrateUserDefaults_OldToNew verifies that deprecated fields are correctly migrated to new fields
func TestMigrateUserDefaults_OldToNew(t *testing.T) {
	// Setup: Create UserDefaults with old fields set
	Config.UserDefaults = UserDefaults{
		// Old deprecated fields
		DarkMode:                   boolPtr(true),
		DisableQuickToggles:        true,
		HideSidebarFileActions:     true,
		StickySidebar:              true,
		SingleClick:                true,
		HideFilesInTree:            true,
		ShowToolsInSidebar:         boolPtr(false),
		DeleteWithoutConfirming:    true,
		DateFormat:                 true,
		ShowHidden:                 true,
		QuickDownload:              true,
		ShowSelectMultiple:         true,
		HideFileExt:                ".tmp",
		ShowCopyPath:               true,
		DeleteAfterArchive:         true,
		DisablePreviewExt:          ".exe",
		EditorQuickSave:            true,
		DisableViewingExt:          ".bin",
		DisableOnlyOfficeExt:       ".md",
		PreferEditorForMarkdown:    true,
		DebugOffice:                true,
		DisableSearchOptions:       true,
		ThemeColor:                 "var(--red)",
		CustomTheme:                "dark",
		Locale:                     "de",
		LockPassword:               true,
		DisableSettings:            true,
		LoginMethod:                "oidc",
		DisableUpdateNotifications: true,
		Permissions: UserDefaultsPermissions{
			Api:      true,
			Admin:    true,
			Modify:   true,
			Share:    true,
			Realtime: true,
			Delete:   true,
			Create:   true,
			Download: boolPtr(false),
		},
		Preview: UserDefaultsPreview{
			Image:              boolPtr(true),
			Video:              boolPtr(true),
			Audio:              boolPtr(true),
			MotionVideoPreview: boolPtr(true),
			Office:             boolPtr(true),
			PopUp:              boolPtr(true),
			HighQuality:        boolPtr(true),
			Folder:             boolPtr(true),
			Models:             boolPtr(true),
			// Deprecated fields that should be migrated
			DisableHideSidebar: true,
			AutoplayMedia:      true,
			DefaultMediaPlayer: true,
		},
		FileLoading: users.FileLoading{
			MaxConcurrent:   5,
			UploadChunkSize: 20,
		},
	}

	// Execute: Run migration
	migrateUserDefaults()

	// Verify: Check that new fields are populated from old fields
	ud := &Config.UserDefaults

	// Sidebar fields
	if !ud.Sidebar.DisableQuickToggles {
		t.Error("sidebar.disableQuickToggles should be true")
	}
	if !ud.Sidebar.HideFileActions {
		t.Error("sidebar.hideFileActions should be true")
	}
	if !ud.Sidebar.Sticky {
		t.Error("sidebar.sticky should be true")
	}
	if !ud.Sidebar.HideFiles {
		t.Error("sidebar.hideFiles should be true")
	}
	if ud.Sidebar.ShowTools == nil || *ud.Sidebar.ShowTools != false {
		t.Error("sidebar.showTools should be false")
	}
	if !ud.Sidebar.DisableHideOnPreview {
		t.Error("sidebar.disableHideOnPreview should be true")
	}

	// Listing fields
	if !ud.Listing.DeleteWithoutConfirming {
		t.Error("listing.deleteWithoutConfirming should be true")
	}
	if !ud.Listing.DateFormat {
		t.Error("listing.dateFormat should be true")
	}
	if !ud.Listing.ShowHidden {
		t.Error("listing.showHidden should be true")
	}
	if !ud.Listing.QuickDownload {
		t.Error("listing.quickDownload should be true")
	}
	if !ud.Listing.ShowSelectMultiple {
		t.Error("listing.showSelectMultiple should be true")
	}
	if !ud.Listing.SingleClick {
		t.Error("listing.singleClick should be true")
	}
	if ud.Listing.HideFileExt != ".tmp" {
		t.Errorf("listing.hideFileExt should be '.tmp', got '%s'", ud.Listing.HideFileExt)
	}
	if !ud.Listing.ShowCopyPath {
		t.Error("listing.showCopyPath should be true")
	}
	if !ud.Listing.DeleteAfterArchive {
		t.Error("listing.deleteAfterArchive should be true")
	}

	// Preview fields
	if ud.Preview.Image == nil || *ud.Preview.Image != true {
		t.Error("preview.image should be true")
	}
	if ud.Preview.Video == nil || *ud.Preview.Video != true {
		t.Error("preview.video should be true")
	}
	if ud.Preview.Audio == nil || *ud.Preview.Audio != true {
		t.Error("preview.audio should be true")
	}
	if ud.Preview.DisablePreviewExt != ".exe" {
		t.Errorf("preview.disablePreviewExt should be '.exe', got '%s'", ud.Preview.DisablePreviewExt)
	}

	// FileViewer fields
	if !ud.FileViewer.EditorQuickSave {
		t.Error("fileViewer.editorQuickSave should be true")
	}
	if ud.FileViewer.AutoplayMedia == nil || *ud.FileViewer.AutoplayMedia != true {
		t.Error("fileViewer.autoplayMedia should be true")
	}
	if ud.FileViewer.DisableViewingExt != ".bin" {
		t.Errorf("fileViewer.disableViewingExt should be '.bin', got '%s'", ud.FileViewer.DisableViewingExt)
	}
	if ud.FileViewer.DisableOnlyOfficeExt != ".md" {
		t.Errorf("fileViewer.disableOnlyOfficeExt should be '.md', got '%s'", ud.FileViewer.DisableOnlyOfficeExt)
	}
	if !ud.FileViewer.PreferEditorForMarkdown {
		t.Error("fileViewer.preferEditorForMarkdown should be true")
	}
	if !ud.FileViewer.DebugOffice {
		t.Error("fileViewer.debugOffice should be true")
	}

	// Search fields
	if !ud.Search.DisableOptions {
		t.Error("search.disableOptions should be true")
	}

	// UI fields
	if ud.UI.DarkMode == nil || *ud.UI.DarkMode != true {
		t.Error("ui.darkMode should be true")
	}
	if ud.UI.ThemeColor != "var(--red)" {
		t.Errorf("ui.themeColor should be 'var(--red)', got '%s'", ud.UI.ThemeColor)
	}
	if ud.UI.CustomTheme != "dark" {
		t.Errorf("ui.customTheme should be 'dark', got '%s'", ud.UI.CustomTheme)
	}
	if ud.UI.Locale != "de" {
		t.Errorf("ui.locale should be 'de', got '%s'", ud.UI.Locale)
	}

	// Account fields
	if !ud.Account.LockPassword {
		t.Error("account.lockPassword should be true")
	}
	if !ud.Account.DisableSettings {
		t.Error("account.disableSettings should be true")
	}
	if ud.Account.LoginMethod != "oidc" {
		t.Errorf("account.loginMethod should be 'oidc', got '%s'", ud.Account.LoginMethod)
	}
	if !ud.Account.DisableUpdateNotifications {
		t.Error("account.disableUpdateNotifications should be true")
	}

	// Account Permissions
	if !ud.Account.Permissions.Api {
		t.Error("account.permissions.api should be true")
	}
	if !ud.Account.Permissions.Admin {
		t.Error("account.permissions.admin should be true")
	}
	if !ud.Account.Permissions.Modify {
		t.Error("account.permissions.modify should be true")
	}
	if !ud.Account.Permissions.Share {
		t.Error("account.permissions.share should be true")
	}
	if !ud.Account.Permissions.Realtime {
		t.Error("account.permissions.realtime should be true")
	}
	if !ud.Account.Permissions.Delete {
		t.Error("account.permissions.delete should be true")
	}
	if !ud.Account.Permissions.Create {
		t.Error("account.permissions.create should be true")
	}
	if ud.Account.Permissions.Download == nil || *ud.Account.Permissions.Download != false {
		t.Error("account.permissions.download should be false")
	}
}
