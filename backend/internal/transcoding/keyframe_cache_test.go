package transcoding

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestHardwareEncodeAvailable(t *testing.T) {
	if hardwareEncodeAvailable(nil) {
		t.Fatal("nil capabilities must report no hardware encode")
	}
	empty := capabilities.NewCapabilities()
	if hardwareEncodeAvailable(empty) {
		t.Fatal("unpopulated capabilities must report no hardware encode")
	}
	// A detected but unavailable QSV encoder must not count as available.
	unavailable := capabilities.NewCapabilities()
	unavailable.Encoders = map[string]capabilities.EncoderCapability{
		"h264_qsv": {Name: "h264_qsv", Compiled: true, Tested: true, Available: false, Kind: "qsv"},
	}
	if hardwareEncodeAvailable(unavailable) {
		t.Fatal("a failed probe must not count as hardware encode available")
	}
	// A working QSV encoder must count, so a silent libx264 fallback is reported.
	available := capabilities.NewCapabilities()
	available.Encoders = map[string]capabilities.EncoderCapability{
		"h264_qsv": {Name: "h264_qsv", Compiled: true, Tested: true, Available: true, Kind: "qsv"},
	}
	if !hardwareEncodeAvailable(available) {
		t.Fatal("an available h264 hardware encoder must report true")
	}
}

func TestKeyframeDiskCacheRoundTrip(t *testing.T) {
	settings.Config.Server.CacheDir = t.TempDir()
	t.Cleanup(func() { settings.Config.Server.CacheDir = "" })

	key := keyframeKey{path: filepath.Join(t.TempDir(), "movie.mkv"), size: 42, modTime: 7}
	if _, ok := readKeyframeDiskCache(key); ok {
		t.Fatal("empty cache must miss")
	}
	writeKeyframeDiskCache(key, []float64{0, 4, 8})
	got, ok := readKeyframeDiskCache(key)
	if !ok || len(got) != 3 {
		t.Fatalf("round trip failed: %v ok=%v", got, ok)
	}
	// A different file version (size/mtime) must not serve the cached index.
	if _, ok := readKeyframeDiskCache(keyframeKey{path: key.path, size: 43, modTime: 7}); ok {
		t.Fatal("a changed size must miss the disk cache")
	}
	if _, ok := readKeyframeDiskCache(keyframeKey{path: key.path, size: 42, modTime: 8}); ok {
		t.Fatal("a changed mtime must miss the disk cache")
	}
}

func TestKeyframeCacheReadsFromDiskWithoutProbing(t *testing.T) {
	settings.Config.Server.CacheDir = t.TempDir()
	t.Cleanup(func() { settings.Config.Server.CacheDir = "" })

	key := keyframeKey{path: "/persisted.mkv", size: 1, modTime: 2}
	writeKeyframeDiskCache(key, []float64{0, 2, 4})

	c := &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}
	var probes atomic.Int32
	got, err := c.get(context.Background(), key, func(context.Context) ([]float64, error) {
		probes.Add(1)
		return nil, errors.New("must not probe")
	})
	if err != nil || len(got) != 3 {
		t.Fatalf("expected disk hit, got %v err=%v", got, err)
	}
	if probes.Load() != 0 {
		t.Fatalf("disk hit must skip the probe (probes=%d)", probes.Load())
	}
}

func TestKeyframesExceedCopyCap(t *testing.T) {
	// 4s segments -> cap is max(12, 12) = 12s.
	if keyframesExceedCopyCap([]float64{0, 2, 4, 6}, 4) {
		t.Fatal("dense keyframes must not exceed the cap")
	}
	if !keyframesExceedCopyCap([]float64{0, 2, 31.8}, 4) {
		t.Fatal("a 29.8s gap must exceed the cap")
	}
	if keyframesExceedCopyCap([]float64{5}, 4) {
		t.Fatal("a single keyframe cannot exceed the cap")
	}
}

func TestKeyframesWithPreflightSkipsFullProbeWhenWindowRulesCopyOut(t *testing.T) {
	var fullCalls int
	times, err := keyframesWithPreflight(
		func() ([]float64, error) { return []float64{0, 29.8}, nil },
		func() ([]float64, error) { fullCalls++; return []float64{0, 2, 4}, nil },
		4,
	)
	if fullCalls != 0 {
		t.Fatalf("full probe must be skipped, ran %d times", fullCalls)
	}
	if len(times) != 0 {
		t.Fatalf("expected empty timeline on preflight short-circuit, got %v", times)
	}
	if !errors.Is(err, errCopyImpossibleByPreflight) {
		t.Fatalf("expected the preflight sentinel, got %v", err)
	}
}

func TestKeyframesWithPreflightFallsBackToFullProbe(t *testing.T) {
	var fullCalls int
	times, err := keyframesWithPreflight(
		func() ([]float64, error) { return []float64{0, 2, 4}, nil },
		func() ([]float64, error) { fullCalls++; return []float64{0, 2, 4, 6, 500}, nil },
		4,
	)
	if err != nil || fullCalls != 1 {
		t.Fatalf("full probe must run once when the window is inconclusive (calls=%d err=%v)", fullCalls, err)
	}
	if len(times) != 5 {
		t.Fatalf("expected the full timeline, got %v", times)
	}
}

func TestKeyframesWithPreflightProbeErrorRunsFullProbe(t *testing.T) {
	var fullCalls int
	_, err := keyframesWithPreflight(
		func() ([]float64, error) { return nil, errors.New("bounded probe failed") },
		func() ([]float64, error) { fullCalls++; return []float64{0, 2}, nil },
		4,
	)
	if err != nil || fullCalls != 1 {
		t.Fatalf("a failed window probe must fall through to the full probe (calls=%d err=%v)", fullCalls, err)
	}
}

func TestKeyframeCacheProbesOncePerFileVersion(t *testing.T) {
	c := &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}
	var calls atomic.Int32
	probe := func(context.Context) ([]float64, error) {
		calls.Add(1)
		return []float64{0, 4}, nil
	}
	key := keyframeKey{path: "/a.mkv", size: 10, modTime: 1}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := c.get(context.Background(), key, probe); err != nil || len(got) != 2 {
				t.Errorf("got %v, %v", got, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("probe calls = %d, want 1", calls.Load())
	}
	if _, err := c.get(context.Background(), keyframeKey{path: "/a.mkv", size: 10, modTime: 2}, probe); err != nil || calls.Load() != 2 {
		t.Fatalf("a modified file must be probed again (calls=%d, err=%v)", calls.Load(), err)
	}
}

func TestKeyframeCacheDoesNotCacheFailures(t *testing.T) {
	c := &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}
	key := keyframeKey{path: "/b.mkv"}
	if _, err := c.get(context.Background(), key, func(context.Context) ([]float64, error) { return nil, errors.New("boom") }); err == nil {
		t.Fatal("expected error")
	}
	got, err := c.get(context.Background(), key, func(context.Context) ([]float64, error) { return []float64{0}, nil })
	if err != nil || len(got) != 1 {
		t.Fatalf("retry after failure: %v, %v", got, err)
	}
}
