package transcoding

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCompleteGeneration creates a finished generation (init + index.m3u8
// with ENDLIST + a published segment + complete job.json) so retention and
// reuse paths can be exercised.
func writeCompleteGeneration(t *testing.T, genDir string, segmentBytes int) {
	t.Helper()
	segDir := filepath.Join(genDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "init.m4s"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00000.m4s"), make([]byte, segmentBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXTINF:4.000000,\nseg/00000.m4s\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(genDir, "index.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}
	status := `{"startSec":0,"firstIndex":0,"lastReadyIndex":0,"producedSec":4,"endList":true,"complete":true}`
	if err := os.WriteFile(filepath.Join(genDir, "job.json"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStoreEvictInactiveByRetention(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	oldDir := filepath.Join(store.root, "fp-old", "gen-old")
	writeCompleteGeneration(t, oldDir, 9000)
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
	// The emptied fingerprint directory must be pruned.
	if _, err := os.Stat(filepath.Join(store.root, "fp-old")); !os.IsNotExist(err) {
		t.Fatal("expected empty fingerprint dir to be pruned")
	}
}

func TestStoreEvictInactiveBySize(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	writeGeneration := func(name string) string {
		t.Helper()
		dir := filepath.Join(store.root, "fp", name)
		writeCompleteGeneration(t, dir, 64*1024)
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

// A cancelled or failed transcode leaves a generation that can never be reused;
// it must be dropped on the next eviction instead of waiting out the retention
// window (which defaults to 24 hours).
func TestStoreEvictsIncompleteGenerationsImmediately(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	partialDir := filepath.Join(store.root, "fp-partial", "gen-partial")
	if err := os.MkdirAll(filepath.Join(partialDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partialDir, "init.m4s"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	// Old enough to be judged, but never completed.
	when := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(partialDir, when, when); err != nil {
		t.Fatal(err)
	}

	evicted, err := store.EvictInactive(nil, 0, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(evicted) != 1 || evicted[0] != partialDir {
		t.Fatalf("evicted=%v want [%q]", evicted, partialDir)
	}
	if _, err := os.Stat(partialDir); !os.IsNotExist(err) {
		t.Fatal("expected incomplete generation to be removed")
	}
}

// A brand-new generation must survive eviction so a producer that has not yet
// written its init segment is not deleted out from under ffmpeg.
func TestStoreKeepsFreshIncompleteGeneration(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	freshDir := filepath.Join(store.root, "fp-fresh", "gen-fresh")
	if err := os.MkdirAll(filepath.Join(freshDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}

	evicted, err := store.EvictInactive(nil, 0, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(evicted) != 0 {
		t.Fatalf("evicted=%v want none for a fresh generation", evicted)
	}
	if _, err := os.Stat(freshDir); err != nil {
		t.Fatalf("fresh generation removed: %v", err)
	}
}

// A completed generation is reusable by any user; it must be discoverable by
// fingerprint alone (no session or user identity in the cache key).
func TestStoreFindReusableGeneration(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	if _, _, ok := store.FindReusableGeneration("missing"); ok {
		t.Fatal("no generation should be reusable for an unknown fingerprint")
	}

	// An incomplete generation is not reusable.
	partial := filepath.Join(store.root, "fp", "gen-partial")
	if err := os.MkdirAll(filepath.Join(partial, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.FindReusableGeneration("fp"); ok {
		t.Fatal("an incomplete generation must not be reusable")
	}

	complete := filepath.Join(store.root, "fp", "gen-complete")
	writeCompleteGeneration(t, complete, 9000)
	got, cov, ok := store.FindReusableGeneration("fp")
	if !ok || got != complete {
		t.Fatalf("FindReusableGeneration = (%q, %v), want (%q, true)", got, ok, complete)
	}
	if cov.FirstIndex != 0 || cov.LastReadyIndex != 0 || !cov.Complete {
		t.Fatalf("coverage = %+v, want FirstIndex=0 LastReadyIndex=0 Complete", cov)
	}
}

// writeServeableSegment writes a published segment file large enough to pass
// the minimum-size serve check.
func writeServeableSegment(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, 9000), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ReadServePlaylist returns the publisher's index.m3u8 verbatim — the
// readiness filtering lives in the publisher, so serving is a plain read.
func TestStoreReadServePlaylistVerbatim(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-MEDIA-SEQUENCE:3\n" +
		"#EXTINF:4.000000,\nseg/00003.m4s\n#EXTINF:4.000000,\nseg/00004.m4s\n"
	if err := os.WriteFile(filepath.Join(jobDir, "index.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}

	data, hasSegments, err := store.ReadServePlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSegments {
		t.Fatal("expected hasSegments for a playlist listing segments")
	}
	if string(data) != playlist {
		t.Fatalf("serve playlist must be verbatim, got %q", string(data))
	}
}

// An index.m3u8 exists before its first segment publishes; hasSegments gates
// serving an empty playlist to hls.js.
func TestStoreReadServePlaylistEmptyIsNotReady(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jobDir, "index.m3u8"), []byte("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, hasSegments, err := store.ReadServePlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	if hasSegments {
		t.Fatal("an empty published playlist must read as not ready")
	}
}

// A completed tail covering only a few minutes of the file is discarded: it
// can never serve a from-start request and only helps seeks into a tiny
// range — not worth the disk. Full-file and larger tails are kept.
func TestStoreEvictsShortPartialGenerations(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	writeGen := func(fp, gen string, firstIdx, count int) string {
		t.Helper()
		dir := filepath.Join(store.root, fp, gen)
		writeProducedRange(t, dir, firstIdx, count, true)
		old := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	short := writeGen("fp-short", "gen", 40, 5) // 20s produced — discarded
	long := writeGen("fp-long", "gen", 40, 100) // 400s produced — kept
	full := writeGen("fp-full", "gen", 0, 5)    // whole file — kept

	evicted, err := store.EvictInactive(nil, 0, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, dir := range evicted {
		found[dir] = true
	}
	if !found[short] {
		t.Fatal("short partial generation should be evicted")
	}
	if found[long] {
		t.Fatal("a >=5min tail should be kept for seek coverage")
	}
	if found[full] {
		t.Fatal("a full-file generation must be kept")
	}
	if _, err := os.Stat(short); !os.IsNotExist(err) {
		t.Fatal("expected short partial generation removed")
	}
}
