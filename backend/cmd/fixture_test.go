package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

const settingsMigrationBoltMinSize = 1024

// settingsMigrationBoltPath returns the committed BoltDB fixture used by settings
// migration tests and the Playwright settings Docker image.
func settingsMigrationBoltPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "_docker", "src", "settings", "backend", "database.db.old"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf(
			"settings migration fixture not found at %s: %v (commit _docker/src/settings/backend/database.db.old)",
			path,
			err,
		)
	}
	if info.Size() < settingsMigrationBoltMinSize {
		t.Fatalf("settings migration fixture at %s is empty or too small (%d bytes)", path, info.Size())
	}
	return path
}

func settingsMigrationConfigPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "_docker", "src", "settings", "backend", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings migration config not found at %s: %v", path, err)
	}
	return path
}
