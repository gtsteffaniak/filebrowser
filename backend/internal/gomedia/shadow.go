package gomedia

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
// time regression is recorded. 2 means +100%: go-media must take more than
// twice as long as ffmpeg.
const timeRegressionRatio = 2.0

// regressionMinFfmpegLatency skips timing ratios on runs too short to measure
// reliably.
const regressionMinFfmpegLatency = 5 * time.Millisecond

// Regression is one significant shadow-mode difference. It never holds a file
// path or name, only format and size information for diagnosis.
type Regression struct {
	CreatedAt time.Time
	// FileKey identifies the file: a SHA-256 of its path. The store keeps one
	// row per key, so a file never produces more than one entry.
	FileKey   string
	Operation string
	Label     string
	Detail    string
	Format    string
	SizeBytes int64
	Codecs    string
}

// regressionFileKey identifies a file without storing its path. The same path
// always maps to the same key.
func regressionFileKey(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])
}

// unsupportedMarks remembers, per op and path, that the go-media fast path
// reported the input as unsupported (by design). logFallbackTo and logUnsupported
// set a mark only while shadow mode is on. timed consumes it in the same call
// that produced the failure, so the shadow run can tell "go-media does not do
// this format" apart from "go-media broke".
var unsupportedMarks sync.Map

func unsupportedKey(op, path string) string { return op + "\x00" + path }

func markUnsupported(op, path string) {
	if ShadowEnabled() {
		unsupportedMarks.Store(unsupportedKey(op, path), struct{}{})
	}
}

func takeUnsupported(op, path string) bool {
	_, ok := unsupportedMarks.LoadAndDelete(unsupportedKey(op, path))
	return ok
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
		FileKey:   regressionFileKey(path),
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

// Incompatible is a file that go-media does not support and ffmpeg handled. Unlike
// Regression, it keeps the file path, so the list can show which files these are.
// It is only produced in shadow (dev) mode.
type Incompatible struct {
	CreatedAt time.Time
	FileKey   string
	Path      string
	Operation string
	Format    string
	SizeBytes int64
}

var incompatibleSink atomic.Pointer[func(Incompatible)]

// SetIncompatibleSink registers the function that stores incompatible files. Until
// it is set, they are not recorded.
func SetIncompatibleSink(fn func(Incompatible)) {
	if fn == nil {
		incompatibleSink.Store(nil)
		return
	}
	incompatibleSink.Store(&fn)
}

// recordIncompatible hands a file that go-media does not support, but ffmpeg did, to the sink.
func recordIncompatible(op, path string) {
	sink := incompatibleSink.Load()
	if sink == nil {
		return
	}
	r := Incompatible{
		CreatedAt: time.Now(),
		FileKey:   regressionFileKey(path),
		Path:      path,
		Operation: op,
		Format:    strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
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

// shadowFfmpegSem allows one shadow ffmpeg run at a time. Shadow ffmpeg work would
// otherwise compete with the next request's go-media decode and skew its timing.
var shadowFfmpegSem = make(chan struct{}, 1)

// maxShadowQueue bounds how many shadow comparisons may be admitted at once,
// running or waiting for shadowFfmpegSem. Without a bound, a burst of requests
// starts one goroutine each, and every waiting goroutine keeps its go-media
// value and legacy closure alive.
const maxShadowQueue = 32

// shadowAdmit holds one token per admitted shadow comparison.
var shadowAdmit = make(chan struct{}, maxShadowQueue)

// shadowInFlight counts admitted shadow comparisons that have not finished,
// including ones waiting for shadowFfmpegSem.
var shadowInFlight atomic.Int64

// spawnShadow runs run in the background when a queue slot is free. When the
// queue is full the comparison is skipped: the caller already has its result,
// and shadow diagnostics are best effort.
func spawnShadow(run func()) {
	select {
	case shadowAdmit <- struct{}{}:
	default:
		shadowLogf("go-media shadow skipped: %d comparisons already queued", maxShadowQueue)
		return
	}
	shadowInFlight.Add(1)
	go func() {
		defer func() {
			shadowInFlight.Add(-1)
			<-shadowAdmit
		}()
		run()
	}()
}

// runLegacy runs the ffmpeg side of a shadow comparison one at a time. It also
// returns how long legacy itself ran. The wait for shadowFfmpegSem is excluded,
// so a comparison queued behind another one does not look slow.
func runLegacy[T any](legacy Legacy[T]) (T, bool, time.Duration) {
	shadowFfmpegSem <- struct{}{}
	defer func() { <-shadowFfmpegSem }()
	start := time.Now()
	v, ok := legacy()
	return v, ok, time.Since(start)
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
	return timedOutcome(op, path, legacy, diff, func() (T, bool, bool) {
		v, ok := fast()
		// Consume the mark even when shadow is off, so it never outlives the call.
		return v, ok, !ok && takeUnsupported(op, path)
	})
}

// timedOutcome is timed for a fast path that reports its own unsupported outcome.
// unsupported is only meaningful when ok is false. It travels with the call, not
// through the shared mark map, so every caller that shares a fast result (for
// example through a singleflight group) sees the same outcome.
func timedOutcome[T any](op, path string, legacy Legacy[T], diff Differ[T], fast func() (v T, ok, unsupported bool)) (T, bool) {
	start := time.Now()
	// Shadow runs in flight when this go-media call starts. A regression recorded
	// with a non-zero load ran under contention and is marked as such.
	load := shadowInFlight.Load()
	v, ok, unsupported := fast()
	if !ok {
		if ShadowEnabled() && Enabled() && legacy != nil {
			spawnShadow(func() {
				runFallbackShadowOutcome(op, path, legacy, unsupported)
			})
		}
		return v, false
	}
	goLatency := time.Since(start)
	shadowLogf("go-media %s used for %s: %.2fms", op, path, ms(goLatency))

	if !ShadowEnabled() || legacy == nil {
		return v, true
	}
	spawnShadow(func() {
		runShadow(op, path, goLatency, v, legacy, diff, load)
	})
	return v, true
}

// runFallbackShadow runs legacy after go-media returned no result. If ffmpeg
// succeeds, the go-media miss is recorded as an error regression.
func runFallbackShadow[T any](op, path string, legacy Legacy[T]) {
	runFallbackShadowOutcome(op, path, legacy, false)
}

// runFallbackShadowOutcome is runFallbackShadow with the go-media outcome known.
// When go-media reported the input as unsupported (by design, not a fault), a
// successful ffmpeg run is expected and is logged at info level, not as a regression.
func runFallbackShadowOutcome[T any](op, path string, legacy Legacy[T], unsupported bool) {
	regressed := false
	ffOK := false
	defer func() {
		shadowStats.observe(op, shadowObservation{goOK: false, ffOK: ffOK, regressed: regressed})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("go-media shadow %s for %s panicked: %v", op, path, r)
		}
	}()
	var legacyVal T
	legacyVal, ffOK, _ = runLegacy(legacy)
	if !ffOK {
		return
	}
	if unsupported {
		logger.Infof("go-media shadow %s for %s: unsupported by go-media, ffmpeg handled it (expected)", op, path)
		recordIncompatible(op, path)
		return
	}
	// go-media produced no value, so the codecs come from the ffmpeg result.
	recordRegression(op, path, LabelError, "go-media returned no result while ffmpeg succeeded", describeCodecs(any(legacyVal)))
	regressed = true
}

// runShadow times legacy, compares it with the go-media value, and logs the
// result. It runs off the request path, so a panic is recovered and logged.
func runShadow[T any](op, path string, goLatency time.Duration, fast T, legacy Legacy[T], diff Differ[T], load int64) {
	regressed := false
	ffOK := false
	var ffLatency time.Duration
	// Closures, not direct arguments: the values are read when the run ends.
	defer func() {
		shadowStats.observe(op, shadowObservation{
			goOK: true, goLatency: goLatency, ffOK: ffOK, ffLatency: ffLatency, regressed: regressed,
		})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("go-media shadow %s for %s panicked: %v", op, path, r)
		}
	}()
	var legacyVal T
	legacyVal, ffOK, ffLatency = runLegacy(legacy)
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
		if load > 0 {
			detail += fmt.Sprintf(" [under load: %d shadow runs in flight at start]", load)
		}
		recordRegression(op, path, LabelTimeRegression, detail, describeCodecs(any(fast)))
		regressed = true
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
