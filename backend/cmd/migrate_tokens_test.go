package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestMigrateAdminTokensRoundTripSQLite(t *testing.T) {
	boltPath := settingsMigrationBoltPath(t)
	sqlitePath := filepath.Join(t.TempDir(), "migrate-tokens.sqlite")

	oldDB, err := storm.Open(boltPath)
	if err != nil {
		t.Fatal(err)
	}
	defer oldDB.Close()

	sqlStore, _, err := sqldb.NewSQLStoreWithOptions(sqlitePath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sqlStore.Close()

	var list []*users.User
	if err = oldDB.All(&list); err != nil {
		t.Fatal(err)
	}
	var admin *users.User
	for _, u := range list {
		if u.Username == "admin" {
			admin = u
			break
		}
	}
	if admin == nil {
		t.Fatal("admin not found")
	}

	normalizeUserTokensBeforeSQLite(admin)
	if err = sqlStore.CreateUser(admin); err != nil {
		t.Fatal(err)
	}

	loaded, err := sqlStore.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	customized, ok := loaded.Tokens["customized"]
	if !ok {
		t.Fatalf("customized missing after reload: %#v", loaded.Tokens)
	}
	if !customized.Permissions.Admin {
		t.Fatalf("customized admin permission missing after reload: %#v", customized.Permissions)
	}
	if customized.Permissions.Api || customized.Permissions.Share || customized.Permissions.Realtime {
		t.Fatalf("customized should only retain admin global after reload: %#v", customized.Permissions)
	}
	if customized.Permissions.Modify || customized.Permissions.Create || customized.Permissions.Delete || customized.Permissions.Download {
		t.Fatalf("customized legacy file ops should be stripped: %#v", customized.Permissions)
	}

	out, _ := json.MarshalIndent(map[string]any{
		"customizedPermissions": customized.Permissions,
		"tokenNames":            len(loaded.Tokens),
	}, "", "  ")
	os.Stdout.Write(out)
	os.Stdout.Write([]byte("\n"))
}

func TestPromoteLegacyApiKeysBeforeSQLite(t *testing.T) {
	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "legacy-tokens"},
		Tokens:       map[string]users.AuthToken{},
	}
	user.ApiKeys = map[string]users.AuthToken{
		"legacy-sync": {Key: "raw-jwt-1"},
		"empty":       {},
	}
	user.Tokens["obsidian"] = users.AuthToken{Name: "obsidian", Token: "raw-jwt-existing"}

	promoteLegacyApiKeysBeforeSQLite(user)

	// Existing Tokens entries win; empty raw tokens are skipped.
	if got := user.Tokens["obsidian"].Token; got != "raw-jwt-existing" {
		t.Fatalf("existing token must win, got %q", got)
	}
	if got := user.Tokens["legacy-sync"].Token; got != "raw-jwt-1" {
		t.Fatalf("promoted token must carry raw JWT, got %q", got)
	}
	if _, ok := user.Tokens["raw-jwt-1"]; !ok {
		t.Fatal("expected raw-JWT alias key after promotion")
	}
	if _, ok := user.Tokens["empty"]; ok {
		t.Fatal("token without raw material must be skipped")
	}
}

func TestUpdateTokenHashBackfillSuccessStampsVersion(t *testing.T) {
	user := &users.User{FrontendUser: users.FrontendUser{Username: "no-tokens"}}
	user.Version = users.ProfileStorageVersion

	changed, failed := updateTokenHashBackfill(user)
	if failed {
		t.Fatal("tokenless user must not fail backfill")
	}
	if !changed {
		t.Fatal("expected version bump to count as changed")
	}
	if user.Version != users.TokenHashBackfillVersion {
		t.Fatalf("version = %d, want %d", user.Version, users.TokenHashBackfillVersion)
	}
}

func TestUpdateTokenHashBackfillFailureSkipsBump(t *testing.T) {
	// Force token hash registration to fail; the version must stay below
	// newest so the backfill retries on next startup.
	orig := addApiToken
	addApiToken = func(string, uint64) error {
		return errors.New("simulated token registration failure")
	}
	t.Cleanup(func() { addApiToken = orig })

	user := &users.User{FrontendUser: users.FrontendUser{Username: "backfill-retry"}}
	user.Version = users.ProfileStorageVersion
	user.ApiKeys = map[string]users.AuthToken{
		"legacy": {Key: "raw-jwt-retry"},
	}

	_, failed := updateTokenHashBackfill(user)
	if !failed {
		t.Fatal("expected failure when token registration fails")
	}
	if user.Version != users.ProfileStorageVersion {
		t.Fatalf("failed backfill must not bump version, got %d", user.Version)
	}
	if bumpToNewestVersion(user, failed) {
		t.Fatal("catch-all must not bump past a failed migration")
	}
}

func TestBumpToNewestVersion(t *testing.T) {
	user := &users.User{FrontendUser: users.FrontendUser{Username: "stale"}}
	user.Version = users.ProfileStorageVersion
	if !bumpToNewestVersion(user, false) {
		t.Fatal("expected bump for stale version without failure")
	}
	if user.Version != users.NewestUserVersion {
		t.Fatalf("version = %d, want %d", user.Version, users.NewestUserVersion)
	}
	if bumpToNewestVersion(user, false) {
		t.Fatal("must not bump when already newest")
	}
}
