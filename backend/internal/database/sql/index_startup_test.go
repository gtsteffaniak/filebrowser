package sql

import (
	"os"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/testutil"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMain(m *testing.M) {
	testutil.QuietLogging()
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
		FTS5:                  true,
		StartupIntegrityCheck: mode,
	}
}

func testIndexSQLConfigNoFTS(mode string) settings.IndexSqlConfig {
	cfg := testIndexSQLConfig(mode)
	cfg.FTS5 = false
	return cfg
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

// TestNewIndexDB_ReusesHealthyDB checks that a matching schema is reused, not recreated.
func TestNewIndexDB_ReusesHealthyDB(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityOff))
	defer pop()

	db, isNew, err := NewIndexDB("reuse_ok", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Fatalf("fresh database should report isNewDb=true")
	}
	if err = db.InsertItem("src1", "/a.txt", &iteminfo.FileInfo{
		Path:     "/a.txt",
		ItemInfo: iteminfo.ItemInfo{Name: "a.txt", Type: "text/plain", ModTime: time.Unix(1, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, isNew2, err := NewIndexDB("reuse_ok", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if isNew2 {
		t.Fatalf("a healthy database should be reused, not recreated")
	}
	if !db2.FTSReady() {
		t.Fatalf("expected the FTS index to be ready on reuse")
	}
}

// TestNewIndexDB_FTSDisabledWhenConfigOff checks that FTS is not created when disabled.
func TestNewIndexDB_FTSDisabledWhenConfigOff(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfigNoFTS(settings.IndexStartupIntegrityOff))
	defer pop()

	db, _, err := NewIndexDB("fts_off", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.FTSReady() {
		t.Fatalf("FTS should be disabled when indexSqlConfig.fts5 is false")
	}
	if db.ftsTableExists() {
		t.Fatalf("the FTS table should not exist when disabled")
	}
}

// TestNewIndexDB_FTSToggleBuildsAndDrops checks that enabling FTS builds from existing
// rows (no rescan) and disabling it removes the table.
func TestNewIndexDB_FTSToggleBuildsAndDrops(t *testing.T) {
	dir := t.TempDir()

	// Start with FTS off and insert a row.
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfigNoFTS(settings.IndexStartupIntegrityOff))
	db, _, err := NewIndexDB("fts_toggle", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.InsertItem("s1", "/a/report.txt", &iteminfo.FileInfo{
		Path:     "/a/report.txt",
		ItemInfo: iteminfo.ItemInfo{Name: "report.txt", Type: "text/plain", ModTime: time.Unix(1, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	if db.FTSReady() {
		t.Fatalf("FTS should be off")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	pop()

	// Reopen with FTS on: it should build from the existing row without a rescan.
	pop2 := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityOff))
	db2, isNew, err := NewIndexDB("fts_toggle", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatalf("reused database should not be recreated when enabling FTS")
	}
	if !db2.FTSReady() {
		t.Fatalf("FTS should be ready after enabling")
	}
	if err = db2.checkFTSHealth(); err != nil {
		t.Fatalf("health after enabling: %v", err)
	}
	filter := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"report"}}, false, false)
	if got := runSearchItems(t, db2, "s1", "/", filter); !sameSet(got, []string{"/a/report.txt"}) {
		t.Fatalf("existing row should be searchable via FTS, got %v", got)
	}
	if err = db2.Close(); err != nil {
		t.Fatal(err)
	}
	pop2()

	// Disable again: the table and triggers should be dropped.
	pop3 := pushTestIndexConfig(t, dir, testIndexSQLConfigNoFTS(settings.IndexStartupIntegrityOff))
	defer pop3()
	db3, _, err := NewIndexDB("fts_toggle", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db3.Close()
	if db3.FTSReady() {
		t.Fatalf("FTS should be off after disabling")
	}
	if db3.ftsTableExists() {
		t.Fatalf("the FTS table should be dropped when disabled")
	}
}
