package gomedia

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// captureRegressions installs a sink that collects regressions for one test.
func captureRegressions(t *testing.T) func() []Regression {
	t.Helper()
	var mu sync.Mutex
	var out []Regression
	SetRegressionSink(func(r Regression) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, r)
	})
	t.Cleanup(func() { SetRegressionSink(nil) })
	return func() []Regression {
		mu.Lock()
		defer mu.Unlock()
		return append([]Regression(nil), out...)
	}
}

func TestRunShadowRecordsTimeRegressionAboveFiftyPercent(t *testing.T) {
	get := captureRegressions(t)

	legacy := func() (float64, bool) {
		time.Sleep(10 * time.Millisecond)
		return 0, true
	}
	// go-media took 30ms, ffmpeg about 10ms: +200%, well above the +50% threshold.
	runShadow("duration", fixtureMP4, 30*time.Millisecond, 0.0, legacy, nil)

	regs := get()
	require.Len(t, regs, 1)
	require.Equal(t, LabelTimeRegression, regs[0].Label)
	require.Equal(t, "duration", regs[0].Operation)
	require.Equal(t, "mp4", regs[0].Format)
	require.Contains(t, regs[0].Detail, "go-media")
	require.Contains(t, regs[0].Detail, "ffmpeg")
}

func TestRunShadowIgnoresSlowdownBelowThreshold(t *testing.T) {
	get := captureRegressions(t)

	legacy := func() (float64, bool) {
		time.Sleep(20 * time.Millisecond)
		return 0, true
	}
	// go-media 25ms vs ffmpeg ~20ms is +25%, under the +50% threshold.
	runShadow("duration", fixtureMP4, 25*time.Millisecond, 0.0, legacy, nil)

	require.Empty(t, get())
}

func TestRunShadowIgnoresTooShortFfmpegRuns(t *testing.T) {
	get := captureRegressions(t)

	// ffmpeg finishes instantly, so a ratio would be noise; no record is expected.
	runShadow("duration", fixtureMP4, 50*time.Millisecond, 0.0, func() (float64, bool) { return 0, true }, nil)

	require.Empty(t, get())
}

func TestFallbackShadowRecordsErrorWhenFfmpegSucceeds(t *testing.T) {
	get := captureRegressions(t)

	runFallbackShadow("duration", fixtureMP4, func() (float64, bool) { return 0, true })

	regs := get()
	require.Len(t, regs, 1)
	require.Equal(t, LabelError, regs[0].Label)
}

func TestFallbackShadowRecordsFfmpegCodecs(t *testing.T) {
	get := captureRegressions(t)

	runFallbackShadow("probe", fixtureMP4, func() (ProbeResult, bool) {
		return ProbeResult{VideoCodec: "h264", AudioCodec: "aac"}, true
	})

	regs := get()
	require.Len(t, regs, 1)
	require.Equal(t, "video=h264 audio=aac", regs[0].Codecs)
}

func TestFallbackShadowRecordsNothingWhenFfmpegFails(t *testing.T) {
	get := captureRegressions(t)

	runFallbackShadow("duration", fixtureMP4, func() (float64, bool) { return 0, false })

	require.Empty(t, get())
}

func TestRegressionDetailHasNoPath(t *testing.T) {
	get := captureRegressions(t)

	runFallbackShadow("duration", fixtureMP4, func() (float64, bool) { return 0, true })

	regs := get()
	require.Len(t, regs, 1)
	require.NotContains(t, regs[0].Detail, fixtureMP4)
	require.False(t, strings.Contains(regs[0].Format, "/"))
}
