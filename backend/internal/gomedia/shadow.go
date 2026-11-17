package gomedia

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Legacy runs the existing ffmpeg (or dhowden) implementation of an operation
// and returns its result and whether it succeeded. It is only called in shadow
// mode, in a background goroutine.
type Legacy[T any] func() (T, bool)

// Differ compares the go-media value with the legacy value and returns one
// human-readable entry per difference. An empty result means they match.
type Differ[T any] func(fast, legacy T) []string

// Labels for significant shadow-mode regressions.
const (
	LabelTimeRegression = "timeRegression"
	LabelError          = "error"
)

// timeRegressionRatio is how much slower go-media may be than ffmpeg before a
// time regression is recorded. 1.5 means +50%.
const timeRegressionRatio = 1.5

// regressionMinFfmpegLatency skips timing ratios on runs too short to measure
// reliably.
const regressionMinFfmpegLatency = 5 * time.Millisecond

// Regression is one significant shadow-mode difference. It never holds a file
// path or name, only format and size information for diagnosis.
type Regression struct {
	CreatedAt time.Time
	Operation string
	Label     string
	Detail    string
	Format    string
	SizeBytes int64
	Codecs    string
}

var regressionSink atomic.Pointer[func(Regression)]

// SetRegressionSink registers the function that stores regressions. Until it is
// set, regressions are only logged.
func SetRegressionSink(fn func(Regression)) {
	if fn == nil {
		regressionSink.Store(nil)
		return
	}
	regressionSink.Store(&fn)
}

// recordRegression builds a Regression for path and hands it to the sink.
func recordRegression(op, path, label, detail, codecs string) {
	logger.Warningf("go-media shadow %s regression (%s) for %s: %s", op, label, path, detail)
	sink := regressionSink.Load()
	if sink == nil {
		return
	}
	r := Regression{
		CreatedAt: time.Now(),
		Operation: op,
		Label:     label,
		Detail:    detail,
		Format:    strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		Codecs:    codecs,
	}
	if fi, err := os.Stat(path); err == nil {
		r.SizeBytes = fi.Size()
	}
	(*sink)(r)
}

// describeCodecs returns the codec summary for results that carry codecs.
func describeCodecs(v any) string {
	switch p := v.(type) {
	case ProbeResult:
		if p.VideoCodec == "" && p.AudioCodec == "" {
			return ""
		}
		return fmt.Sprintf("video=%s audio=%s", p.VideoCodec, p.AudioCodec)
	}
	return ""
}

// ShadowEnabled reports whether shadow mode is on. Shadow mode runs the
// ffmpeg implementation after every successful go-media call, so the two can
// be compared. It is only active in dev mode (FILEBROWSER_DEVMODE=true).
func ShadowEnabled() bool {
	return settings.Env.IsDevMode
}

// timed runs the go-media fast path and logs its latency when it succeeds.
// In shadow mode, legacy runs in a background goroutine. The caller gets the
// go-media result immediately. The goroutine logs the timing and, when the
// results differ, a warning that says how they differ.
//
// When fast returns ok=false the caller falls back to its own ffmpeg code. In
// shadow mode legacy also runs in the background, so an ffmpeg success that
// go-media missed is recorded as an error regression.
func timed[T any](op, path string, legacy Legacy[T], diff Differ[T], fast func() (T, bool)) (T, bool) {
	start := time.Now()
	v, ok := fast()
	if !ok {
		if ShadowEnabled() && Enabled() && legacy != nil {
			go runFallbackShadow(op, path, legacy)
		}
		return v, false
	}
	goLatency := time.Since(start)
	shadowLogf("go-media %s used for %s: %.2fms", op, path, ms(goLatency))

	if !ShadowEnabled() || legacy == nil {
		return v, true
	}
	go runShadow(op, path, goLatency, v, legacy, diff)
	return v, true
}

// runFallbackShadow runs legacy after go-media returned no result. If ffmpeg
// succeeds, the go-media miss is recorded as an error regression.
func runFallbackShadow[T any](op, path string, legacy Legacy[T]) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("go-media shadow %s for %s panicked: %v", op, path, r)
		}
	}()
	legacyVal, ffOK := legacy()
	if !ffOK {
		return
	}
	// go-media produced no value, so the codecs come from the ffmpeg result.
	recordRegression(op, path, LabelError, "go-media returned no result while ffmpeg succeeded", describeCodecs(any(legacyVal)))
}

// runShadow times legacy, compares it with the go-media value, and logs the
// result. It runs off the request path, so a panic is recovered and logged.
func runShadow[T any](op, path string, goLatency time.Duration, fast T, legacy Legacy[T], diff Differ[T]) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("go-media shadow %s for %s panicked: %v", op, path, r)
		}
	}()
	ffStart := time.Now()
	legacyVal, ffOK := legacy()
	ffLatency := time.Since(ffStart)
	shadowLogf("go-media shadow %s for %s: go-media=%.2fms legacy(ffmpeg/dhowden)=%.2fms diff(go-legacy)=%+.2fms",
		op, path, ms(goLatency), ms(ffLatency), ms(goLatency)-ms(ffLatency))

	if !ffOK {
		// go-media read the file but the legacy path failed. That is an outcome
		// difference, but not a wrong value, so it is logged at info level.
		logger.Infof("go-media shadow %s for %s: legacy failed while go-media returned a result", op, path)
		return
	}
	if ffLatency >= regressionMinFfmpegLatency && float64(goLatency) > timeRegressionRatio*float64(ffLatency) {
		detail := fmt.Sprintf("go-media %.1fms vs ffmpeg %.1fms (+%.0f%%)",
			ms(goLatency), ms(ffLatency), (ms(goLatency)/ms(ffLatency)-1)*100)
		recordRegression(op, path, LabelTimeRegression, detail, describeCodecs(any(fast)))
	}
	var diffs []string
	if diff != nil {
		diffs = diff(fast, legacyVal)
	}
	var warnings []string
	for _, d := range diffs {
		// Entries prefixed "expected:" are known, harmless differences (for
		// example two backends choosing different frames). They are logged at
		// info level so real differences still produce warnings.
		if strings.HasPrefix(d, expectedDiffPrefix) {
			logger.Infof("go-media shadow %s for %s: %s", op, path, d)
			continue
		}
		warnings = append(warnings, d)
	}
	if len(warnings) > 0 {
		logger.Warningf("go-media shadow %s DIFFERS for %s: %s", op, path, strings.Join(warnings, "; "))
	}
}

// expectedDiffPrefix marks a difference that is expected between the backends.
const expectedDiffPrefix = "expected: "

// diffNum formats a numeric comparison for a shadow warning.
func diffNum(name string, fast, legacy float64, unit string) string {
	return fmt.Sprintf("%s go-media=%.4g%s ffmpeg=%.4g%s (delta %+.4g%s)", name, fast, unit, legacy, unit, fast-legacy, unit)
}

// ms converts a duration to fractional milliseconds.
func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// shadowLogf logs go-media timing at debug level (including dev shadow runs).
func shadowLogf(format string, args ...any) {
	logger.Debugf(format, args...)
}
