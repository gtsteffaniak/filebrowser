package gomedia

import (
	"fmt"
	"strings"
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
// When fast returns ok=false the caller falls back to its own ffmpeg code as
// before, so legacy is not run here.
func timed[T any](op, path string, legacy Legacy[T], diff Differ[T], fast func() (T, bool)) (T, bool) {
	start := time.Now()
	v, ok := fast()
	if !ok {
		return v, false
	}
	goLatency := time.Since(start)
	logger.Debugf("go-media %s used for %s: %.2fms", op, path, ms(goLatency))

	if !ShadowEnabled() || legacy == nil {
		return v, true
	}
	go runShadow(op, path, goLatency, v, legacy, diff)
	return v, true
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
	logger.Debugf("go-media shadow %s for %s: go-media=%.2fms legacy(ffmpeg/dhowden)=%.2fms diff(legacy-go)=%+.2fms legacy_ok=%t",
		op, path, ms(goLatency), ms(ffLatency), ms(ffLatency)-ms(goLatency), ffOK)

	if !ffOK {
		// go-media read the file but the legacy path failed. That is an outcome
		// difference, but not a wrong value, so it is logged at info level.
		logger.Infof("go-media shadow %s for %s: legacy failed while go-media returned a result", op, path)
		return
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
