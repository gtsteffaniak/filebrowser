package ffmpeg

import "math"

const (
	hlsKeyframeMinGapSec      = 0.5
	hlsMinSegmentDurationSec  = 0.5 // ffmpeg cannot encode shorter on-demand segments reliably
	hlsKeyframeDensityFrac    = 0.45 // more than ~1 keyframe per 2.2s → likely corrupt index
)

// SanitizeHLSKeyframes filters spurious keyframe probes from corrupt indexes.
// Returns nil when keyframes are unusable so callers fall back to a fixed grid.
// Long GOPs (e.g. 20s+) are kept; stream copy requires cuts on real keyframes.
func SanitizeHLSKeyframes(keyframes []float64, durationSec float64) []float64 {
	if len(keyframes) == 0 || durationSec <= 0 {
		return nil
	}
	if float64(len(keyframes)) > durationSec*hlsKeyframeDensityFrac {
		return nil
	}

	out := make([]float64, 0, len(keyframes))
	for _, t := range keyframes {
		if t < 0 || t >= durationSec {
			continue
		}
		if len(out) == 0 {
			out = append(out, t)
			continue
		}
		if t-out[len(out)-1] < hlsKeyframeMinGapSec {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	if out[0] > 0.001 {
		out = append([]float64{0}, out...)
	}
	return out
}

// BuildHLSSegmentTimeline returns segment start times and durations in seconds.
// When keyframes are available each segment begins on a keyframe; otherwise a fixed grid is used.
func BuildHLSSegmentTimeline(durationSec float64, keyframes []float64) (starts, durations []float64) {
	if durationSec <= 0 {
		durationSec = HLSSegmentDurationSec
	}
	if len(keyframes) == 0 {
		return fixedHLSSegmentTimeline(durationSec)
	}

	kf := append([]float64(nil), keyframes...)
	if kf[0] > 0.001 {
		kf = append([]float64{0}, kf...)
	}

	for i := 0; i < len(kf); i++ {
		start := kf[i]
		if start >= durationSec {
			break
		}
		end := durationSec
		if i+1 < len(kf) {
			end = kf[i+1]
		}
		dur := end - start
		if dur <= 0.01 {
			continue
		}
		starts = append(starts, start)
		durations = append(durations, dur)
	}
	if len(starts) == 0 {
		return fixedHLSSegmentTimeline(durationSec)
	}
	return mergeTinyTailSegment(starts, durations)
}

func fixedHLSSegmentTimeline(durationSec float64) (starts, durations []float64) {
	count := int(math.Ceil(durationSec / HLSSegmentDurationSec))
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		start := float64(i) * HLSSegmentDurationSec
		dur := HLSSegmentDurationSec
		if rem := durationSec - start; rem > 0 && rem < dur {
			dur = rem
		}
		if dur <= 0 {
			break
		}
		starts = append(starts, start)
		durations = append(durations, dur)
	}
	return mergeTinyTailSegment(starts, durations)
}

// mergeTinyTailSegment folds a sub-minimum tail into the previous segment so ffmpeg
// can produce valid output (very short -t values often yield empty TS/fMP4).
func mergeTinyTailSegment(starts, durations []float64) ([]float64, []float64) {
	if len(durations) < 2 || durations[len(durations)-1] >= hlsMinSegmentDurationSec {
		return starts, durations
	}
	durations[len(durations)-2] += durations[len(durations)-1]
	return starts[:len(starts)-1], durations[:len(durations)-1]
}
