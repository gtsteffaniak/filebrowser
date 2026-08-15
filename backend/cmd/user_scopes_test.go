package cmd

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestUpdateUserScopes_mergesMissingDefaultEnabled(t *testing.T) {
	settings.Initialize(settingsMigrationConfigPath(t))
	alignSettingsSourcesForMigrationFixture(t)

	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "graham"},
		BackendScopes: []users.BackendScope{
			{Path: fixturePlaywrightSource, Scope: "/myfolder"},
			{Path: fixtureDockerSource, Scope: "/"},
		},
	}

	if !updateUserScopes(user) {
		t.Fatal("expected missing defaultEnabled access source to be merged")
	}
	if len(user.BackendScopes) != 3 {
		t.Fatalf("scopes=%#v want 3 including access", user.BackendScopes)
	}
	found := false
	for _, scope := range user.BackendScopes {
		if scope.Path == fixtureAccessSource {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("access source should be added for defaultEnabled: %#v", user.BackendScopes)
	}
	// Preserve existing scope paths
	for _, scope := range user.BackendScopes {
		if scope.Path == fixturePlaywrightSource && scope.Scope != "/myfolder" {
			t.Fatalf("playwright scope path changed: %#v", scope)
		}
	}
}

func TestMigrationGraham_gainsAccessAfterScopeMerge(t *testing.T) {
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
	// Migration fixture itself may omit access; merge happens in updateUserScopes.
	if _, ok := graham.BackendSourcePermissions[fixtureAccessSource]; ok {
		t.Fatalf("graham should not have access permissions from migration alone: %#v", graham.BackendSourcePermissions)
	}

	updateUserScopes(graham)
	updateSourcePermissions(graham)
	updateSidebarLinks(graham)

	foundScope := false
	for _, scope := range graham.BackendScopes {
		if scope.Path == fixtureAccessSource {
			foundScope = true
			break
		}
	}
	if !foundScope {
		t.Fatalf("graham should gain access scope after merge: %#v", graham.BackendScopes)
	}
	if _, ok := graham.BackendSourcePermissions[fixtureAccessSource]; !ok {
		t.Fatalf("graham should have access source permissions after merge: %#v", graham.BackendSourcePermissions)
	}

	foundLink := false
	for _, link := range usersidebar.FrontendLinks(graham.SidebarLinks, true) {
		if link.Name == "access" || link.SourceName == fixtureAccessSource {
			foundLink = true
			break
		}
	}
	if !foundLink {
		t.Fatalf("graham should have access sidebar link after merge: %#v", graham.SidebarLinks)
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
