package transcoding

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreServeContractOpensNormalizedSegment(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobDir := t.TempDir()
	segDir := filepath.Join(jobDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "init.m4s"), []byte("init-segment-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	segPath := filepath.Join(segDir, "00000.m4s")
	payload := make([]byte, 9000)
	if err := os.WriteFile(segPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segPath+".ready", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "ffmpeg.m3u8"), []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4,\n00000.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
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

	mgr.mu.Lock()
	sess := mgr.sessions[info.ID]
	sess.jobDir = jobDir
	mgr.mu.Unlock()

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
