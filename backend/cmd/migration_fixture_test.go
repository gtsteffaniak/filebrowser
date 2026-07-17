package cmd

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMigrationFixturePostState(t *testing.T) {
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

	playwrightPath := fixturePlaywrightSource
	dockerPath := fixtureDockerSource
	accessPath := fixtureAccessSource

	admin, err := sqlStore.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.Permissions.Admin || !admin.Permissions.Api || !admin.Permissions.Share {
		t.Fatalf("admin globals=%+v", admin.Permissions)
	}
	if admin.Permissions.Realtime {
		t.Fatal("admin realtime should be false")
	}

	fullPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	for _, sourcePath := range []string{playwrightPath, dockerPath, accessPath} {
		assertSourceFilePerms(t, admin, sourcePath, fullPerms)
	}

	customized, ok := admin.Tokens["customized"]
	if !ok {
		t.Fatal("customized token missing")
	}
	if !customized.Permissions.Admin {
		t.Fatalf("customized token globals=%+v", customized.Permissions)
	}
	if customized.Permissions.Api || customized.Permissions.Share || customized.Permissions.Realtime {
		t.Fatalf("customized token should only have admin global: %+v", customized.Permissions)
	}
	if customized.Permissions.Modify || customized.Permissions.Create || customized.Permissions.Delete || customized.Permissions.Download {
		t.Fatalf("customized token legacy file ops should be stripped: %+v", customized.Permissions)
	}

	basic, err := sqlStore.GetUserByUsername("basic")
	if err != nil {
		t.Fatal(err)
	}
	if !basic.Permissions.Share || basic.Permissions.Admin || basic.Permissions.Api {
		t.Fatalf("basic globals=%+v", basic.Permissions)
	}
	viewOnly := users.SourceFilePermissions{View: true}
	for _, sourcePath := range []string{playwrightPath, dockerPath, accessPath} {
		assertSourceFilePerms(t, basic, sourcePath, viewOnly)
	}

	graham, err := sqlStore.GetUserByUsername("graham")
	if err != nil {
		t.Fatal(err)
	}
	grahamPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: false,
	}
	assertSourceFilePerms(t, graham, playwrightPath, grahamPerms)
	assertSourceFilePerms(t, graham, dockerPath, grahamPerms)

	allRules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	playwrightRules := allRules[playwrightPath]
	if rule, ok := playwrightRules["/text-files/bash.sh/"]; !ok || len(rule.Deny.Users) != 1 {
		t.Fatalf("playwright access rule=%+v ok=%v", rule, ok)
	} else if _, denied := rule.Deny.Users["admin"]; !denied {
		t.Fatalf("playwright access rule deny=%+v", rule.Deny.Users)
	}

	accessRules := allRules[accessPath]
	if len(accessRules) != 1 {
		t.Fatalf("access source rule count=%d", len(accessRules))
	}
	if rule, ok := accessRules["/"]; !ok || len(rule.Deny.Users) != 1 {
		t.Fatalf("access source rule=%+v ok=%v", rule, ok)
	} else if _, denied := rule.Deny.Users["basic"]; !denied {
		t.Fatalf("access source rule deny=%+v", rule.Deny.Users)
	}

	shares, err := sqlStore.GetSharesByUserID(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 2 {
		t.Fatalf("share count=%d", len(shares))
	}
	shareByHash := make(map[string]*share.Share, len(shares))
	for _, s := range shares {
		shareByHash[s.Hash] = s
	}
	if link, ok := shareByHash["lMhwHkF-hqCN92-QIJJZow"]; !ok || link.Path != "/myfolder/" ||
		!link.AllowModify || !link.AllowCreate || !link.AllowDelete {
		t.Fatalf("myfolder share=%+v", link)
	}
	if link, ok := shareByHash["dGhQi4AcMhva2Ne-7x7fvw"]; !ok || link.Path != "/test & test.txt/" ||
		link.AllowModify || link.AllowCreate || link.AllowDelete {
		t.Fatalf("test share=%+v", link)
	}
}

func assertSourceFilePerms(t *testing.T, user *users.User, sourcePath string, want users.SourceFilePermissions) {
	t.Helper()
	got, ok := user.BackendSourcePermissions[sourcePath]
	if !ok {
		t.Fatalf("user %q missing source permissions for %q", user.Username, sourcePath)
	}
	if got != want {
		t.Fatalf("user %q source %q perms=%+v want=%+v", user.Username, sourcePath, got, want)
	}
}
