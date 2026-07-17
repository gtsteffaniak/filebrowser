package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestValidateDatabasePaths_missingBoltOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	settings.Config.Server.DatabaseV2.MigrateFrom = filepath.Join(dir, "database.db.old")
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "filebrowser.sqlite")

	err := validateDatabasePaths()
	if err == nil {
		t.Fatal("expected error when migrateFrom file is missing on fresh install")
	}
}

func TestValidateDatabasePaths_emptyBoltOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	boltPath := filepath.Join(dir, "database.db.old")
	if err := os.WriteFile(boltPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.DatabaseV2.MigrateFrom = boltPath
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "filebrowser.sqlite")

	err := validateDatabasePaths()
	if err == nil {
		t.Fatal("expected error when migrateFrom file is empty on fresh install")
	}
}

func TestValidateDatabasePaths_missingBoltWhenSQLiteExists(t *testing.T) {
	dir := t.TempDir()
	settings.Config.Server.DatabaseV2.MigrateFrom = filepath.Join(dir, "missing.db.old")
	sqlitePath := filepath.Join(dir, "filebrowser.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.DatabaseV2.Path = sqlitePath

	if err := validateDatabasePaths(); err == nil {
		t.Fatal("expected error when migrateFrom is set but legacy file is missing after migration")
	}
}

func TestValidateDatabasePaths_okWhenSQLiteExistsWithoutMigrateFrom(t *testing.T) {
	dir := t.TempDir()
	settings.Config.Server.DatabaseV2.MigrateFrom = ""
	sqlitePath := filepath.Join(dir, "filebrowser.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.DatabaseV2.Path = sqlitePath

	if err := validateDatabasePaths(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if checkMigrationNeeded() {
		t.Fatal("migration should not run when sqlite already exists")
	}
}

func TestValidateDatabasePaths_okWhenSQLiteAndBoltExist(t *testing.T) {
	dir := t.TempDir()
	boltPath := filepath.Join(dir, "database.db.old")
	if err := os.WriteFile(boltPath, []byte("bolt"), 0o600); err != nil {
		t.Fatal(err)
	}
	sqlitePath := filepath.Join(dir, "filebrowser.sqlite")
	if err := os.WriteFile(sqlitePath, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.DatabaseV2.MigrateFrom = boltPath
	settings.Config.Server.DatabaseV2.Path = sqlitePath

	if err := validateDatabasePaths(); err != nil {
		t.Fatalf("expected no error when both databases exist: %v", err)
	}
	if checkMigrationNeeded() {
		t.Fatal("migration should not run when sqlite already exists")
	}
}

func TestValidateDatabasePaths_rejectsDatabaseDBAsSQLitePath(t *testing.T) {
	dir := t.TempDir()
	settings.Config.Server.DatabaseV2.MigrateFrom = ""
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "database.db")

	if err := validateDatabasePaths(); err == nil {
		t.Fatal("expected error when sqlite path is database.db on fresh install")
	}
}

func TestValidateDatabasePaths_rejectsUnrenamedLegacyDatabaseDB(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	settings.Config.Server.DatabaseV2.MigrateFrom = ""
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "filebrowser.sqlite")
	if err := os.WriteFile("database.db", []byte("legacy-bolt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := validateDatabasePaths(); err == nil {
		t.Fatal("expected error when unrenamed database.db exists on fresh install")
	}
}

func TestCheckMigrationNeeded_trueWhenBoltPresentAndSQLiteMissing(t *testing.T) {
	dir := t.TempDir()
	boltPath := filepath.Join(dir, "database.db.old")
	if err := os.WriteFile(boltPath, []byte("bolt"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.DatabaseV2.MigrateFrom = boltPath
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "filebrowser.sqlite")

	if err := validateDatabasePaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checkMigrationNeeded() {
		t.Fatal("expected migration to be needed")
	}
}

func TestValidateDatabasePaths_okFreshInstallWithoutMigrateFrom(t *testing.T) {
	dir := t.TempDir()
	settings.Config.Server.DatabaseV2.MigrateFrom = ""
	settings.Config.Server.DatabaseV2.Path = filepath.Join(dir, "filebrowser.sqlite")

	if err := validateDatabasePaths(); err != nil {
		t.Fatalf("expected no error on fresh install without migrateFrom: %v", err)
	}
}
