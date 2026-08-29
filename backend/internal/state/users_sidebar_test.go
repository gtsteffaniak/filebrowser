package state

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func initSidebarLinkTestDB(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
}

func countSourceSidebarLinks(links []users.SidebarLink) int {
	n := 0
	for _, link := range links {
		if strings.HasPrefix(link.Category, "source") {
			n++
		}
	}
	return n
}

func TestCreateUserAddsSidebarLinksForAllScopes(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "scope-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
				{Name: "include", Scope: "/"},
			},
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("scope-user")
	if err != nil {
		t.Fatal(err)
	}
	if count := countSourceSidebarLinks(loaded.SidebarLinks); count != 2 {
		t.Fatalf("source sidebar links = %d, want 2: %#v", count, loaded.SidebarLinks)
	}
}

func TestUpdateUserScopesAddsMissingSidebarLink(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "patch-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
			},
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "patch-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
				{Name: "include", Scope: "/"},
			},
		},
	}
	if err := UpdateUser(patch, "", "scopes"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("patch-user")
	if err != nil {
		t.Fatal(err)
	}
	if count := countSourceSidebarLinks(loaded.SidebarLinks); count != 2 {
		t.Fatalf("source sidebar links = %d, want 2: %#v", count, loaded.SidebarLinks)
	}
}

func TestUpdateUserScopesKeepsStaleSidebarLinks(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "stale-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
				{Name: "include", Scope: "/"},
			},
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "stale-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
			},
		},
	}
	if err := UpdateUser(patch, "", "scopes"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("stale-user")
	if err != nil {
		t.Fatal(err)
	}
	if count := countSourceSidebarLinks(loaded.SidebarLinks); count != 2 {
		t.Fatalf("expected stale include link kept, got %d links: %#v", count, loaded.SidebarLinks)
	}
}

func TestUpdateUserSidebarLinksPreservesFolderShortcuts(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "folder-user",
			FrontendScopes: []users.FrontendScope{
				{Name: "exclude", Scope: "/"},
			},
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	links := []users.SidebarLink{
		{Name: "exclude", Category: "source", SourceName: "exclude", Target: "/"},
		{Name: "Photos", Category: string(users.SidebarLinkSourceMinimal), Icon: "photo", SourceName: "exclude", Target: "/photos"},
		{Name: "divider", Category: "divider", Target: "#"},
	}
	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "folder-user",
			NonAdminEditable: users.NonAdminEditable{
				SidebarLinks: links,
			},
		},
	}
	if err := UpdateUser(patch, "", "sidebarLinks"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("folder-user")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SidebarLinks) != 3 {
		t.Fatalf("len(sidebarLinks) = %d, want 3: %#v", len(loaded.SidebarLinks), loaded.SidebarLinks)
	}
	if loaded.SidebarLinks[1].Name != "Photos" || loaded.SidebarLinks[1].Icon != "photo" || loaded.SidebarLinks[1].Target != "/photos" {
		t.Fatalf("folder shortcut not preserved: %#v", loaded.SidebarLinks[1])
	}
	if !strings.Contains(loaded.SidebarLinks[0].SourceName, "playwright-files") {
		t.Fatalf("expected canonical path persisted, got SourceName=%q", loaded.SidebarLinks[0].SourceName)
	}
}
