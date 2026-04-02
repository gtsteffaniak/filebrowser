package sql

import (
	"os"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func TestMain(m *testing.M) {
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0o644, 0o755)
	}
	os.Exit(m.Run())
}

func TestNormalizeStartupIntegrityMode(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", settings.IndexStartupIntegrityQuickCheck},
		{"quickCheck", settings.IndexStartupIntegrityQuickCheck},
		{"probe", settings.IndexStartupIntegrityProbe},
		{"off", settings.IndexStartupIntegrityOff},
		{"not-a-real-mode", settings.IndexStartupIntegrityQuickCheck},
	}
	for _, tt := range tests {
		if got := normalizeStartupIntegrityMode(tt.in); got != tt.want {
			t.Errorf("normalizeStartupIntegrityMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// pushTestIndexConfig sets CacheDir and index SQL options for NewIndexDB; returned func restores prior values.
func pushTestIndexConfig(t *testing.T, cacheDir string, idx settings.IndexSqlConfig) func() {
	t.Helper()
	prevDir := settings.Config.Server.CacheDir
	prevIdx := settings.Config.Server.IndexSqlConfig
	settings.Config.Server.CacheDir = cacheDir
	settings.Config.Server.IndexSqlConfig = idx
	return func() {
		settings.Config.Server.CacheDir = prevDir
		settings.Config.Server.IndexSqlConfig = prevIdx
	}
}

func testIndexSQLConfig(mode string) settings.IndexSqlConfig {
	return settings.IndexSqlConfig{
		WalMode:               false,
		BatchSize:             1000,
		CacheSizeMB:           32,
		DisableReuse:          false,
		StartupIntegrityCheck: mode,
	}
}

func TestNewIndexDB_StartupProbe_NewFile(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityProbe))
	defer pop()

	db, _, err := NewIndexDB("probe_new", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestNewIndexDB_StartupProbe_ReopenWithData(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityProbe))
	defer pop()

	db, _, err := NewIndexDB("probe_reopen", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	info := &iteminfo.FileInfo{
		Path: "/a.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "a.txt",
			Size:    12,
			ModTime: time.Unix(1, 0),
			Type:    "text/plain",
			Hidden:  false,
		},
	}
	if err = db.InsertItem("src1", "/a.txt", info); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	var db2 *IndexDB
	db2, _, err = NewIndexDB("probe_reopen", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
}

func TestNewIndexDB_StartupQuickCheckStillWorks(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityQuickCheck))
	defer pop()

	db, _, err := NewIndexDB("quick_default", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestNewIndexDB_StartupOff(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityOff))
	defer pop()

	db, _, err := NewIndexDB("off_mode", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}
