package transcoding

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreServeContractOpensNormalizedSegment(t *testing.T) {
	store, storeErr := NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr, err := NewManager(ctx, NewFakeEngine(), store, Options{BaseURL: "/testing"})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	info, err := mgr.Start(ctx, StartInput{
		Username:  "contract-user",
		UserLimit: 1,
		Source:    "s",
		Path:      "/clip.mp4",
		RealPath:  t.TempDir() + "/clip.mp4",
		Profile:   ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}

	jobDir, err := mgr.JobDir("contract-user", info.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the producer's initial writes, then populate the generation the way
	// ffmpeg plus the segment aligner would before the playlist is served.
	waitForInit(t, store, jobDir)
	writeServeFixture(t, jobDir, "#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4,\n00000.m4s\n")

	playlist, err := mgr.Playlist("contract-user", info.ID)
	if err != nil {
		t.Fatal(err)
	}
	base := "/testing/api/media/transcode/sessions/" + info.ID
	if !strings.Contains(string(playlist), base+"/init.m4s") {
		t.Fatalf("playlist missing init URL: %q", string(playlist))
	}
	if !strings.Contains(string(playlist), base+"/seg/00000.m4s") {
		t.Fatalf("playlist missing segment URL: %q", string(playlist))
	}

	initFile, err := mgr.OpenInit("contract-user", info.ID)
	if err != nil {
		t.Fatalf("open init: %v", err)
	}
	initFile.Close()

	segFile, err := mgr.OpenSegment("contract-user", info.ID, "00000.m4s")
	if err != nil {
		t.Fatalf("open segment: %v", err)
	}
	segFile.Close()
}

// writeServeFixture populates a generation with an aligned segment and a playlist,
// mirroring what ffmpeg plus the segment aligner produce on disk.
func writeServeFixture(t *testing.T, jobDir, playlist string) {
	t.Helper()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	segPath := filepath.Join(segDir, "00000.m4s")
	if err := os.WriteFile(segPath, make([]byte, 9000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segPath+".ready", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitForInit waits until the fake producer has written its init segment, so a
// test can populate the generation without racing the producer's own writes.
func waitForInit(t *testing.T, store *Store, jobDir string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !store.InitReady(jobDir) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the producer to write init.m4s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
