package transcoding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreEvictInactiveByRetention(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(store.root, "fp-old", "gen-old")
	if err := os.MkdirAll(filepath.Join(oldDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "init.m4s"), []byte("init"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	activeDir := filepath.Join(store.root, "fp-active", "gen-active")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	evicted, err := store.EvictInactive(map[string]struct{}{
		activeDir: {},
	}, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(evicted) != 1 || evicted[0] != oldDir {
		t.Fatalf("evicted=%v want [%q]", evicted, oldDir)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("expected old generation to be removed")
	}
	if _, err := os.Stat(activeDir); err != nil {
		t.Fatalf("active dir removed: %v", err)
	}
}

func TestStoreEvictInactiveBySize(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeGeneration := func(name string) string {
		t.Helper()
		dir := filepath.Join(store.root, "fp", name)
		if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
			t.Fatal(err)
		}
		payload := make([]byte, 64*1024)
		if err := os.WriteFile(filepath.Join(dir, "init.m4s"), payload, 0o644); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-time.Duration(len(name)) * time.Minute)
		if err := os.Chtimes(dir, when, when); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	first := writeGeneration("gen-a")
	second := writeGeneration("gen-b")

	evicted, err := store.EvictInactive(nil, 80*1024, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(evicted) != 1 || evicted[0] != first {
		t.Fatalf("evicted=%v want [%q]", evicted, first)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("expected oldest generation removed")
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second generation removed: %v", err)
	}
}

func TestStoreReadPlaylistNormalizesBareSegments(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobDir := t.TempDir()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00000.m4s"), []byte("segment-bytes-long-enough-for-serve-check"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4,\n00000.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := store.ReadPlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "seg/00000.m4s") {
		t.Fatalf("expected normalized playlist, got %q", string(data))
	}
}
