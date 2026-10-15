package transcoding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeCompleteGeneration creates a finished generation (init + playlist with
// ENDLIST + a ready segment) so retention/size eviction can be exercised.
func writeCompleteGeneration(t *testing.T, genDir string, segmentBytes int) {
	t.Helper()
	segDir := filepath.Join(genDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "init.m4s"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	segPath := filepath.Join(segDir, "00000.m4s")
	if err := os.WriteFile(segPath, make([]byte, segmentBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segPath+".ready", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXTINF:4.000000,\nseg/00000.m4s\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(genDir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
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
	if _, ok := store.FindReusableGeneration("missing"); ok {
		t.Fatal("no generation should be reusable for an unknown fingerprint")
	}

	// An incomplete generation is not reusable.
	partial := filepath.Join(store.root, "fp", "gen-partial")
	if err := os.MkdirAll(filepath.Join(partial, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.FindReusableGeneration("fp"); ok {
		t.Fatal("an incomplete generation must not be reusable")
	}

	complete := filepath.Join(store.root, "fp", "gen-complete")
	writeCompleteGeneration(t, complete, 9000)
	got, ok := store.FindReusableGeneration("fp")
	if !ok || got != complete {
		t.Fatalf("FindReusableGeneration = (%q, %v), want (%q, true)", got, ok, complete)
	}
}

// writeServeableSegment writes a segment file large enough to pass the
// minimum-size serve check, optionally marking it aligned (.ready).
func writeServeableSegment(t *testing.T, dir, name string, ready bool) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, 9000), 0o644); err != nil {
		t.Fatal(err)
	}
	if ready {
		if err := os.WriteFile(path+".ready", []byte("1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestStoreReadPlaylistNormalizesBareSegments(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeServeableSegment(t, segDir, "00000.m4s", true)
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4,\n00000.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, _, err := store.ReadServePlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "seg/00000.m4s") {
		t.Fatalf("expected normalized playlist, got %q", string(data))
	}
	if strings.Count(string(data), "#EXTM3U") != 1 {
		t.Fatalf("expected a single #EXTM3U header, got %q", string(data))
	}
}

// Segments that are not yet aligned (no .ready marker) must be withheld from the
// served playlist so a client never requests a segment the server would reject.
func TestStoreReadServePlaylistWithholdsUnalignedSegments(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeServeableSegment(t, segDir, "00000.m4s", true)
	writeServeableSegment(t, segDir, "00001.m4s", false)
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:0\n" +
		"#EXT-X-MAP:URI=\"init.m4s\"\n" +
		"#EXTINF:4.000000,\nseg/00000.m4s\n#EXTINF:4.000000,\nseg/00001.m4s\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}

	data, ready, err := store.ReadServePlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "seg/00000.m4s") {
		t.Fatalf("ready segment missing from playlist: %q", got)
	}
	if strings.Contains(got, "seg/00001.m4s") {
		t.Fatalf("unaligned segment must be withheld: %q", got)
	}
	// ENDLIST must not appear while segments are still being produced.
	if strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Fatalf("incomplete playlist must not advertise ENDLIST: %q", got)
	}
	if ready != 1 {
		t.Fatalf("ready segments = %d, want 1", ready)
	}
}

// A finished playlist (all segments ready) keeps ENDLIST and reports a ready segment.
func TestStoreReadServePlaylistKeepsEndListWhenComplete(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeServeableSegment(t, segDir, "00000.m4s", true)
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXTINF:4.000000,\nseg/00000.m4s\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}

	data, _, err := store.ReadServePlaylist(jobDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "#EXT-X-ENDLIST") {
		t.Fatalf("complete playlist must keep ENDLIST: %q", string(data))
	}
}

// The served playlist must not mutate the on-disk file (no race with ffmpeg).
func TestStoreReadServePlaylistDoesNotMutateDisk(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	jobDir := t.TempDir()
	playlistPath := filepath.Join(jobDir, "ffmpeg.m3u8")
	original := "#EXTM3U\n#EXTINF:4.000000,\n00000.m4s\n"
	if err := os.WriteFile(playlistPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.ReadServePlaylist(jobDir); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("on-disk playlist was modified: %q", string(after))
	}
}

// A mid-stream session must report the first included segment as MEDIA-SEQUENCE
// so hls.js computes fragment sequence numbers correctly.
func TestBuildServePlaylistRewritesMediaSequence(t *testing.T) {
	raw := []byte("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:3\n" +
		"#EXTINF:4.000000,\nseg/00003.m4s\n#EXTINF:4.000000,\nseg/00004.m4s\n")
	out, ready := buildServePlaylist(raw, func(string) bool { return true })
	if ready != 2 {
		t.Fatalf("ready = %d, want 2", ready)
	}
	got := string(out)
	if !strings.Contains(got, "#EXT-X-MEDIA-SEQUENCE:3") {
		t.Fatalf("media sequence not preserved: %q", got)
	}
	if !strings.Contains(got, "#EXT-X-TARGETDURATION:4") {
		t.Fatalf("target duration missing: %q", got)
	}
}
