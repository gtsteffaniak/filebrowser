package gomedia

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// probeShadowRuns returns how many shadow runs have been recorded for probe.
func probeShadowRuns() int64 {
	for _, s := range ShadowStats() {
		if s.Operation == "probe" {
			return s.Runs
		}
	}
	return 0
}

// Concurrent probes of one unsupported file share a single container read through
// probeGroup. Every caller must see the unsupported outcome, so none of them may
// record an error regression.
func TestConcurrentUnsupportedProbesAreNotErrorRegressions(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)
	get := captureRegressions(t)
	ResetShadowStats()
	t.Cleanup(ResetShadowStats)

	// Not a container go-media reads, so the probe is unsupported by design.
	path := filepath.Join(t.TempDir(), "not-media.mp4")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), 4096), 0o600))

	const callers = 16
	legacy := func() (ProbeResult, bool) {
		return ProbeResult{Duration: 1, VideoCodec: "h264"}, true
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = Probe(path, legacy)
		}()
	}
	close(start)
	wg.Wait()

	require.Eventually(t, func() bool { return probeShadowRuns() == callers }, 5*time.Second, 5*time.Millisecond)
	require.Empty(t, get(), "unsupported-by-design probes must not be error regressions for any caller")
}

// When the admission queue is full, the comparison is skipped. The caller still
// gets the go-media result right away and legacy never runs.
func TestShadowSkippedWhenQueueIsFull(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)

	filled := 0
	for len(shadowAdmit) < cap(shadowAdmit) {
		shadowAdmit <- struct{}{}
		filled++
	}
	t.Cleanup(func() {
		for i := 0; i < filled; i++ {
			<-shadowAdmit
		}
	})

	var calls atomic.Int32
	legacy := func() (float64, bool) {
		calls.Add(1)
		return 1, true
	}
	v, ok := timed("duration", fixtureMP4, legacy, nil, func() (float64, bool) { return 2, true })
	require.True(t, ok)
	require.Equal(t, 2.0, v)

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), calls.Load(), "no comparison runs while the queue is full")
}

// A comparison that waits behind another for the ffmpeg slot must not count that
// wait as ffmpeg time. Only the legacy call itself is measured.
func TestShadowLatencyExcludesSemaphoreWait(t *testing.T) {
	ResetShadowStats()
	t.Cleanup(ResetShadowStats)

	holding := make(chan struct{})
	release := make(chan struct{})
	holder := func() (float64, bool) {
		close(holding)
		<-release
		return 0, true
	}
	queued := func() (float64, bool) { return 0, true }

	done := make(chan struct{}, 2)
	go func() {
		runShadow("holder", fixtureMP4, time.Millisecond, 0.0, holder, nil, 0)
		done <- struct{}{}
	}()
	<-holding

	go func() {
		runShadow("queued", fixtureMP4, time.Millisecond, 0.0, queued, nil, 0)
		done <- struct{}{}
	}()
	// The queued comparison waits behind the holder for about 100ms.
	time.Sleep(100 * time.Millisecond)
	close(release)
	<-done
	<-done

	var queuedAvg float64
	for _, s := range ShadowStats() {
		if s.Operation == "queued" {
			queuedAvg = s.FfmpegAvgMs
		}
	}
	require.Less(t, queuedAvg, 50.0, "ffmpeg latency must not include the wait for the shared slot")
}
