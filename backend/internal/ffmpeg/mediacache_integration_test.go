package ffmpeg

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/mediacache"
)

// TestProbeFileUsesSharedCache checks that ffprobe results go into the shared
// cache, cached values are reused, and WithoutCache bypasses both.
func TestProbeFileUsesSharedCache(t *testing.T) {
	svc := initIntegrationService(t)
	path, key := copyTestFixtureMP4(t)

	got, err := svc.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	cached, ok := mediacache.Probes.Get(key.String())
	if !ok || cached != *got {
		t.Fatalf("ProbeFile must store its result in the shared cache: cached=%+v ok=%v got=%+v", cached, ok, *got)
	}

	sentinel := FileProbeInfo{Duration: 1, VideoCodec: "sentinel"}
	mediacache.Probes.Set(key.String(), sentinel)
	got, err = svc.ProbeFile(context.Background(), path)
	if err != nil || *got != sentinel {
		t.Fatalf("ProbeFile must reuse a cached value (e.g. from go-media): got=%+v err=%v", got, err)
	}

	fresh, err := svc.ProbeFile(mediacache.WithoutCache(context.Background()), path)
	if err != nil || *fresh == sentinel {
		t.Fatalf("WithoutCache must probe fresh: got=%+v err=%v", fresh, err)
	}
	if cached, _ := mediacache.Probes.Get(key.String()); cached != sentinel {
		t.Fatal("WithoutCache must not overwrite the shared cache")
	}
}

// TestGetMediaDurationUsesSharedCache checks that cached durations are reused
// and WithoutCache probes fresh.
func TestGetMediaDurationUsesSharedCache(t *testing.T) {
	svc := initIntegrationService(t)
	path, key := copyTestFixtureMP4(t)

	mediacache.Durations.Set(key.String(), 4242)
	d, err := svc.GetMediaDuration(context.Background(), path)
	if err != nil || d != 4242 {
		t.Fatalf("GetMediaDuration must reuse a cached value: d=%v err=%v", d, err)
	}
	d, err = svc.GetMediaDuration(mediacache.WithoutCache(context.Background()), path)
	if err != nil || d == 4242 || d <= 0 {
		t.Fatalf("WithoutCache must probe fresh: d=%v err=%v", d, err)
	}
}