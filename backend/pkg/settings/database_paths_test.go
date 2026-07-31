package settings

import (
	"path/filepath"
	"testing"
)

func TestResolveDatabasePaths_migrateFromDefaultUsesEnv(t *testing.T) {
	boltPath := filepath.Join(t.TempDir(), "database.db")
	t.Setenv("FILEBROWSER_DATABASE", boltPath)

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.MigrateFrom = MigrateFromDefault

	if err := ResolveDatabasePaths(); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}
	if Config.Server.DatabaseV2.MigrateFrom != boltPath {
		t.Fatalf("migrateFrom = %q, want %q", Config.Server.DatabaseV2.MigrateFrom, boltPath)
	}
}

func TestResolveDatabasePaths_migrateFromDefaultFallsBackToDatabaseDB(t *testing.T) {
	t.Setenv("FILEBROWSER_DATABASE", "")

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.MigrateFrom = MigrateFromDefault

	if err := ResolveDatabasePaths(); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}
	if Config.Server.DatabaseV2.MigrateFrom != "database.db" {
		t.Fatalf("migrateFrom = %q, want database.db", Config.Server.DatabaseV2.MigrateFrom)
	}
}

func TestResolveDatabasePaths_emptyPathUsesEnvDefault(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.sqlite")
	t.Setenv("FILEBROWSER_DATABASE_PATH", want)
	t.Setenv("FILEBROWSER_DATABASE", "")

	Config = SetDefaults(false)
	Config.Server.DatabaseV2.Path = ""
	Config.Server.DatabaseV2.MigrateFrom = ""

	if err := ResolveDatabasePaths(); err != nil {
		t.Fatalf("ResolveDatabasePaths: %v", err)
	}
	if Config.Server.DatabaseV2.Path != want {
		t.Fatalf("path = %q, want %q", Config.Server.DatabaseV2.Path, want)
	}
}
