package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMediaCacheDefaults(t *testing.T) {
	prev := Config.Integrations.Media.CacheDurationMins
	t.Cleanup(func() {
		Config.Integrations.Media.CacheDurationMins = prev
	})
	Config.Integrations.Media.CacheDurationMins = -5
	normalizeMediaCache()
	if got := MediaCacheDurationMins(); got != 0 {
		t.Fatalf("cacheDurationMins = %d, want 0", got)
	}
	if MediaDiskCacheEnabled() {
		t.Fatal("expected disk cache disabled")
	}
}

func TestMediaDiskCacheEnabled(t *testing.T) {
	prev := Config.Integrations.Media.CacheDurationMins
	t.Cleanup(func() {
		Config.Integrations.Media.CacheDurationMins = prev
	})
	Config.Integrations.Media.CacheDurationMins = 30
	if !MediaDiskCacheEnabled() {
		t.Fatal("expected disk cache enabled")
	}
	if got := MediaCacheDuration(); got != 30*time.Minute {
		t.Fatalf("duration = %s", got)
	}
}

func TestTranscodeCacheDir(t *testing.T) {
	prev := Config.Server.CacheDir
	t.Cleanup(func() {
		Config.Server.CacheDir = prev
	})
	Config.Server.CacheDir = "/tmp/fb-cache"
	if got := TranscodeCacheDir(); got != filepath.Join("/tmp/fb-cache", "transcode") {
		t.Fatalf("dir = %q", got)
	}
}

func TestPrepareTranscodeCacheDirIsNoOp(t *testing.T) {
	prevMins := Config.Integrations.Media.CacheDurationMins
	prevCache := Config.Server.CacheDir
	t.Cleanup(func() {
		Config.Integrations.Media.CacheDurationMins = prevMins
		Config.Server.CacheDir = prevCache
	})

	root := t.TempDir()
	Config.Server.CacheDir = root
	Config.Integrations.Media.CacheDurationMins = 10
	dir := TranscodeCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareTranscodeCacheDir(); err != nil {
		t.Fatalf("PrepareTranscodeCacheDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("expected PrepareTranscodeCacheDir to leave cache dir untouched")
	}
}
