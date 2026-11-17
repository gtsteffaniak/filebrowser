// Package mediacache holds the media metadata caches shared by the ffmpeg and
// go-media code paths. Both backends read and write the same entries, so a
// value found by one is reused by the other instead of being probed again.
//
// Every entry is keyed by file identity: path, size, and modification time in
// nanoseconds. An edited file gets a new key, so a stale value is never served
// for it; the old entry ages out after TTL.
package mediacache

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-cache/cache"
)

// TTL is how long a cached media value is kept after it is stored.
const TTL = 24 * time.Hour

// ProbeInfo is the container summary both backends produce: ffprobe through
// go-ffmpeg, and go-media. FormatName uses ffprobe's format_name strings.
type ProbeInfo struct {
	Duration   float64
	VideoCodec string
	AudioCodec string
	FormatName string
}

var (
	// Probes caches container probes (duration, first video and audio codec,
	// format name).
	Probes = cache.NewCache[ProbeInfo](TTL)
	// Durations caches media length in seconds.
	Durations = cache.NewCache[float64](TTL)
	// SubtitleTracks caches the embedded subtitle track list of a video.
	SubtitleTracks = cache.NewCache[[]utils.SubtitleTrack](TTL)
	// SubtitleContent caches one embedded subtitle stream converted to WebVTT.
	// Use Key.Stream for its key.
	SubtitleContent = cache.NewCache[string](TTL)
)

// Key identifies one version of a file. The zero Key is never produced for a
// real file, because Path is always set.
type Key struct {
	Path    string
	Size    int64
	ModTime int64 // modification time in Unix nanoseconds
}

// FromFileInfo returns the Key for path using an already-read FileInfo, so
// callers that stat or open the file anyway do not stat it twice.
func FromFileInfo(path string, info fs.FileInfo) Key {
	return Key{Path: path, Size: info.Size(), ModTime: info.ModTime().UnixNano()}
}

// Stat returns the Key for path. It follows symlinks, like os.Stat.
func Stat(path string) (Key, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Key{}, err
	}
	return FromFileInfo(path, info), nil
}

// String returns the cache key for k. The path is quoted so a path containing
// the separator cannot collide with another file's key.
func (k Key) String() string {
	return fmt.Sprintf("%q:%d:%d", k.Path, k.Size, k.ModTime)
}

// Stream returns the cache key for one stream of k, used by SubtitleContent.
func (k Key) Stream(index int) string {
	return fmt.Sprintf("%s:%d", k.String(), index)
}

// ClearAll drops every cached media value. It is used when the backend that
// produced the values is switched (the admin "Always use ffmpeg" setting), so
// the newly selected backend answers fresh.
func ClearAll() {
	Probes.ClearAll()
	Durations.ClearAll()
	SubtitleTracks.ClearAll()
	SubtitleContent.ClearAll()
}

// bypassKey is the context key set by WithoutCache.
type bypassKey struct{}

// WithoutCache returns a context under which the ffmpeg code paths neither read
// nor write the shared caches. Dev-mode shadow runs use it, so the ffmpeg result
// they compare against go-media is computed fresh rather than read back from the
// value go-media just stored.
func WithoutCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassKey{}, true)
}

// Bypassed reports whether ctx was returned by WithoutCache. A nil context is
// not bypassed.
func Bypassed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(bypassKey{}).(bool)
	return v
}
