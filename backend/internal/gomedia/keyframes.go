package gomedia

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/gtsteffaniak/go-media/pkg/media"
)

// keyframeTimeDedupeSec matches go-ffmpeg's ffprobe keyframe probe, which
// drops keyframes within 1 ms of the previous one.
const keyframeTimeDedupeSec = 0.001

// keyframeShadowTolerance is how far apart two backends' keyframe times can be
// before the shadow comparison reports them. ffprobe prints six decimals.
const keyframeShadowTolerance = 0.002

// KeyframeTimes returns the presentation times, in seconds, of the first video
// stream's keyframes, for HLS segment planning. The result is sorted, has no
// negative times, and drops keyframes within 1 ms of the previous one, the same
// as go-ffmpeg's ffprobe probe. go-media reads only the container index (MP4
// stss/stts/ctts, Matroska Cues or block headers), so it is much faster than
// demuxing every packet with ffprobe.
//
// ok is false when go-media is disabled, the container is not one go-media
// indexes, or the read fails; the caller then runs ffprobe. ctx cancels the
// file reads. legacy is the ffprobe probe, run only in shadow mode.
//
// go-media can report fewer keyframes than ffprobe (ffprobe's parsers also flag
// IDR frames the container index omits, and some Matroska muxers index only
// part of the keyframes in Cues). It never reports a keyframe ffprobe would not.
func KeyframeTimes(ctx context.Context, path string, legacy Legacy[[]float64]) ([]float64, bool) {
	return timed("keyframe index", path, legacy, keyframeDiff, func() ([]float64, bool) {
		return keyframeTimesFast(ctx, path, 0)
	})
}

// KeyframeTimesWithin is KeyframeTimes limited to keyframes within windowSec
// seconds of the first keyframe. It is the pre-flight check that can rule out
// stream copy without reading the whole index (Matroska files without Cues).
// It has no shadow comparison.
func KeyframeTimesWithin(ctx context.Context, path string, windowSec float64) ([]float64, bool) {
	return timed("keyframe preflight", path, nil, nil, func() ([]float64, bool) {
		return keyframeTimesFast(ctx, path, windowSec)
	})
}

// keyframeTimesFast reads the keyframe index with go-media. windowSec > 0
// limits it to keyframes within that many seconds of the first one.
func keyframeTimesFast(ctx context.Context, path string, windowSec float64) ([]float64, bool) {
	if !Enabled() {
		return nil, false
	}
	if !supportsVideoContainer(path) {
		return nil, false
	}
	if err := ctx.Err(); err != nil {
		return nil, false
	}
	f, size, err := openFile(path)
	if err != nil {
		logFallback("keyframe index", path, err)
		return nil, false
	}
	defer f.Close()

	r := ctxReaderAt{ctx: ctx, r: f}
	var times []float64
	if windowSec > 0 {
		times, err = media.KeyframeTimesWithin(r, size, windowSec)
	} else {
		times, err = media.KeyframeTimes(r, size)
	}
	if err != nil {
		if ctx.Err() == nil {
			logFallback("keyframe index", path, err)
		}
		return nil, false
	}
	times = dedupeKeyframeTimes(times)
	if len(times) == 0 {
		logUnsupported("keyframe index", path, "no keyframes in container index")
		return nil, false
	}
	return times, true
}

// dedupeKeyframeTimes sorts times, drops negative values, and drops times
// within keyframeTimeDedupeSec of the previous kept time, as go-ffmpeg does for
// ffprobe output.
func dedupeKeyframeTimes(times []float64) []float64 {
	sorted := append([]float64(nil), times...)
	sort.Float64s(sorted)
	out := make([]float64, 0, len(sorted))
	for _, t := range sorted {
		if t < 0 || math.IsNaN(t) || math.IsInf(t, 0) {
			continue
		}
		if len(out) == 0 || t > out[len(out)-1]+keyframeTimeDedupeSec {
			out = append(out, t)
		}
	}
	return out
}

// ctxReaderAt fails reads once ctx is done, so a cancelled request or the plan
// timeout stops a long index walk on a slow share.
type ctxReaderAt struct {
	ctx context.Context
	r   io.ReaderAt
}

// ReadAt reads from the wrapped reader unless the context is done.
func (c ctxReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.ReadAt(p, off)
}

// keyframeDiff compares go-media's keyframe index with ffprobe's. go-media
// reporting a subset of ffprobe's keyframes is expected (parser-flagged IDR
// frames, partial Cues) and is logged at info level.
func keyframeDiff(fast, legacy []float64) []string {
	missing := 0
	j := 0
	for _, t := range fast {
		for j < len(legacy) && legacy[j] < t-keyframeShadowTolerance {
			j++
			missing++
		}
		if j >= len(legacy) || math.Abs(legacy[j]-t) > keyframeShadowTolerance {
			return []string{fmt.Sprintf("keyframe at %.6fs from go-media not in ffmpeg index (go-media=%d ffmpeg=%d keyframes)", t, len(fast), len(legacy))}
		}
		j++
	}
	missing += len(legacy) - j
	if missing > 0 {
		return []string{fmt.Sprintf("%sgo-media found %d of %d ffmpeg keyframes (keyframes outside the container index)", expectedDiffPrefix, len(fast), len(legacy))}
	}
	return nil
}
