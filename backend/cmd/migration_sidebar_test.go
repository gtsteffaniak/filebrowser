package cmd

import (
	"path/filepath"
	"testing"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func writeSidebarMigrationBolt(t *testing.T, path string) {
	t.Helper()
	db, err := storm.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			NonAdminEditable: users.NonAdminEditable{
				Password: "$2a$10$IYCsziHjzH0mPc.bZwRuXefQKPVXfFqjdyfVmNcL.XZJsgyfxljDy",
				SidebarLinks: []users.SidebarLink{
					{
						Name:       "My Files",
						Category:   string(users.SidebarLinkSourceMinimal),
						Icon:       "folder",
						SourceName: fixturePlaywrightSource,
						Target:     "/",
					},
					{
						Name:     "External Docs",
						Category: string(users.SidebarLinkCustom),
						Target:   "https://example.com/docs",
						Icon:     "link",
					},
				},
			},
			Username:    "sidebaruser",
			LoginMethod: users.LoginMethodPassword,
			FrontendScopes: []users.FrontendScope{
				{Name: fixturePlaywrightSource, Scope: "/"},
				{Name: fixtureDockerSource, Scope: "/"},
			},
			Permissions: users.Permissions{
				Download: true,
			},
		},
		Version: 3,
	}
	if err := db.Save(user); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationSidebarLinks_customMinimalPartialScopesAndCustomURL(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize(settingsMigrationConfigPath(t))
	settings.Env.IsPlaywright = true
	alignSettingsSourcesForMigrationFixture(t)

	boltPath := filepath.Join(t.TempDir(), "sidebar-migration.db.old")
	writeSidebarMigrationBolt(t, boltPath)

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	settings.Config.Server.DatabaseV2.Path = dbPath
	settings.Config.Server.DatabaseV2.MigrateFrom = boltPath

	if err := migrateFromBoltToSQLite(); err != nil {
		t.Fatal(err)
	}

	sqlStore, _, err := sqldb.NewSQLStoreWithOptions(dbPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sqlStore.Close()

	sidebarUser, err := sqlStore.GetUserByUsername("sidebaruser")
	if err != nil {
		t.Fatal(err)
	}

	links := sidebarUser.SidebarLinks
	if len(links) < 3 {
		t.Fatalf("sidebaruser links=%#v, want at least 3 after scope merge", links)
	}

	var minimalLink *users.SidebarLink
	var customLink *users.SidebarLink
	var dockerLink *users.SidebarLink
	for i := range links {
		link := links[i]
		switch {
		case link.Category == string(users.SidebarLinkSourceMinimal):
			minimalLink = &links[i]
		case link.Category == string(users.SidebarLinkCustom):
			customLink = &links[i]
		case link.SourceName == fixtureDockerSource || link.Name == "docker":
			dockerLink = &links[i]
		}
	}
	if minimalLink == nil {
		t.Fatalf("missing source-minimal link in %#v", links)
	}
	if minimalLink.Name != "My Files" || minimalLink.Icon != "folder" {
		t.Fatalf("custom minimal link not preserved: %#v", *minimalLink)
	}
	if customLink == nil || customLink.Name != "External Docs" || customLink.Target != "https://example.com/docs" {
		t.Fatalf("custom link not preserved: %#v", customLink)
	}
	if dockerLink == nil {
		t.Fatalf("docker scope link missing after merge: %#v", links)
	}

	frontendLinks := usersidebar.FrontendLinks(links, true)
	if len(frontendLinks) < 3 {
		t.Fatalf("frontend links=%#v", frontendLinks)
	}
}
