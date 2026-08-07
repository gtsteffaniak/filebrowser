package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const settingsMigrationBoltMinSize = 1024

// Docker source paths stored in the committed database.db.old Bolt fixture.
const (
	fixturePlaywrightSource = "/app/frontend/tests/playwright-files"
	fixtureDockerSource     = "/app/backend"
	fixtureAccessSource     = "/tests/playwright-files"
)

// settingsMigrationBoltPath returns the committed read-only BoltDB reference used by
// migration tests and the Playwright settings Docker image. Tests must never write to it.
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

// alignSettingsSourcesForMigrationFixture maps config sources to the Docker paths
// stored in database.db.old so migration tests can run locally without modifying
// the committed Bolt fixture.
func alignSettingsSourcesForMigrationFixture(t *testing.T) {
	t.Helper()
	byName := map[string]string{
		"playwright + files": fixturePlaywrightSource,
		"docker":             fixtureDockerSource,
		"access":             fixtureAccessSource,
	}
	newSourceMap := make(map[string]*settings.Source, len(byName))
	for name, dockerPath := range byName {
		source, ok := settings.Config.Server.NameToSource[name]
		if !ok {
			t.Fatalf("source %q not in config", name)
		}
		if source.Path != dockerPath {
			delete(settings.Config.Server.SourceMap, source.Path)
			source.Path = dockerPath
		}
		newSourceMap[dockerPath] = source
		settings.Config.Server.NameToSource[name] = source
	}
	settings.Config.Server.SourceMap = newSourceMap
	sources := make([]*settings.Source, 0, len(byName))
	for _, name := range []string{"playwright + files", "docker", "access"} {
		sources = append(sources, settings.Config.Server.NameToSource[name])
	}
	settings.Config.Server.Sources = sources
}
