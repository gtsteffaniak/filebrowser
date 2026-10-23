package transcoding

import (
	"context"
	"sync"
)

// keyframeCacheMaxEntries bounds the in-memory keyframe index cache.
const keyframeCacheMaxEntries = 256

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
// Concurrent callers for the same file wait on one probe.
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
	times, err := probe(ctx)
	if err != nil {
		return nil, err
	}
	e.times, e.ok = times, true
	return times, nil
}
