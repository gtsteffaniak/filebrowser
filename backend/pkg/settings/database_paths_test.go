package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDatabasePaths_migrateFromDefault(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("server:\n  sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	boltPath := filepath.Join(dir, "database.db")
	if err := os.WriteFile(boltPath, []byte("bolt"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FILEBROWSER_DATABASE", boltPath)
	t.Setenv("FILEBROWSER_DATABASE_PATH", filepath.Join(dir, "filebrowser.sqlite"))

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.MigrateFrom = MigrateFromDefault
	Config.Server.DatabaseV2.Path = "filebrowser.sqlite"

	if err := ResolveDatabasePaths(configFile); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}

	if Config.Server.DatabaseV2.MigrateFrom != boltPath {
		t.Fatalf("migrateFrom = %q, want %q", Config.Server.DatabaseV2.MigrateFrom, boltPath)
	}
	wantSQLite := filepath.Join(dir, "filebrowser.sqlite")
	if Config.Server.DatabaseV2.Path != wantSQLite {
		t.Fatalf("path = %q, want %q", Config.Server.DatabaseV2.Path, wantSQLite)
	}
}

func TestResolveDatabasePaths_migrateFromDefaultMissingEnv(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("server:\n  sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FILEBROWSER_DATABASE", "")

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.MigrateFrom = MigrateFromDefault

	err := ResolveDatabasePaths(configFile)
	if err == nil {
		t.Fatal("expected error when migrateFrom is default but FILEBROWSER_DATABASE is unset")
	}
}

func TestResolveDatabasePaths_onlyMigrateFromUsesEnvDefaultPath(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("server:\n  sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	boltPath := filepath.Join(dir, "legacy.db")
	t.Setenv("FILEBROWSER_DATABASE", boltPath)
	t.Setenv("FILEBROWSER_DATABASE_PATH", filepath.Join(dir, "data", "filebrowser.sqlite"))

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.MigrateFrom = MigrateFromDefault
	Config.Server.DatabaseV2.Path = ""

	if err := ResolveDatabasePaths(configFile); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}

	wantSQLite := filepath.Join(dir, "data", "filebrowser.sqlite")
	if Config.Server.DatabaseV2.Path != wantSQLite {
		t.Fatalf("path = %q, want %q", Config.Server.DatabaseV2.Path, wantSQLite)
	}
}

func TestResolveDatabasePaths_relativePathsAgainstConfigDir(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("server:\n  sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.Path = "data/filebrowser.sqlite"
	Config.Server.DatabaseV2.MigrateFrom = "data/database.db.old"

	if err := ResolveDatabasePaths(configFile); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}

	if Config.Server.DatabaseV2.Path != filepath.Join(dir, "data/filebrowser.sqlite") {
		t.Fatalf("unexpected path: %q", Config.Server.DatabaseV2.Path)
	}
	if Config.Server.DatabaseV2.MigrateFrom != filepath.Join(dir, "data/database.db.old") {
		t.Fatalf("unexpected migrateFrom: %q", Config.Server.DatabaseV2.MigrateFrom)
	}
}

func TestLegacyDatabasePathInConfigDir(t *testing.T) {
	Env.ConfigDir = "/home/filebrowser/data"
	want := filepath.Join("/home/filebrowser/data", "database.db")
	if got := LegacyDatabasePathInConfigDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
