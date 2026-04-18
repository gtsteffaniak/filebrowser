package settings

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// TestApplyUserDefaults_copiesUserDefaultsOntoUser guards against drift between
// [UserDefaults] and [ApplyUserDefaults] (regression for #2278-style bugs).
func TestApplyUserDefaults_copiesUserDefaultsOntoUser(t *testing.T) {
	saved := Config
	defer func() { Config = saved }()

	dark := false
	dlFalse := false
	Config = Settings{
		Server: Server{},
		UserDefaults: UserDefaults{
			EditorQuickSave:            true,
			HideSidebarFileActions:     true,
			DisableQuickToggles:        true,
			DisableSearchOptions:       true,
			StickySidebar:              false,
			HideFilesInTree:            true,
			DarkMode:                   &dark,
			Locale:                     "de",
			ViewMode:                   "list",
			SingleClick:                true,
			ShowHidden:                 true,
			DateFormat:                 true,
			GallerySize:                7,
			ThemeColor:                 "var(--red)",
			QuickDownload:              true,
			DisablePreviewExt:          ".exe",
			DisableViewingExt:          ".bat",
			DisableSettings:            true,
			LockPassword:               true,
			DisableUpdateNotifications: true,
			DeleteWithoutConfirming:    true,
			DeleteAfterArchive:         false,
			DisableOfficePreviewExt:    "legacy",
			DisableOnlyOfficeExt:       ".x",
			CustomTheme:                "Night",
			ShowSelectMultiple:         true,
			DebugOffice:                true,
			PreferEditorForMarkdown:    true,
			LoginMethod:                "password",
			FileLoading: users.FileLoading{
				MaxConcurrent:     3,
				UploadChunkSize:   5,
				DownloadChunkSize: 2,
			},
			Permissions: UserDefaultsPermissions{
				Api:      true,
				Admin:    true,
				Modify:   true,
				Share:    true,
				Realtime: true,
				Delete:   true,
				Create:   true,
				Download: &dlFalse,
			},
			Preview: UserDefaultsPreview{
				DisableHideSidebar: true,
				Image:              boolPtr(false),
				Video:              boolPtr(false),
				Audio:              boolPtr(false),
				MotionVideoPreview: boolPtr(false),
				Office:             boolPtr(false),
				PopUp:              boolPtr(false),
				AutoplayMedia:      boolPtr(false),
				DefaultMediaPlayer: true,
				Folder:             boolPtr(false),
				Models:             boolPtr(false),
			},
		},
	}

	u := &users.User{Username: "alice"}
	ApplyUserDefaults(u)

	want := users.User{
		DisableSettings: true,
		LockPassword:    true,
		LoginMethod:     users.LoginMethodPassword,
		Permissions: users.Permissions{
			Api:      true,
			Admin:    true,
			Modify:   true,
			Share:    true,
			Realtime: true,
			Delete:   true,
			Create:   true,
			Download: false,
		},
		NonAdminEditable: users.NonAdminEditable{
			EditorQuickSave:            true,
			HideSidebarFileActions:     true,
			DisableQuickToggles:        true,
			DisableSearchOptions:       true,
			StickySidebar:              false,
			HideFilesInTree:            true,
			DarkMode:                   false,
			Locale:                     "de",
			ViewMode:                   "list",
			SingleClick:                true,
			ShowHidden:                 true,
			DateFormat:                 true,
			GallerySize:                7,
			ThemeColor:                 "var(--red)",
			QuickDownload:              true,
			DisablePreviewExt:          ".exe",
			DisableViewingExt:          ".bat",
			DisableUpdateNotifications: true,
			DisableOfficePreviewExt:    "legacy",
			DisableOnlyOfficeExt:       ".x",
			CustomTheme:                "Night",
			ShowSelectMultiple:         true,
			DebugOffice:                true,
			DeleteWithoutConfirming:    true,
			DeleteAfterArchive:         false,
			PreferEditorForMarkdown:    true,
			FileLoading: users.FileLoading{
				MaxConcurrent:     3,
				UploadChunkSize:   5,
				DownloadChunkSize: 2,
			},
			Preview: users.Preview{
				DisableHideSidebar: true,
				Image:              false,
				Video:              false,
				Audio:              false,
				MotionVideoPreview: false,
				Office:             false,
				PopUp:              false,
				AutoplayMedia:      false,
				DefaultMediaPlayer: true,
				Folder:             false,
				Models:             false,
			},
		},
	}

	if diff := cmp.Diff(want.Permissions, u.Permissions); diff != "" {
		t.Fatalf("Permissions mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want.NonAdminEditable, u.NonAdminEditable); diff != "" {
		t.Fatalf("NonAdminEditable mismatch (-want +got):\n%s", diff)
	}
	if u.DisableSettings != want.DisableSettings {
		t.Fatalf("DisableSettings: got %v want %v", u.DisableSettings, want.DisableSettings)
	}
	if u.LockPassword != want.LockPassword {
		t.Fatalf("LockPassword: got %v want %v", u.LockPassword, want.LockPassword)
	}
	if u.LoginMethod != want.LoginMethod {
		t.Fatalf("LoginMethod: got %q want %q", u.LoginMethod, want.LoginMethod)
	}
}

func TestApplyUserDefaults_setsLoginMethodWhenEmpty(t *testing.T) {
	saved := Config
	defer func() { Config = saved }()

	Config = Settings{
		Server:       Server{},
		UserDefaults: UserDefaults{LoginMethod: "jwt"},
	}
	u := &users.User{Username: "x"}
	ApplyUserDefaults(u)
	if u.LoginMethod != users.LoginMethodJwt {
		t.Fatalf("LoginMethod: got %q want jwt", u.LoginMethod)
	}
}

func TestApplyUserDefaults_preservesLoginMethodWhenAlreadySet(t *testing.T) {
	saved := Config
	defer func() { Config = saved }()

	Config = Settings{
		Server:       Server{},
		UserDefaults: UserDefaults{LoginMethod: "password"},
	}
	u := &users.User{Username: "x", LoginMethod: users.LoginMethodProxy}
	ApplyUserDefaults(u)
	if u.LoginMethod != users.LoginMethodProxy {
		t.Fatalf("LoginMethod: got %q want proxy", u.LoginMethod)
	}
}
