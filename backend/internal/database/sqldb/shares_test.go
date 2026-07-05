package sqldb

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestUnmarshalShareSettingsEmpty(t *testing.T) {
	link := &share.Share{}
	if err := unmarshalShareSettings(nil, link); err != nil {
		t.Fatalf("nil settings: %v", err)
	}
	if err := unmarshalShareSettings([]byte{}, link); err != nil {
		t.Fatalf("empty settings: %v", err)
	}
}

func TestSaveSharePreservesShareLimits(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shares.db")
	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}

	original := &share.Share{
		ShareSettings: share.ShareSettings{
			ShareLimits: share.ShareLimits{
				SourceName:           "default",
				AllowedUsernames:     []string{"alice", "bob"},
				DownloadsLimit:       5,
				MaxBandwidth:         100,
				PerUserDownloadLimit: true,
			},
		},
		ShareColumns: share.ShareColumns{
			Hash:   "test-hash-limits",
			Path:   "/docs/file.txt",
			Expire: 0,
		},
		SourcePath: "/data",
		UserID:     1,
		Version:    1,
	}

	if err = store.SaveShare(original); err != nil {
		t.Fatalf("SaveShare: %v", err)
	}

	loaded, err := store.GetShareByHash("test-hash-limits")
	if err != nil {
		t.Fatalf("GetShareByHash: %v", err)
	}
	if loaded.SourceName != "default" {
		t.Fatalf("SourceName = %q, want default", loaded.SourceName)
	}
	if len(loaded.AllowedUsernames) != 2 || loaded.AllowedUsernames[0] != "alice" || loaded.AllowedUsernames[1] != "bob" {
		t.Fatalf("AllowedUsernames = %#v", loaded.AllowedUsernames)
	}
	if loaded.DownloadsLimit != 5 {
		t.Fatalf("DownloadsLimit = %d, want 5", loaded.DownloadsLimit)
	}
	if loaded.MaxBandwidth != 100 {
		t.Fatalf("MaxBandwidth = %d, want 100", loaded.MaxBandwidth)
	}
	if !loaded.PerUserDownloadLimit {
		t.Fatal("expected PerUserDownloadLimit to be preserved")
	}
}
