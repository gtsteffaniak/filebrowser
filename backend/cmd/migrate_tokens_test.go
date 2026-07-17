package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestMigrateAdminTokensRoundTripSQLite(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	boltPath := filepath.Join(repoRoot, "_docker", "src", "settings", "backend", "database.db.old")
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
	if customized.Permissions.Modify || customized.Permissions.Create || customized.Permissions.Delete {
		t.Fatalf("file-op permissions should be stripped from token metadata: %#v", customized.Permissions)
	}

	out, _ := json.MarshalIndent(map[string]any{
		"customizedPermissions": customized.Permissions,
		"tokenNames":            len(loaded.Tokens),
	}, "", "  ")
	os.Stdout.Write(out)
	os.Stdout.Write([]byte("\n"))
}
