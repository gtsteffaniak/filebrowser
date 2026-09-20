package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestInitAuthSigningKeyPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	store, _, err := sqldb.NewSQLStoreWithOptions(dbPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDb = store
	t.Cleanup(func() {
		_ = store.Close()
		sqlDb = nil
	})

	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	first := settings.Config.Auth.Key
	if first == "" {
		t.Fatal("expected generated key")
	}

	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	if settings.Config.Auth.Key != first {
		t.Fatalf("expected same key after reload, got different values")
	}
}
