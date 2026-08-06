package activity

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestUserUpdateChangesSkipsSensitiveFields(t *testing.T) {
	before := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				DarkMode: true,
			},
		},
		TOTPSecret: "secret-value",
	}
	after := *before
	after.DarkMode = false
	after.TOTPSecret = "new-secret"

	changes := UserUpdateChanges(before, &after, []string{"totpSecret", "darkMode"}, false)
	for _, c := range changes {
		if c.Field == "totpSecret" {
			t.Fatalf("totpSecret must not appear in activity changes: %#v", changes)
		}
	}
}

func TestUserUpdateChangesFiltersUnchangedFields(t *testing.T) {
	before := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "akadmin",
			NonAdminEditable: users.NonAdminEditable{
				DarkMode:       true,
				StickySidebar:  true,
				Locale:         "en",
				SingleClick:    false,
				ThemeColor:     "var(--blue)",
				QuickDownload:  false,
				DeleteAfterArchive: true,
			},
		},
	}
	after := *before
	after.DarkMode = false
	after.Sorting = users.Sorting{By: "name", Asc: true}

	which := []string{
		"preview", "stickySidebar", "darkMode", "locale", "singleClick", "sorting",
		"showHidden", "dateFormat", "themeColor", "quickDownload", "fileLoading",
		"sidebarLinks", "deleteAfterArchive", "preferEditorForMarkdown", "showFirstLogin",
	}

	changes := UserUpdateChanges(before, &after, which, false)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d: %#v", len(changes), changes)
	}
	if changes[0].Field != "darkMode" || changes[0].From != "true" || changes[0].To != "false" {
		t.Fatalf("unexpected darkMode change: %#v", changes[0])
	}
	if changes[1].Field != "sorting.asc" || changes[1].From != "false" || changes[1].To != "true" {
		t.Fatalf("unexpected sorting.asc change: %#v", changes[1])
	}
	if changes[2].Field != "sorting.by" || changes[2].From != "" || changes[2].To != "name" {
		t.Fatalf("unexpected sorting.by change: %#v", changes[2])
	}
}

func TestValueFieldChangesPointerTransitions(t *testing.T) {
	perms := users.MarkSourceFilePermissionsConfigured(users.SourceFilePermissions{View: true})

	t.Run("nil and nil", func(t *testing.T) {
		var before, after *users.SourceFilePermissions
		changes := valueFieldChanges(reflect.ValueOf(before), reflect.ValueOf(after), "permissions")
		if len(changes) != 0 {
			t.Fatalf("expected no changes, got %#v", changes)
		}
	})

	t.Run("nil to value", func(t *testing.T) {
		var before *users.SourceFilePermissions
		changes := valueFieldChanges(reflect.ValueOf(before), reflect.ValueOf(perms), "permissions")
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %#v", changes)
		}
		if changes[0].Field != "permissions" {
			t.Fatalf("unexpected field: %#v", changes[0])
		}
		if changes[0].From != "null" {
			t.Fatalf("expected From null, got %q", changes[0].From)
		}
		if changes[0].To == "" || changes[0].To == "null" {
			t.Fatalf("expected permissions JSON in To, got %q", changes[0].To)
		}
	})

	t.Run("value to nil", func(t *testing.T) {
		var after *users.SourceFilePermissions
		changes := valueFieldChanges(reflect.ValueOf(perms), reflect.ValueOf(after), "permissions")
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %#v", changes)
		}
		if changes[0].Field != "permissions" {
			t.Fatalf("unexpected field: %#v", changes[0])
		}
		if changes[0].To != "null" {
			t.Fatalf("expected To null, got %q", changes[0].To)
		}
		if changes[0].From == "" || changes[0].From == "null" {
			t.Fatalf("expected permissions JSON in From, got %q", changes[0].From)
		}
	})
}

func TestUserUpdateChangesExpandsNestedStructFields(t *testing.T) {
	before := &users.User{
		FrontendUser: users.FrontendUser{
			NonAdminEditable: users.NonAdminEditable{
				Preview: users.Preview{
					AutoplayMedia: false,
					Image:         true,
					Video:         true,
				},
			},
		},
	}
	after := *before
	after.Preview.AutoplayMedia = true

	changes := UserUpdateChanges(before, &after, []string{"preview"}, false)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %#v", len(changes), changes)
	}
	if changes[0].Field != "preview.autoplayMedia" || changes[0].From != "false" || changes[0].To != "true" {
		t.Fatalf("unexpected preview.autoplayMedia change: %#v", changes[0])
	}
}

type activityChangePair struct {
	from string
	to   string
}

func testDownloadsSourceConfig() *users.SourceConfigProvider {
	return &users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			switch path {
			case "/Users/steffag/Downloads":
				return users.SourceInfo{Path: path, Name: "Downloads"}, true
			case "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files":
				return users.SourceInfo{Path: path, Name: "access"}, true
			default:
				return users.SourceInfo{}, false
			}
		},
	}
}

func withTestSourceConfig(t *testing.T, provider *users.SourceConfigProvider) {
	t.Helper()
	previous := users.GetSourceConfig()
	users.SetSourceConfig(provider)
	t.Cleanup(func() {
		users.SetSourceConfig(previous)
	})
}

func TestUserUpdateChangesExpandsPreviewBulkToggle(t *testing.T) {
	before := &users.User{
		FrontendUser: users.FrontendUser{
			NonAdminEditable: users.NonAdminEditable{
				Preview: users.Preview{
					DisableHideSidebar: false,
					Image:              true,
					Video:              true,
					Audio:              true,
					MotionVideoPreview: true,
					Office:             true,
					PopUp:              true,
					AutoplayMedia:      true,
					Folder:             true,
					Models:             true,
				},
			},
		},
	}
	after := *before
	after.Preview.DisableHideSidebar = true
	after.Preview.Image = false
	after.Preview.Video = false
	after.Preview.Audio = false
	after.Preview.MotionVideoPreview = false
	after.Preview.Office = false
	after.Preview.PopUp = false
	after.Preview.AutoplayMedia = false
	after.Preview.Folder = false
	after.Preview.Models = false

	changes := UserUpdateChanges(before, &after, []string{"preview"}, false)
	if len(changes) != 10 {
		t.Fatalf("expected 10 preview field changes, got %d: %#v", len(changes), changes)
	}
	foundDisableHide := false
	for _, c := range changes {
		if !strings.HasPrefix(c.Field, "preview.") {
			t.Fatalf("expected preview.* field, got %q", c.Field)
		}
		if c.Field == "preview.disableHideSidebar" {
			foundDisableHide = true
			if c.From != "false" || c.To != "true" {
				t.Fatalf("unexpected preview.disableHideSidebar change: %#v", c)
			}
			continue
		}
		if c.From != "true" || c.To != "false" {
			t.Fatalf("expected true -> false, got %#v", c)
		}
	}
	if !foundDisableHide {
		t.Fatal("expected preview.disableHideSidebar change")
	}
}

func TestUserUpdateChangesExpandsBackendSourcePermissions(t *testing.T) {
	withTestSourceConfig(t, testDownloadsSourceConfig())

	downloadsPath := "/Users/steffag/Downloads"
	accessPath := "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files"
	beforePerms := users.MarkSourceFilePermissionsConfigured(users.SourceFilePermissions{
		View: true, Download: true,
	})
	afterDownloadsPerms := users.MarkSourceFilePermissionsConfigured(users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Delete: true, Create: true,
	})
	afterAccessPerms := beforePerms

	before := &users.User{
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			downloadsPath: beforePerms,
			accessPath:    beforePerms,
		},
	}
	after := &users.User{
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			downloadsPath: afterDownloadsPerms,
			accessPath:    afterAccessPerms,
		},
	}

	changes := UserUpdateChanges(before, after, []string{"backendSourcePermissions"}, false)
	if len(changes) != 3 {
		t.Fatalf("expected 3 permission changes for Downloads only, got %d: %#v", len(changes), changes)
	}
	found := map[string]activityChangePair{}
	for _, c := range changes {
		found[c.Field] = activityChangePair{from: c.From, to: c.To}
	}
	for _, field := range []string{
		"backendSourcePermissions.Downloads.modify",
		"backendSourcePermissions.Downloads.delete",
		"backendSourcePermissions.Downloads.create",
	} {
		if pair, ok := found[field]; !ok || pair.from != "false" || pair.to != "true" {
			t.Fatalf("missing or unexpected change for %s: %#v", field, found)
		}
	}
}

func TestUserUpdateChangesExpandsScopePermissions(t *testing.T) {
	withTestSourceConfig(t, testDownloadsSourceConfig())

	downloadsPath := "/Users/steffag/Downloads"
	accessPath := "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files"
	basePerms := users.MarkSourceFilePermissionsConfigured(users.SourceFilePermissions{
		View: true, Download: true,
	})
	downloadsPerms := users.MarkSourceFilePermissionsConfigured(users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Delete: true, Create: true,
	})

	before := &users.User{
		BackendScopes: []users.BackendScope{
			{Path: downloadsPath, Scope: "/", Permissions: basePerms},
			{Path: accessPath, Scope: "/", Permissions: basePerms},
		},
	}
	after := &users.User{
		BackendScopes: []users.BackendScope{
			{Path: downloadsPath, Scope: "/", Permissions: downloadsPerms},
			{Path: accessPath, Scope: "/", Permissions: basePerms},
		},
	}

	changes := UserUpdateChanges(before, after, []string{"scopes"}, false)
	if len(changes) != 3 {
		t.Fatalf("expected 3 scope permission changes for Downloads only, got %d: %#v", len(changes), changes)
	}
	for _, c := range changes {
		if !strings.HasPrefix(c.Field, "scopes.Downloads.permissions.") {
			t.Fatalf("unexpected scope field %q", c.Field)
		}
		if c.From != "false" || c.To != "true" {
			t.Fatalf("expected false -> true, got %#v", c)
		}
	}
}

func TestShareUpdateChangesLogsChangedAttributes(t *testing.T) {
	before := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{
				ShareTheme: "light",
				Title:      "before",
			},
			ShareLimits: share.ShareLimits{
				DownloadsLimit: 5,
			},
		},
	}
	after := *before
	after.ShareTheme = "dark"
	after.Title = "after"
	after.DownloadsLimit = 10

	changes := ShareUpdateChanges(before, &after)
	if len(changes) < 3 {
		t.Fatalf("expected at least 3 changes, got %d: %#v", len(changes), changes)
	}
	found := map[string]activityChangePair{}
	for _, c := range changes {
		found[c.Field] = activityChangePair{from: c.From, to: c.To}
	}
	if pair, ok := found["shareTheme"]; !ok || pair.to != "dark" {
		t.Fatalf("missing shareTheme change: %#v", found)
	}
	if pair, ok := found["title"]; !ok || pair.to != "after" {
		t.Fatalf("missing title change: %#v", found)
	}
	if pair, ok := found["downloadsLimit"]; !ok || pair.from != "5" || pair.to != "10" {
		t.Fatalf("missing downloadsLimit change: %#v", found)
	}
	if _, ok := found["hash"]; ok {
		t.Fatalf("hash must not appear in share update changes: %#v", found)
	}
}

func TestSidebarLinksFieldChangeIgnoresPathVsName(t *testing.T) {
	withTestSourceConfig(t, &users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			switch path {
			case "/Users/steffag/Downloads":
				return users.SourceInfo{Path: path, Name: "Downloads"}, true
			case "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files":
				return users.SourceInfo{Path: path, Name: "access"}, true
			default:
				return users.SourceInfo{}, false
			}
		},
		GetSourceByName: func(name string) (users.SourceInfo, bool) {
			switch name {
			case "Downloads":
				return users.SourceInfo{Path: "/Users/steffag/Downloads", Name: "Downloads"}, true
			case "access":
				return users.SourceInfo{Path: "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files", Name: "access"}, true
			default:
				return users.SourceInfo{}, false
			}
		},
	})

	toolsLink := users.SidebarLink{Name: "Tools", Category: "tool", Target: "/tools", Icon: "build"}
	before := &users.User{
		FrontendUser: users.FrontendUser{
			NonAdminEditable: users.NonAdminEditable{
				ShowToolsInSidebar: true,
				SidebarLinks: []users.SidebarLink{
					toolsLink,
					{Name: "Downloads", Category: "source", Target: "/", SourceName: "/Users/steffag/Downloads"},
					{Name: "access", Category: "source", Target: "/", SourceName: "/Users/steffag/git/personal/filebrowser/frontend/tests/playwright-files"},
				},
			},
		},
	}
	after := &users.User{
		FrontendUser: users.FrontendUser{
			NonAdminEditable: users.NonAdminEditable{
				ShowToolsInSidebar: true,
				SidebarLinks: []users.SidebarLink{
					toolsLink,
					{Name: "Downloads", Category: "source", Target: "/", SourceName: "Downloads"},
					{Name: "access", Category: "source", Target: "/", SourceName: "access"},
				},
			},
		},
	}

	if _, ok := sidebarLinksFieldChange(before, after); ok {
		t.Fatal("expected sidebarLinks to be unchanged when only sourceName representation differs")
	}

	changes := UserUpdateChanges(before, after, []string{"sidebarLinks"}, false)
	if len(changes) != 0 {
		t.Fatalf("expected no sidebarLinks change in user update, got %#v", changes)
	}
}
