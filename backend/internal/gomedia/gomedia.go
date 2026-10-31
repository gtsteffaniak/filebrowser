// Package gomedia is the go-media fast path for probing, duration, tags, lyrics,
// embedded subtitles, and video thumbnails.
//
// Every function returns ok=false when the caller must use the existing ffmpeg
// (or dhowden) code instead. That happens when:
//   - the admin "Always use ffmpeg" setting is on (no log, go-media is skipped);
//   - go-media reports the input as unsupported (logged as a warning);
//   - go-media fails for any other reason (logged as an error).
//
// Callers never resize or crop after go-media. Output sizes are requested from
// go-media directly.
package gomedia

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/gtsteffaniak/go-media/pkg/media"
	"golang.org/x/sync/singleflight"
)

var (
	subtitleListCache    = cache.NewCache[[]utils.SubtitleTrack](24 * time.Hour)
	subtitleContentCache = cache.NewCache[string](24 * time.Hour)

	// probeCache caches successful probe results by file identity (path, modtime,
	// size). A directory listing probes every video child, and the preview flow
	// probes the focused file again; without this each request re-read the same
	// containers. Keying on modtime+size means an edited file is re-probed.
	probeCache = cache.NewCache[ProbeResult](24 * time.Hour)

	// probeGroup coalesces concurrent probes of the same file so a burst of
	// requests for one container reads it once.
	probeGroup singleflight.Group
)

// Enabled reports whether the go-media fast path may run. The admin
// "Always use ffmpeg" setting turns it off.
func Enabled() bool {
	return !settings.AlwaysUseFfmpegEnabled()
}

// videoDimensions returns the first video stream's width and height.
func videoDimensions(info *media.ProbeInfo) (w, h int, ok bool) {
	for _, s := range info.Streams {
		if s.Type == media.StreamVideo && s.Width > 0 && s.Height > 0 {
			return s.Width, s.Height, true
		}
	}
	return 0, 0, false
}

// videoContainerExts are the containers go-media parses for video probes,
// thumbnails, and embedded subtitles. Other video containers (AVI, WMV, TS, ...)
// are not go-media targets, so they go straight to ffmpeg with no file open and
// no log line. go-media reads MOV as MP4.
var videoContainerExts = map[string]bool{
	".mp4":  true,
	".m4v":  true,
	".mov":  true,
	".mkv":  true,
	".webm": true,
}

// supportsVideoContainer reports whether go-media targets path's container.
func supportsVideoContainer(path string) bool {
	return videoContainerExts[strings.ToLower(filepath.Ext(path))]
}

// logFallback records why go-media was not used for op on path.
func logFallback(op, path string, err error) {
	// ErrNotMedia means the file is not a container go-media reads. That is an
	// expected fallback, not a failure, so it logs as a warning.
	if errors.Is(err, media.ErrUnsupported) || errors.Is(err, media.ErrUnsupportedProfile) || errors.Is(err, media.ErrNotMedia) {
		logger.Warningf("go-media does not support %s for %s (%v); using ffmpeg", op, path, err)
		return
	}
	logger.Errorf("go-media %s failed for %s: %v; falling back to ffmpeg", op, path, err)
}

// logUnsupported records a container or codec that go-media does not handle.
func logUnsupported(op, path, reason string) {
	logger.Warningf("go-media does not support %s for %s (%s); using ffmpeg", op, path, reason)
}

// openFile opens path for go-media, which reads through io.ReaderAt.
func openFile(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

// ffprobeFormatNames maps go-media containers to the format_name strings ffprobe
// reports, so the stored container value does not change for existing users.
// go-media appends the ftyp brand to MP4 names (for example ",isom"), which
// ffprobe does not include, so MP4 is normalized as well.
var ffprobeFormatNames = map[media.Format]string{
	media.FormatMP4:  "mov,mp4,m4a,3gp,3g2,mj2",
	media.FormatMKV:  "matroska,webm",
	media.FormatWebM: "matroska,webm",
	media.FormatWAV:  "wav",
	media.FormatFLAC: "flac",
	media.FormatOGG:  "ogg",
	media.FormatMP3:  "mp3",
	media.FormatAIFF: "aiff",
	media.FormatAU:   "au",
}

// ProbeResult is the same shape as the ffprobe result, so callers can use one
// code path for both backends.
type ProbeResult = ffmpeg.FileProbeInfo

// Probe returns duration, codecs, and container name for a local media file.
// legacy is the ffmpeg probe, run only in shadow mode.
func Probe(path string, legacy Legacy[ProbeResult]) (ProbeResult, bool) {
	return timed("probe", path, legacy, probeDiff, func() (ProbeResult, bool) { return probeFast(path) })
}

func probeFast(path string) (ProbeResult, bool) {
	if !Enabled() {
		return ProbeResult{}, false
	}
	if !supportsVideoContainer(path) {
		return ProbeResult{}, false
	}

	f, size, err := openFile(path)
	if err != nil {
		logFallback("probe", path, err)
		return ProbeResult{}, false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		logFallback("probe", path, err)
		return ProbeResult{}, false
	}
	cacheKey := probeCacheKey(path, info.ModTime().UnixNano(), size)
	if res, ok := probeCache.Get(cacheKey); ok {
		return res, true
	}

	// Coalesce concurrent probes of the same file into one container read.
	v, err, _ := probeGroup.Do(cacheKey, func() (any, error) {
		res, ok := parseProbe(path, f, size)
		if !ok {
			return nil, errProbeUnsupported
		}
		probeCache.Set(cacheKey, res)
		return res, nil
	})
	if err != nil {
		return ProbeResult{}, false
	}
	res, ok := v.(ProbeResult)
	if !ok {
		return ProbeResult{}, false
	}
	return res, true
}

// errProbeUnsupported marks a probe that go-media chose not to answer. It carries
// no detail because parseProbe already logged the specific reason.
var errProbeUnsupported = errors.New("probe unsupported")

// probeCacheKey identifies a file version for caching: path plus modtime and size.
// An edited file gets a new key, so stale metadata is never served.
func probeCacheKey(path string, modTimeNanos, size int64) string {
	return fmt.Sprintf("probe:%s:%d:%d", path, modTimeNanos, size)
}

// parseProbe reads a container and builds a ProbeResult. It reports false when
// go-media should defer to ffmpeg, having logged why.
func parseProbe(path string, f *os.File, size int64) (ProbeResult, bool) {
	info, err := media.Probe(f, size)
	if err != nil {
		logFallback("probe", path, err)
		return ProbeResult{}, false
	}

	if info.Duration <= 0 {
		// Fragmented or unusual files can report no length. ffprobe answers instead,
		// the same as durationFast does.
		logUnsupported("probe", path, "duration not in container header")
		return ProbeResult{}, false
	}
	res := ProbeResult{Duration: info.Duration.Seconds(), FormatName: info.FormatName}
	if name, ok := ffprobeFormatNames[info.Format]; ok {
		res.FormatName = name
	}
	for _, s := range info.Streams {
		switch s.Type {
		case media.StreamVideo:
			if s.Codec == "" {
				// The UI would show no codec, so let ffprobe answer instead.
				logUnsupported("probe", path, "video codec not identified")
				return ProbeResult{}, false
			}
			if res.VideoCodec == "" {
				res.VideoCodec = s.Codec
			}
		case media.StreamAudio:
			if s.Codec == "" {
				logUnsupported("probe", path, "audio codec not identified")
				return ProbeResult{}, false
			}
			if res.AudioCodec == "" {
				res.AudioCodec = s.Codec
			}
		}
	}
	return res, true
}

// Duration returns the media length in seconds from container headers.
// go-media may report an estimate (MP3 without a VBR header); the source is not
// surfaced to callers.
// legacy is the ffmpeg duration probe, run only in shadow mode.
func Duration(path string, legacy Legacy[float64]) (float64, bool) {
	return timed("duration", path, legacy, durationDiff, func() (float64, bool) { return durationFast(path) })
}

func durationFast(path string) (float64, bool) {
	if !Enabled() {
		return 0, false
	}
	f, size, err := openFile(path)
	if err != nil {
		logFallback("duration", path, err)
		return 0, false
	}
	defer f.Close()

	res, err := media.Duration(f, size)
	if err != nil {
		logFallback("duration", path, err)
		return 0, false
	}
	if res.Duration <= 0 {
		// Unknown duration is not an error; ffmpeg may still find one.
		return 0, false
	}
	return res.Duration.Seconds(), true
}

// AudioInfo is the tag, lyrics availability, and cover art FileBrowser reads
// from an audio file.
type AudioInfo struct {
	Tags      media.Tags
	HasLyrics bool
	Artwork   []byte
}

// Audio reads tags and lyrics availability. Artwork is read only when withArt
// is true. A missing cover is not an error.
// legacy is the dhowden tag read, run only in shadow mode.
func Audio(path string, withArt bool, legacy Legacy[AudioInfo]) (AudioInfo, bool) {
	return timed("tags", path, legacy, audioDiffer(withArt), func() (AudioInfo, bool) { return audioFast(path, withArt) })
}

func audioFast(path string, withArt bool) (AudioInfo, bool) {
	if !Enabled() {
		return AudioInfo{}, false
	}
	f, size, err := openFile(path)
	if err != nil {
		logFallback("tags", path, err)
		return AudioInfo{}, false
	}
	defer f.Close()

	// One container parse gives tags, the embedded lyrics flag, and cover bytes.
	// Separate ReadTags/HasLyrics/EmbeddedArtwork calls each re-parsed the file.
	audio, err := media.InspectAudio(f, size, media.AudioOptions{WithArtwork: withArt})
	if err != nil {
		logFallback("tags", path, err)
		return AudioInfo{}, false
	}
	info := AudioInfo{Tags: audio.Tags, HasLyrics: audio.HasEmbeddedLyrics}
	if !info.HasLyrics {
		// Sidecar lyrics are only checked when the file has no embedded lyrics.
		_, _, info.HasLyrics = media.LyricsSidecar(path)
	}
	if withArt {
		info.Artwork = audio.Artwork
	}
	return info, true
}

// Lyrics returns lyrics from a sidecar file, or else from embedded tags. The
// format is "embedded" for tags or the sidecar extension. An empty result with
// ok=true means go-media checked and found no lyrics.
// legacy is the sidecar and dhowden lyrics read, run only in shadow mode.
func Lyrics(path string, legacy Legacy[LyricsResult]) (text, format string, ok bool) {
	res, ok := timed("lyrics", path, legacy, lyricsDiff, func() (LyricsResult, bool) {
		t, f, found := lyricsFast(path)
		return LyricsResult{Text: t, Format: f}, found
	})
	return res.Text, res.Format, ok
}

func lyricsFast(path string) (text, format string, ok bool) {
	if !Enabled() {
		return "", "", false
	}
	if sidecarText, ext, found := media.LyricsSidecar(path); found {
		return sidecarText, ext, true
	}
	f, size, err := openFile(path)
	if err != nil {
		logFallback("lyrics", path, err)
		return "", "", false
	}
	defer f.Close()

	// InspectAudio puts embedded lyrics in Tags.Raw["lyrics"] from the same parse.
	audio, err := media.InspectAudio(f, size, media.AudioOptions{})
	if err != nil {
		logFallback("lyrics", path, err)
		return "", "", false
	}
	text = audio.Tags.Raw["lyrics"]
	if text == "" {
		return "", "", false
	}
	return text, "embedded", true
}

// EmbeddedSubtitles lists embedded subtitle tracks. go-media supports MKV, WebM,
// and MP4 only. Other containers are reported as unsupported, not as "no
// subtitles", because go-media returns an empty list for them.
// legacy is the ffmpeg subtitle detection, run only in shadow mode.
func EmbeddedSubtitles(path string, modtime time.Time, legacy Legacy[[]utils.SubtitleTrack]) ([]utils.SubtitleTrack, bool) {
	return timed("subtitle list", path, legacy, subtitleListDiff, func() ([]utils.SubtitleTrack, bool) {
		return embeddedSubtitlesFast(path, modtime)
	})
}

func embeddedSubtitlesFast(path string, modtime time.Time) ([]utils.SubtitleTrack, bool) {
	if !Enabled() {
		return nil, false
	}
	if !supportsVideoContainer(path) {
		return nil, false
	}
	key := fmt.Sprintf("embedded_subtitles:%s:%d", path, modtime.Unix())
	if cached, ok := subtitleListCache.Get(key); ok {
		return cached, true
	}

	f, size, err := openFile(path)
	if err != nil {
		logFallback("subtitle list", path, err)
		return nil, false
	}
	defer f.Close()

	info, err := media.Probe(f, size)
	if err != nil {
		logFallback("subtitle list", path, err)
		return nil, false
	}
	switch info.Format {
	case media.FormatMKV, media.FormatWebM, media.FormatMP4:
	default:
		logUnsupported("subtitle list", path, "container "+string(info.Format))
		return nil, false
	}

	streams := make([]ffmpeg.SubtitleStream, 0, len(info.SubtitleTracks))
	for _, t := range info.SubtitleTracks {
		streams = append(streams, ffmpeg.SubtitleStream{
			Index:    t.Index,
			Codec:    t.Codec,
			Language: t.Language,
			Title:    t.Title,
		})
	}
	tracks := ffmpeg.BuildSubtitleTracks(streams)
	subtitleListCache.Set(key, tracks)
	return tracks, true
}

// ExtractSubtitle returns the content of an embedded subtitle stream as WebVTT.
// streamIndex is the same index EmbeddedSubtitles reports.
// legacy is the ffmpeg subtitle extraction, run only in shadow mode.
func ExtractSubtitle(path string, streamIndex int, legacy Legacy[string]) (string, bool) {
	return timed("subtitle extract", path, legacy, subtitleExtractDiff, func() (string, bool) {
		return extractSubtitleFast(path, streamIndex)
	})
}

func extractSubtitleFast(path string, streamIndex int) (string, bool) {
	if !Enabled() {
		return "", false
	}
	if !supportsVideoContainer(path) {
		return "", false
	}
	st, err := os.Stat(path)
	if err != nil {
		logFallback("subtitle extract", path, err)
		return "", false
	}
	key := fmt.Sprintf("subtitle_content:%s:%d:%d", path, streamIndex, st.ModTime().Unix())
	if content, ok := subtitleContentCache.Get(key); ok {
		return content, true
	}

	f, size, err := openFile(path)
	if err != nil {
		logFallback("subtitle extract", path, err)
		return "", false
	}
	defer f.Close()

	vtt, err := media.ExtractEmbeddedSubtitle(f, size, streamIndex)
	if err != nil {
		logFallback("subtitle extract", path, err)
		return "", false
	}
	if strings.TrimSpace(vtt) == "" {
		logFallback("subtitle extract", path, errors.New("empty subtitle output"))
		return "", false
	}
	subtitleContentCache.Set(key, vtt)
	return vtt, true
}

// VideoThumbnail returns a JPEG frame at seekPercent that fits inside
// width x height with aspect ratio kept. Crop is never applied. Quality is JPEG
// quality 1-100.
//
// A zero width or height means "original size". go-media defaults a zero bound
// to 320px, so the source's own dimensions are passed as the bound instead.
// Then no scaling happens in go-media either.
// legacy is the ffmpeg preview frame, run only in shadow mode.
func VideoThumbnail(path string, width, height, quality int, seekPercent float64, legacy Legacy[[]byte]) ([]byte, bool) {
	return timed("video thumbnail", path, legacy, thumbnailDiff, func() ([]byte, bool) {
		return videoThumbnailFast(path, width, height, quality, seekPercent)
	})
}

func videoThumbnailFast(path string, width, height, quality int, seekPercent float64) ([]byte, bool) {
	if !Enabled() {
		return nil, false
	}
	if !supportsVideoContainer(path) {
		return nil, false
	}
	f, size, err := openFile(path)
	if err != nil {
		logFallback("video thumbnail", path, err)
		return nil, false
	}
	defer f.Close()

	if width <= 0 || height <= 0 {
		info, probeErr := media.Probe(f, size)
		if probeErr != nil {
			logFallback("video thumbnail", path, probeErr)
			return nil, false
		}
		srcW, srcH, found := videoDimensions(info)
		if !found {
			logUnsupported("video thumbnail", path, "source dimensions not found")
			return nil, false
		}
		if width <= 0 {
			width = srcW
		}
		if height <= 0 {
			height = srcH
		}
	}

	res, err := media.Thumbnail(f, size, media.ThumbnailOptions{
		MaxWidth:    width,
		MaxHeight:   height,
		SeekPercent: seekPercent,
		Format:      "jpeg",
		Quality:     quality,
	})
	if err != nil {
		logFallback("video thumbnail", path, err)
		return nil, false
	}
	if len(res.Data) == 0 {
		logFallback("video thumbnail", path, errors.New("empty thumbnail output"))
		return nil, false
	}
	return res.Data, true
}

// SidecarSubtitleExts are the sidecar formats go-media lists and converts.
// Other subtitle extensions in iteminfo.SubtitleExts keep the directory scan.
var SidecarSubtitleExts = map[string]bool{
	".srt": true,
	".vtt": true,
	".ass": true,
	".ssa": true,
}

// ExternalSubtitles lists sidecar subtitles next to videoName, for the formats in
// SidecarSubtitleExts, using go-media. siblingNames are the other file names in
// the same directory. legacy is the directory scan, run only in shadow mode.
func ExternalSubtitles(videoName string, siblingNames []string, legacy Legacy[[]utils.SubtitleTrack]) ([]utils.SubtitleTrack, bool) {
	return timed("external subtitle list", videoName, legacy, subtitleListDiff, func() ([]utils.SubtitleTrack, bool) {
		return externalSubtitlesFast(videoName, siblingNames)
	})
}

func externalSubtitlesFast(videoName string, siblingNames []string) ([]utils.SubtitleTrack, bool) {
	if !Enabled() {
		return nil, false
	}
	tracks := media.ExternalSubtitleTracks(videoName, siblingNames)
	out := make([]utils.SubtitleTrack, 0, len(tracks))
	for _, t := range tracks {
		if !SidecarSubtitleExts["."+t.Codec] {
			continue
		}
		out = append(out, utils.SubtitleTrack{Name: t.Name, Language: t.Language, Codec: t.Codec})
	}
	return out, true
}
