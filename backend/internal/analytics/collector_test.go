package analytics

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func resetCacheForTest(t *testing.T) {
	t.Helper()
	InvalidateCache()
	if Ready() {
		t.Fatal("cache should be empty after InvalidateCache")
	}
}

func setCachedEnvelopeForTest(data []byte) {
	cacheMu.Lock()
	cachedEnvelope = append([]byte(nil), data...)
	cacheMu.Unlock()
}

func setupAnalyticsTest(t *testing.T) {
	t.Helper()
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	resetCacheForTest(t)
}

func TestReadyReflectsCacheState(t *testing.T) {
	resetCacheForTest(t)
	if Ready() {
		t.Fatal("expected Ready() false for empty cache")
	}

	setCachedEnvelopeForTest([]byte(`{"cached":true}`))
	if !Ready() {
		t.Fatal("expected Ready() true after cache populated")
	}
}

func TestInvalidateCacheClearsSnapshot(t *testing.T) {
	setCachedEnvelopeForTest([]byte(`{"cached":true}`))
	InvalidateCache()
	if Ready() {
		t.Fatal("expected Ready() false after InvalidateCache")
	}
	if _, err := cachedSnapshotCopy(); err == nil {
		t.Fatal("expected cachedSnapshotCopy error after invalidation")
	}
}

func TestPreviewSnapshotUsesCachedCopy(t *testing.T) {
	resetCacheForTest(t)
	want := []byte(`{"preview":"cached"}`)
	setCachedEnvelopeForTest(want)

	got, err := PreviewSnapshot()
	if err != nil {
		t.Fatalf("PreviewSnapshot: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("PreviewSnapshot = %q, want %q", got, want)
	}

	// Mutating the returned slice must not affect the cache.
	got[0] = 'X'
	fresh, err := PreviewSnapshot()
	if err != nil {
		t.Fatalf("PreviewSnapshot second call: %v", err)
	}
	if string(fresh) != string(want) {
		t.Fatal("PreviewSnapshot should return independent copy of cached envelope")
	}
}

func TestRefreshSnapshotBuildsCache(t *testing.T) {
	setupAnalyticsTest(t)
	RefreshSnapshot()
	if !Ready() {
		t.Fatal("expected Ready() true after RefreshSnapshot")
	}

	body, err := PreviewSnapshot()
	if err != nil {
		t.Fatalf("PreviewSnapshot after refresh: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty preview body")
	}
}

func TestNotifySourceFullScanCompleteRebuildsCache(t *testing.T) {
	setupAnalyticsTest(t)
	NotifySourceFullScanComplete("test-source")
	if !Ready() {
		t.Fatal("expected Ready() true after full scan notification")
	}
}

func TestCacheConcurrency(t *testing.T) {
	resetCacheForTest(t)
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers * 3)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_ = Ready()
		}()
		go func() {
			defer wg.Done()
			InvalidateCache()
		}()
		go func() {
			defer wg.Done()
			setCachedEnvelopeForTest([]byte(`{"concurrent":true}`))
			_, _ = PreviewSnapshot()
		}()
	}

	wg.Wait()
}
