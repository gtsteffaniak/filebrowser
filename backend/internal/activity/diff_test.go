package activity

import (
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
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %#v", len(changes), changes)
	}
	if changes[0].Field != "darkMode" || changes[0].From != "true" || changes[0].To != "false" {
		t.Fatalf("unexpected darkMode change: %#v", changes[0])
	}
	if changes[1].Field != "sorting" {
		t.Fatalf("unexpected second change: %#v", changes[1])
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

type activityChangePair struct {
	from string
	to   string
}

func TestSidebarLinksFieldChangeIgnoresPathVsName(t *testing.T) {
	hadSourceConfig := users.SourceConfigLoaded()
	t.Cleanup(func() {
		if !hadSourceConfig {
			users.SetSourceConfig(nil)
		}
	})
	users.SetSourceConfig(&users.SourceConfigProvider{
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
