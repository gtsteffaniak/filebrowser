package shadowregress

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

// useCacheDir points the cache directory at a temp dir for one test, so the
// database is created there the same way the index database is.
func useCacheDir(t *testing.T) string {
	t.Helper()
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0o644, 0o755)
	}
	dir := t.TempDir()
	prev := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = dir
	t.Cleanup(func() { settings.Config.Server.CacheDir = prev })
	return dir
}

func TestStoreRecordAndListNewestFirst(t *testing.T) {
	dir := useCacheDir(t)
	s := &Store{}

	require.NoError(t, s.Record(Entry{FileKey: "a", CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: "first", Format: "mp4", SizeBytes: 10}))
	require.NoError(t, s.Record(Entry{FileKey: "b", CreatedAt: time.Now(), Operation: "probe", Label: "timeRegression", Detail: "second", Format: "mkv", SizeBytes: 20, Codecs: "video=h264 audio=aac"}))

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "second", entries[0].Detail)
	require.Equal(t, "video=h264 audio=aac", entries[0].Codecs)
	require.Equal(t, int64(1), entries[0].Occurrences)
	require.Equal(t, "first", entries[1].Detail)

	// The database lives in the cache directory's sql/ folder, like the index database.
	_, err = os.Stat(filepath.Join(dir, "sql", "shadow_regressions.db"))
	require.NoError(t, err)
}

func TestStoreListEmptyIsEmptySlice(t *testing.T) {
	useCacheDir(t)
	s := &Store{}

	entries, err := s.List()
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.Empty(t, entries)
}

func TestStorePrunesToMaxEntries(t *testing.T) {
	useCacheDir(t)
	s := &Store{}

	for i := 0; i < MaxEntries+5; i++ {
		require.NoError(t, s.Record(Entry{FileKey: fmt.Sprint("file-", i), CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: fmt.Sprint(i), Format: "mp4"}))
	}

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, MaxEntries)
	require.Equal(t, fmt.Sprint(MaxEntries+4), entries[0].Detail, "newest entry is kept")
}

// One file, many regressions: one row. The row shows the latest detail, the
// first time the file was seen, and how many times it regressed.
func TestStoreOneRowPerFile(t *testing.T) {
	useCacheDir(t)
	s := &Store{}
	first := time.Now().Add(-time.Hour)

	for i := 0; i < 5; i++ {
		require.NoError(t, s.Record(Entry{
			FileKey:   "same-file",
			CreatedAt: first.Add(time.Duration(i) * time.Minute),
			Operation: "heic jpeg", Label: "timeRegression", Detail: fmt.Sprint("run ", i),
			Format: "heic", SizeBytes: 1777539,
		}))
	}
	// A different file is a different row, even with identical metadata.
	require.NoError(t, s.Record(Entry{FileKey: "other-file", CreatedAt: first, Operation: "heic jpeg", Label: "timeRegression", Detail: "other", Format: "heic", SizeBytes: 1777539}))

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// The file with the latest sighting is first.
	require.Equal(t, "run 4", entries[0].Detail)
	require.Equal(t, int64(5), entries[0].Occurrences)
	require.Equal(t, first.UnixMilli(), entries[0].FirstSeen.UnixMilli(), "first seen is kept")
	require.Equal(t, first.Add(4*time.Minute).UnixMilli(), entries[0].CreatedAt.UnixMilli())

	require.Equal(t, "other", entries[1].Detail)
	require.Equal(t, int64(1), entries[1].Occurrences)
}

// A file that keeps regressing stays in the list, even after many other files
// were recorded after its first row.
func TestStorePruneKeepsRecentlySeenFile(t *testing.T) {
	useCacheDir(t)
	s := &Store{}
	base := time.Now().Add(-time.Hour)

	// The busy file is recorded first, so it is the oldest row at the start.
	require.NoError(t, s.Record(Entry{FileKey: "busy", CreatedAt: base, Operation: "duration", Label: "error", Detail: "busy", Format: "mp4"}))
	for i := 0; i < MaxEntries-1; i++ {
		require.NoError(t, s.Record(Entry{FileKey: fmt.Sprint("other-", i), CreatedAt: base.Add(time.Duration(i+1) * time.Second), Operation: "duration", Label: "error", Detail: "x", Format: "mp4"}))
	}
	// The busy file regresses again. It updates its row and becomes the most recently seen.
	require.NoError(t, s.Record(Entry{FileKey: "busy", CreatedAt: base.Add(time.Hour), Operation: "duration", Label: "error", Detail: "busy again", Format: "mp4"}))
	// One more file takes the table over the limit, so one row is pruned.
	require.NoError(t, s.Record(Entry{FileKey: "last", CreatedAt: base.Add(2 * time.Hour), Operation: "duration", Label: "error", Detail: "last", Format: "mp4"}))

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, MaxEntries)
	require.Equal(t, "last", entries[0].Detail)
	require.Equal(t, "busy again", entries[1].Detail, "busy file survives the prune")
	require.Equal(t, int64(2), entries[1].Occurrences)
}

func TestStoreRejectsEntryWithoutFileKey(t *testing.T) {
	useCacheDir(t)
	s := &Store{}

	require.Error(t, s.Record(Entry{CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: "x", Format: "mp4"}))
	entries, err := s.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestStoreClearRemovesAll(t *testing.T) {
	useCacheDir(t)
	s := &Store{}

	require.NoError(t, s.Record(Entry{FileKey: "x", CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: "x", Format: "mp4"}))
	require.NoError(t, s.Clear())

	entries, err := s.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestStoreIncompatibleOneRowPerFileAndClear(t *testing.T) {
	useCacheDir(t)
	s := &Store{}
	now := time.Now()

	for i := 0; i < 3; i++ {
		require.NoError(t, s.RecordIncompatible(IncompatibleEntry{
			FileKey: "wmv-file", CreatedAt: now.Add(time.Duration(i) * time.Second),
			Operation: "video thumbnail", Path: "/lib/clip.wmv", Format: "wmv", SizeBytes: 50,
		}))
	}
	require.NoError(t, s.RecordIncompatible(IncompatibleEntry{FileKey: "mov-file", CreatedAt: now, Operation: "video thumbnail", Path: "/lib/a.mov", Format: "mov"}))

	list, err := s.ListIncompatible()
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "/lib/clip.wmv", list[0].Path)
	require.Equal(t, int64(3), list[0].Occurrences)
	require.Equal(t, int64(1), list[1].Occurrences)

	require.Error(t, s.RecordIncompatible(IncompatibleEntry{Path: "/x"}), "file key is required")

	// Clear removes both the regressions and the incompatible files.
	require.NoError(t, s.Record(Entry{FileKey: "r", CreatedAt: now, Operation: "probe", Label: "error", Detail: "x", Format: "mp4"}))
	require.NoError(t, s.Clear())
	list, err = s.ListIncompatible()
	require.NoError(t, err)
	require.Empty(t, list)
	entries, err := s.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}
