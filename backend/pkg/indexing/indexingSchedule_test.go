package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	indexingdb "github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	ierrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// fakeMetaStore is a minimal ports.IndexMetaStore for startup/restore tests.
type fakeMetaStore struct {
	info  *indexingdb.IndexInfo
	saved *indexingdb.IndexInfo
}

func (f *fakeMetaStore) GetIndexInfo(path string) (indexingdb.IndexInfo, error) {
	if f.info == nil {
		return indexingdb.IndexInfo{}, ierrors.ErrNotExist
	}
	return *f.info, nil
}

func (f *fakeMetaStore) SaveIndexInfo(info *indexingdb.IndexInfo) error {
	f.saved = info
	return nil
}

func (f *fakeMetaStore) ResetAllIndexComplexities() error { return nil }

// persistedScanners builds persisted scanner info that is one hour old on the slowest tier, so the
// restored nextRun lies in the future and no scan is due while a test runs.
func persistedScanners(paths ...string) map[string]*indexingdb.PersistedScannerInfo {
	out := make(map[string]*indexingdb.PersistedScannerInfo, len(paths))
	last := time.Now().Add(-time.Hour)
	for _, p := range paths {
		out[p] = &indexingdb.PersistedScannerInfo{
			Path:            p,
			CurrentSchedule: len(scanScheduleTiers) - 1,
			LastScanned:     last,
		}
	}
	return out
}

// newRestoreTestIndex returns an index over a temp source directory and a temp index database.
// persisted is served by the meta store unless it is nil (no persisted index data).
func newRestoreTestIndex(t *testing.T, name string, subdirs []string, persisted map[string]*indexingdb.PersistedScannerInfo) *Index {
	t.Helper()
	srcDir := t.TempDir()
	for _, dir := range subdirs {
		if err := os.MkdirAll(filepath.Join(srcDir, dir), 0o755); err != nil {
			t.Fatalf("creating source subdir: %v", err)
		}
	}
	db, _, err := dbsql.NewIndexDB("test_"+name, "OFF", 1000, 32, true)
	if err != nil {
		t.Fatalf("creating test index db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := &fakeMetaStore{}
	if persisted != nil {
		store.info = &indexingdb.IndexInfo{Path: srcDir, Source: name, Scanners: persisted}
	}
	SetMetaStore(store)
	t.Cleanup(func() { SetMetaStore(nil) })

	idx := &Index{
		Source:              settings.Source{Name: name, Path: srcDir},
		db:                  db,
		mock:                true,
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
		scanUpdatedPaths:    make(map[string]bool),
	}
	// Registered last so the scheduler goroutine stops before the database is closed.
	t.Cleanup(idx.stopScheduler)
	return idx
}

// A restart over a persisted index resumes the stored schedule instead of serial-scanning every
// scanner, which is what re-reads every already-indexed file on every start.
func TestRestartWithPersistedScannersSkipsSerialPass(t *testing.T) {
	idx := newRestoreTestIndex(t, "restart_no_pass", nil, persistedScanners("/"))
	idx.setupMultiScanner(false)
	if idx.needsAdaptiveBootstrap() {
		t.Fatalf("persisted scanners must resume their schedule, not force a serial startup pass")
	}
}

// folderSizes is in-memory only and quick scans pick the folders to stat from it, so a restart must
// restore it from the persisted index or change detection goes blind until the next full scan.
func TestFolderSizesRestoredOnStartup(t *testing.T) {
	idx := newRestoreTestIndex(t, "restore_sizes", []string{"sub"}, persistedScanners("/", "/sub/"))
	idx.UpdateMetadata(&iteminfo.FileInfo{
		Path:     "/",
		ItemInfo: iteminfo.ItemInfo{Type: "directory"},
		Folders: []iteminfo.ItemInfo{
			{Name: "sub", Type: "directory", Size: 4242},
		},
	}, nil, true)

	idx.setupMultiScanner(false)

	size, ok := idx.GetFolderSize("/sub/")
	if !ok {
		t.Fatalf("persisted folder size for /sub/ was not restored")
	}
	if size != 4242 {
		t.Fatalf("restored size for /sub/ = %d, want 4242", size)
	}
}

// fullScanCounter decides quick vs full for the next run; dropping it on restart means every
// scanner does a full scan (and DetectType opens every file) after every restart.
func TestFullScanCounterSurvivesRestart(t *testing.T) {
	first := newRestoreTestIndex(t, "counter_write", nil, nil)
	first.scanners = map[string]*Scanner{
		"/": {
			scanPath:        "/",
			idx:             first,
			fullScanCounter: 3,
			currentSchedule: len(scanScheduleTiers) - 1,
			lastScanned:     time.Now().Add(-time.Hour),
		},
	}
	if err := first.writePersistedIndexInfo(); err != nil {
		t.Fatalf("writePersistedIndexInfo: %v", err)
	}
	store, ok := metaStore.(*fakeMetaStore)
	if !ok || store.saved == nil || store.saved.Scanners["/"] == nil {
		t.Fatal("scanner info was not persisted")
	}
	if got := store.saved.Scanners["/"].FullScanCounter; got != 3 {
		t.Fatalf("persisted fullScanCounter = %d, want 3", got)
	}

	second := newRestoreTestIndex(t, "counter_read", nil, store.saved.Scanners)
	second.setupMultiScanner(false)
	second.mu.RLock()
	scanner := second.scanners["/"]
	second.mu.RUnlock()
	if scanner == nil {
		t.Fatal("root scanner missing after restart")
	}
	var restored int
	scanner.withStatsRLock(func() { restored = scanner.fullScanCounter })
	if restored != 3 {
		t.Fatalf("restored fullScanCounter = %d, want 3", restored)
	}
}

// An index with no persisted scan times must still do the serial pass, or a new or empty index
// would never be scanned.
func TestEmptyIndexStillBootstrapsSerialPass(t *testing.T) {
	idx := &Index{
		Source: settings.Source{Name: "empty", Path: t.TempDir()},
		mock:   true,
		scanners: map[string]*Scanner{
			"/": {scanPath: "/"},
		},
	}
	if !idx.needsAdaptiveBootstrap() {
		t.Fatal("a scanner with no lastScanned must trigger the serial pass")
	}
	idx.scanners["/"].lastScanned = time.Now()
	if idx.needsAdaptiveBootstrap() {
		t.Fatal("a scanner with a restored lastScanned must not trigger the serial pass")
	}
}
