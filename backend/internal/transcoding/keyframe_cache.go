package transcoding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// keyframeCacheMaxEntries bounds the in-memory keyframe index cache.
const keyframeCacheMaxEntries = 256

// keyframeBoundedPreflightWindowSec is how much of a file's head the pre-flight
// keyframe probe demuxes. It only needs to be long enough to expose a GOP
// longer than the copy cap (>= 12s), so twice the cap covers a single whole
// GOP plus slack. The full-file probe still runs when the window does not
// already rule stream copy out.
const keyframeBoundedPreflightWindowSec = 24.0

// keyframeDiskCacheVersion invalidates persisted indexes when the probe format
// or the sanity rules around it change.
const keyframeDiskCacheVersion = 1

// keyframeKey identifies a file version; a changed size or mtime misses.
type keyframeKey struct {
	path    string
	size    int64
	modTime int64
}

type keyframeEntry struct {
	mu    sync.Mutex
	ok    bool
	times []float64
}

// keyframeCache memoizes keyframe probes. The probe demuxes every video packet,
// which takes tens of seconds on a large file over a network share, so
// repeated plan builds (profile switches, restarts) must not repeat it.
// Concurrent callers for the same file wait on one probe. Successful probes are
// also written to disk so a server restart does not re-pay for the same file.
type keyframeCache struct {
	mu      sync.Mutex
	entries map[keyframeKey]*keyframeEntry
}

var defaultKeyframeCache = &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}

func (c *keyframeCache) get(ctx context.Context, key keyframeKey, probe func(context.Context) ([]float64, error)) ([]float64, error) {
	c.mu.Lock()
	e := c.entries[key]
	if e == nil {
		if len(c.entries) >= keyframeCacheMaxEntries {
			for k := range c.entries {
				delete(c.entries, k)
				break
			}
		}
		e = &keyframeEntry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ok {
		return e.times, nil
	}
	if times, ok := readKeyframeDiskCache(key); ok {
		e.times, e.ok = times, true
		return times, nil
	}
	times, err := probe(ctx)
	if err != nil {
		return nil, err
	}
	e.times, e.ok = times, true
	writeKeyframeDiskCache(key, times)
	return times, nil
}

// keyframeDiskStamp is the persisted index plus the file identity it was built
// from. The identity is re-checked on read so a path reused for a different
// file cannot serve a stale timeline.
type keyframeDiskStamp struct {
	Version int       `json:"version"`
	Size    int64     `json:"size"`
	ModTime int64     `json:"modTime"`
	Times   []float64 `json:"times"`
}

// keyframeDiskCacheEnabled reports whether a cache dir is configured. Without
// one the fallback path would be a relative directory next to the process CWD,
// so persistence is skipped rather than polluting the working tree.
func keyframeDiskCacheEnabled() bool {
	return settings.Config.Server.CacheDir != ""
}

func keyframeDiskPath(key keyframeKey) string {
	sum := sha256.Sum256([]byte(key.path))
	name := hex.EncodeToString(sum[:])[:32] + ".json"
	return filepath.Join(settings.TranscodeKeyframeCacheDir(), name)
}

func readKeyframeDiskCache(key keyframeKey) ([]float64, bool) {
	if !keyframeDiskCacheEnabled() {
		return nil, false
	}
	data, err := os.ReadFile(keyframeDiskPath(key))
	if err != nil {
		return nil, false
	}
	var stamp keyframeDiskStamp
	if err := json.Unmarshal(data, &stamp); err != nil {
		return nil, false
	}
	if stamp.Version != keyframeDiskCacheVersion || stamp.Size != key.size || stamp.ModTime != key.modTime {
		return nil, false
	}
	if len(stamp.Times) == 0 {
		return nil, false
	}
	return stamp.Times, true
}

func writeKeyframeDiskCache(key keyframeKey, times []float64) {
	if len(times) == 0 || !keyframeDiskCacheEnabled() {
		return
	}
	dir := settings.TranscodeKeyframeCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Debugf("transcode: keyframe cache dir unavailable: %v", err)
		return
	}
	data, err := json.Marshal(keyframeDiskStamp{
		Version: keyframeDiskCacheVersion,
		Size:    key.size,
		ModTime: key.modTime,
		Times:   times,
	})
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "kf-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return
	}
	// Atomic publish: a torn write must never be read back as a valid index.
	if err := os.Rename(tmpName, keyframeDiskPath(key)); err != nil {
		os.Remove(tmpName)
	}
}

// keyframesExceedCopyCap reports whether the probed keyframe timeline contains a
// gap longer than the copy segment cap, which makes stream copy impossible for a
// VOD playlist (ffmpeg would emit a multi-minute #EXTINF segment).
func keyframesExceedCopyCap(keyframes []float64, segmentSec float64) bool {
	if len(keyframes) < 2 {
		return false
	}
	cap := maxCopySegmentDurationSec(segmentSec)
	prev := keyframes[0]
	for _, t := range keyframes[1:] {
		if t-prev > cap {
			return true
		}
		prev = t
	}
	return false
}

// keyframesWithPreflight decides a file's keyframe timeline from a bounded
// pre-flight probe and a full probe. The window only short-circuits toward
// transcode (the fail-safe direction): when it already contains a gap past the
// copy cap the full probe is skipped and nil is returned, since the VOD plan
// only needs the resulting "needs transcode" bit. When the window does not
// already rule copy out, the full-file probe runs — the plan needs every
// keyframe to build the segment grid.
func keyframesWithPreflight(boundedProbe, fullProbe func() ([]float64, error), segmentSec float64) ([]float64, error) {
	bounded, err := boundedProbe()
	if err == nil && keyframesExceedCopyCap(bounded, segmentSec) {
		return nil, errCopyImpossibleByPreflight
	}
	return fullProbe()
}

// errCopyImpossibleByPreflight marks the bounded probe short-circuit. The
// caller treats it the same as "no keyframes": force transcode, do not fail.
var errCopyImpossibleByPreflight = fmt.Errorf("pre-flight keyframe gap exceeds copy cap")

// probeKeyframesWithPreflight is keyframesWithPreflight bound to the ffmpeg
// service for a specific file.
func probeKeyframesWithPreflight(ctx context.Context, svc *ffmpeg.Service, realPath string, segmentSec float64) ([]float64, error) {
	return keyframesWithPreflight(
		func() ([]float64, error) {
			return svc.ProbeVideoKeyframeTimesBounded(ctx, realPath, keyframeBoundedPreflightWindowSec)
		},
		func() ([]float64, error) { return svc.ProbeVideoKeyframeTimes(ctx, realPath) },
		segmentSec,
	)
}

// keyframeProbeTimeout bounds a single full-file keyframe probe. The default
// demux has no deadline of its own, so a slow network share would otherwise
// stall plan building (and the client's session POST) indefinitely.
const keyframeProbeTimeout = 2 * time.Minute

func probeKeyframesCached(ctx context.Context, svc *ffmpeg.Service, realPath string, size, modTime int64, segmentSec float64, preflight bool) ([]float64, error) {
	key := keyframeKey{path: realPath, size: size, modTime: modTime}
	return defaultKeyframeCache.get(ctx, key, func(ctx context.Context) ([]float64, error) {
		probeCtx, cancel := context.WithTimeout(ctx, keyframeProbeTimeout)
		defer cancel()
		if preflight {
			return probeKeyframesWithPreflight(probeCtx, svc, realPath, segmentSec)
		}
		return svc.ProbeVideoKeyframeTimes(probeCtx, realPath)
	})
}
