package gomedia

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShadowStatsCountsAndPairedAverages(t *testing.T) {
	ResetShadowStats()
	t.Cleanup(ResetShadowStats)

	// Both sides succeeded: counted in the averages.
	shadowStats.observe("probe", shadowObservation{goOK: true, goLatency: 10 * time.Millisecond, ffOK: true, ffLatency: 20 * time.Millisecond})
	// go-media succeeded, ffmpeg failed: a run and a success, not a paired timing.
	shadowStats.observe("probe", shadowObservation{goOK: true, goLatency: 900 * time.Millisecond, ffOK: false})
	// go-media missed, ffmpeg succeeded, and it was recorded as a regression.
	shadowStats.observe("probe", shadowObservation{goOK: false, ffOK: true, regressed: true})
	shadowStats.observe("duration", shadowObservation{goOK: true, goLatency: time.Millisecond, ffOK: true, ffLatency: time.Millisecond, regressed: true})

	stats := ShadowStats()
	require.Len(t, stats, 2)
	require.Equal(t, "duration", stats[0].Operation, "sorted by operation")

	probe := stats[1]
	require.Equal(t, int64(3), probe.Runs)
	require.Equal(t, int64(2), probe.Succeeded)
	require.Equal(t, int64(1), probe.Regressions)
	require.Equal(t, int64(1), probe.PairedRuns)
	require.InDelta(t, 10, probe.GoAvgMs, 0.001, "average uses only paired runs")
	require.InDelta(t, 20, probe.FfmpegAvgMs, 0.001)

	require.Equal(t, int64(1), stats[0].Regressions)
}

func TestShadowStatsResetClearsEverything(t *testing.T) {
	shadowStats.observe("probe", shadowObservation{goOK: true})
	ResetShadowStats()
	require.Empty(t, ShadowStats())
}

func TestRegressionCarriesFileKeyWithoutPath(t *testing.T) {
	get := captureRegressions(t)

	recordRegression("probe", "/library/a.mkv", LabelError, "detail", "")
	recordRegression("probe", "/library/a.mkv", LabelError, "again", "")
	recordRegression("probe", "/library/b.mkv", LabelError, "other", "")

	rs := get()
	require.Len(t, rs, 3)
	require.NotEmpty(t, rs[0].FileKey)
	require.Equal(t, rs[0].FileKey, rs[1].FileKey, "same path, same key")
	require.NotEqual(t, rs[0].FileKey, rs[2].FileKey, "different path, different key")
	require.NotContains(t, rs[0].FileKey, "/library", "the key must not contain the path")
	require.Len(t, rs[0].FileKey, 64, "SHA-256 in hex")
}

func TestUnsupportedButFfmpegHandledIsListed(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)
	get := captureRegressions(t)

	var mu sync.Mutex
	var got []Incompatible
	SetIncompatibleSink(func(i Incompatible) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, i)
	})
	t.Cleanup(func() { SetIncompatibleSink(nil) })

	const path = "/tmp/unsupported-listed.wmv"
	done := make(chan struct{})
	legacy := func() (float64, bool) {
		defer close(done)
		return 12.5, true
	}
	markUnsupported("duration", path)
	_, ok := timed("duration", path, legacy, nil, func() (float64, bool) { return 0, false })
	require.False(t, ok)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy shadow did not run")
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 1
	}, 5*time.Second, 10*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, path, got[0].Path)
	require.Equal(t, "duration", got[0].Operation)
	require.Equal(t, "wmv", got[0].Format)
	require.Len(t, got[0].FileKey, 64)
	require.Empty(t, get(), "unsupported-but-handled is not a regression")
}
