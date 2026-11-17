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

	require.NoError(t, s.Record(Entry{CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: "first", Format: "mp4", SizeBytes: 10}))
	require.NoError(t, s.Record(Entry{CreatedAt: time.Now(), Operation: "probe", Label: "timeRegression", Detail: "second", Format: "mkv", SizeBytes: 20, Codecs: "video=h264 audio=aac"}))

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "second", entries[0].Detail)
	require.Equal(t, "video=h264 audio=aac", entries[0].Codecs)
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
		require.NoError(t, s.Record(Entry{CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: fmt.Sprint(i), Format: "mp4"}))
	}

	entries, err := s.List()
	require.NoError(t, err)
	require.Len(t, entries, MaxEntries)
	require.Equal(t, fmt.Sprint(MaxEntries+4), entries[0].Detail, "newest entry is kept")
}

func TestStoreClearRemovesAll(t *testing.T) {
	useCacheDir(t)
	s := &Store{}

	require.NoError(t, s.Record(Entry{CreatedAt: time.Now(), Operation: "duration", Label: "error", Detail: "x", Format: "mp4"}))
	require.NoError(t, s.Clear())

	entries, err := s.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}
