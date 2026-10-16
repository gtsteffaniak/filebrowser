package transcoding

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Wiring the manager at startup must not require the cache tree to exist: the
// directory is created on demand when a transcode starts, and transcoding may be
// disabled. Every read path has to tolerate a missing root.
func TestStoreCreatesRootOnDemand(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "transcoding")
	store, storeErr := NewStore(root)
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("NewStore must not create the cache root (stat err = %v)", statErr)
	}

	// Read paths must all work against a missing root.
	if err := store.ReconcileStartup(); err != nil {
		t.Fatalf("ReconcileStartup: %v", err)
	}
	if _, err := store.EvictInactive(nil, 0, time.Hour); err != nil {
		t.Fatalf("EvictInactive: %v", err)
	}
	if _, ok := store.FindReusableGeneration("fp"); ok {
		t.Fatal("no generation should be reusable for a missing root")
	}
	if _, _, err := store.ReadServePlaylist(filepath.Join(root, "fp", "gen")); err == nil {
		t.Fatal("reading a playlist from a missing generation must fail")
	}

	// The first job creates the whole tree.
	jobDir, err := store.EnsureJobDir("fp", "gen")
	if err != nil {
		t.Fatalf("EnsureJobDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(jobDir, "seg")); err != nil {
		t.Fatalf("job dir not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "fp", ".owner")); err != nil {
		t.Fatalf("fingerprint owner marker not created: %v", err)
	}
	// A fresh generation is not complete, so it is not reusable yet.
	if _, ok := store.FindReusableGeneration("fp"); ok {
		t.Fatal("an incomplete generation must not be reusable")
	}
}
