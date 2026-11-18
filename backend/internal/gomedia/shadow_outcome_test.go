package gomedia

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A container outside go-media's allow-list is declined by design: no fast
// attempt, no legacy shadow run, no regression.
func TestDeclinedContainerSkipsShadowEntirely(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)
	get := captureRegressions(t)

	var calls atomic.Int32
	legacy := func() (ProbeResult, bool) {
		calls.Add(1)
		return ProbeResult{Duration: 1, VideoCodec: "wmv3", AudioCodec: "wmapro"}, true
	}
	for _, p := range []string{"/tmp/clip.wmv", "/tmp/partial.crdownload", "/tmp/old.avi"} {
		_, ok := Probe(p, legacy)
		require.False(t, ok, p)
	}
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), calls.Load(), "legacy must not run for declined containers")
	require.Empty(t, get())
}

// When go-media reports unsupported (by design) and ffmpeg succeeds, the
// result is expected: it must not be recorded as an error regression.
func TestUnsupportedOutcomeIsNotARegression(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)
	get := captureRegressions(t)

	const path = "/tmp/unsupported-outcome.mkv"
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
	time.Sleep(20 * time.Millisecond)
	require.Empty(t, get(), "unsupported-by-design must not be an error regression")
}

// A plain go-media miss (no unsupported mark) with a successful ffmpeg run is
// still a real error regression.
func TestPlainMissIsStillAnErrorRegression(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)
	get := captureRegressions(t)

	done := make(chan struct{})
	legacy := func() (float64, bool) {
		defer close(done)
		return 12.5, true
	}
	_, ok := timed("duration", fixtureMP4, legacy, nil, func() (float64, bool) { return 0, false })
	require.False(t, ok)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy shadow did not run")
	}
	require.Eventually(t, func() bool { return len(get()) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, LabelError, get()[0].Label)
}

// The unsupported mark is consumed by the call that set it, even when shadow
// is off, so it cannot leak into a later call on the same path.
func TestUnsupportedMarkIsConsumed(t *testing.T) {
	withDevMode(t, true)
	markUnsupported("probe", "/tmp/x.mkv")
	require.True(t, takeUnsupported("probe", "/tmp/x.mkv"))
	require.False(t, takeUnsupported("probe", "/tmp/x.mkv"))

	withDevMode(t, false)
	markUnsupported("probe", "/tmp/x.mkv")
	require.False(t, takeUnsupported("probe", "/tmp/x.mkv"), "no mark is stored outside dev mode")
}
