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

func TestShadowSkipsLegacyWhenGoMediaFails(t *testing.T) {
	withGate(t, false)
	withDevMode(t, true)

	var calls atomic.Int32
	// A missing file makes go-media fail, so the caller's own fallback handles it.
	_, ok := Probe("/does/not/exist.mp4", func() (ProbeResult, bool) {
		calls.Add(1)
		return ProbeResult{}, true
	})
	require.False(t, ok)
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(0), calls.Load())
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
