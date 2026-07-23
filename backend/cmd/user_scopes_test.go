package cmd

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestUpdateUserScopes_preservesPartialScopes(t *testing.T) {
	settings.Initialize(settingsMigrationConfigPath(t))
	alignSettingsSourcesForMigrationFixture(t)

	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "graham"},
		BackendScopes: []users.BackendScope{
			{Path: fixturePlaywrightSource, Scope: "/myfolder"},
			{Path: fixtureDockerSource, Scope: "/"},
		},
	}

	updateUserScopes(user)
	if len(user.BackendScopes) != 2 {
		t.Fatalf("scopes=%#v want 2 partial scopes", user.BackendScopes)
	}
	for _, scope := range user.BackendScopes {
		if scope.Path == fixtureAccessSource {
			t.Fatalf("access source should not be added to partial-scope user: %#v", user.BackendScopes)
		}
	}
}

func TestMigrationGraham_noAccessScopeOrSidebarLink(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize(settingsMigrationConfigPath(t))
	settings.Env.IsPlaywright = true
	alignSettingsSourcesForMigrationFixture(t)

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	settings.Config.Server.DatabaseV2.Path = dbPath
	settings.Config.Server.DatabaseV2.MigrateFrom = settingsMigrationBoltPath(t)
	if err := migrateFromBoltToSQLite(); err != nil {
		t.Fatal(err)
	}

	sqlStore, _, err := sqldb.NewSQLStoreWithOptions(dbPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sqlStore.Close()

	graham, err := sqlStore.GetUserByUsername("graham")
	if err != nil {
		t.Fatal(err)
	}
	if graham.BackendSourcePermissions == nil {
		t.Fatal("expected backend source permissions after migration")
	}
	if _, ok := graham.BackendSourcePermissions[fixtureAccessSource]; ok {
		t.Fatalf("graham should not have access permissions after migration: %#v", graham.BackendSourcePermissions)
	}

	for _, link := range usersidebar.FrontendLinks(graham.SidebarLinks, true) {
		if link.Name == "access" {
			t.Fatalf("graham should not have access sidebar link after migration: %#v", graham.SidebarLinks)
		}
	}

	updateUserScopes(graham)
	updateSourcePermissions(graham)
	updateSidebarLinks(graham)

	for _, scope := range graham.BackendScopes {
		if scope.Path == fixtureAccessSource {
			t.Fatalf("graham should not gain access scope: %#v", graham.BackendScopes)
		}
	}
	if _, ok := graham.BackendSourcePermissions[fixtureAccessSource]; ok {
		t.Fatalf("graham should not have access source permissions: %#v", graham.BackendSourcePermissions)
	}

	frontendLinks := usersidebar.FrontendLinks(graham.SidebarLinks, true)
	for _, link := range frontendLinks {
		if link.Name == "access" {
			t.Fatalf("graham should not have access sidebar link: %#v", frontendLinks)
		}
	}
}

func TestUpdateUserScopes_seedsDefaultEnabledWhenEmpty(t *testing.T) {
	settings.Initialize(settingsMigrationConfigPath(t))
	alignSettingsSourcesForMigrationFixture(t)

	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "newuser"},
	}

	if !updateUserScopes(user) {
		t.Fatal("expected default-enabled scopes to be seeded")
	}
	if len(user.BackendScopes) != 3 {
		t.Fatalf("scopes=%#v want all default-enabled sources", user.BackendScopes)
	}
}
