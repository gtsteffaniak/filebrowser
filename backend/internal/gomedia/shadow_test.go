package gomedia

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

// withDevMode sets FILEBROWSER_DEVMODE behaviour for one test.
func withDevMode(t *testing.T, on bool) {
	t.Helper()
	prev := settings.Env.IsDevMode
	settings.Env.IsDevMode = on
	t.Cleanup(func() { settings.Env.IsDevMode = prev })
}

func TestShadowRunsLegacyOnlyInDevMode(t *testing.T) {
	withGate(t, false)

	var calls atomic.Int32
	legacy := func() (float64, bool) {
		calls.Add(1)
		return 0, true
	}

	withDevMode(t, false)
	_, ok := Duration(fixtureMP4, legacy)
	require.True(t, ok)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), calls.Load(), "legacy must not run outside dev mode")

	withDevMode(t, true)
	_, ok = Duration(fixtureMP4, legacy)
	require.True(t, ok)
	require.Eventually(t, func() bool { return calls.Load() == 1 }, 5*time.Second, 5*time.Millisecond,
		"legacy must run once in the background after go-media succeeds in dev mode")
}

// TestShadowDoesNotBlockResult checks that the caller gets the go-media result
// while the legacy run is still blocked, and that legacy finishes later.
func TestShadowDoesNotBlockResult(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)

	release := make(chan struct{})
	done := make(chan struct{})
	legacy := func() (float64, bool) {
		<-release
		close(done)
		return 0, true
	}

	resCh := make(chan bool, 1)
	go func() {
		_, ok := Duration(fixtureMP4, legacy)
		resCh <- ok
	}()
	select {
	case ok := <-resCh:
		require.True(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("Duration waited for the blocked shadow run")
	}

	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("background shadow run never finished")
	}
}

// TestShadowPanicIsRecovered checks that a panicking legacy run does not crash
// the process and the go-media result is still returned.
func TestShadowPanicIsRecovered(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)

	_, ok := Duration(fixtureMP4, func() (float64, bool) { panic("boom") })
	require.True(t, ok)
	time.Sleep(50 * time.Millisecond)
}

// TestShadowRunsLegacyWhenGoMediaFails checks that when go-media returns no
// result, shadow mode still runs ffmpeg and records an error regression if
// ffmpeg succeeds. The caller's own fallback still handles the request.
func TestShadowRunsLegacyWhenGoMediaFails(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)

	var got atomic.Pointer[Regression]
	SetRegressionSink(func(r Regression) { got.Store(&r) })
	t.Cleanup(func() { SetRegressionSink(nil) })

	var calls atomic.Int32
	// A missing file makes go-media fail, so the caller's own fallback handles it.
	_, ok := Probe("/does/not/exist.mp4", func() (ProbeResult, bool) {
		calls.Add(1)
		return ProbeResult{}, true
	})
	require.False(t, ok)
	require.Eventually(t, func() bool { return got.Load() != nil }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, LabelError, got.Load().Label)
	require.Equal(t, "probe", got.Load().Operation)
	require.Equal(t, "mp4", got.Load().Format)
}

func TestShadowSkippedWhenAlwaysUseFfmpeg(t *testing.T) {
	withGate(t, true)
	withDevMode(t, true)

	var calls atomic.Int32
	_, ok := Duration(fixtureMP4, func() (float64, bool) {
		calls.Add(1)
		return 0, true
	})
	require.False(t, ok)
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(0), calls.Load())
}
