package ffmpeg

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/mediacache"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

// DetectEmbeddedSubtitles detects embedded subtitle streams. Results are stored
// in the shared media cache, keyed by the file's current size and modification
// time; modtime is kept for API compatibility and is not used for the key.
func DetectEmbeddedSubtitles(videoPath string, modtime time.Time) []utils.SubtitleTrack {
	_ = modtime
	return DetectEmbeddedSubtitlesContext(context.Background(), videoPath)
}

// DetectEmbeddedSubtitlesContext is DetectEmbeddedSubtitles with a context. The
// shared media cache is skipped when ctx is mediacache.WithoutCache.
func DetectEmbeddedSubtitlesContext(ctx context.Context, videoPath string) []utils.SubtitleTrack {
	useCache := !mediacache.Bypassed(ctx)
	key, statErr := mediacache.Stat(videoPath)
	useCache = useCache && statErr == nil
	if useCache {
		if cached, ok := mediacache.SubtitleTracks.Get(key.String()); ok {
			return cached
		}
	}

	svc := Get()
	if svc == nil || svc.inner == nil {
		return nil
	}

	tracks, err := svc.inner.DetectSubtitles(ctx, videoPath)
	if err != nil {
		logger.Debug("ffprobe failed for file: " + videoPath + ", error: " + err.Error())
		return nil
	}

	subtitles := mapSubtitleTracks(tracks)
	if useCache {
		mediacache.SubtitleTracks.Set(key.String(), subtitles)
	}
	return subtitles
}

// SubtitleStream is one embedded subtitle stream, independent of the probe
// backend (ffprobe via go-ffmpeg, or go-media).
type SubtitleStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
}

// BuildSubtitleTracks maps embedded subtitle streams to track entries. Shared by
// the ffmpeg and go-media paths so both produce identical track lists.
func BuildSubtitleTracks(streams []SubtitleStream) []utils.SubtitleTrack {
	subtitles := make([]utils.SubtitleTrack, 0, len(streams))
	mapper := &subtitleTrackMapper{
		seenLang: make(map[string]int),
	}
	for _, stream := range streams {
		subtitles = append(subtitles, mapper.track(stream))
	}
	return subtitles
}

func mapSubtitleTracks(tracks []goffmpeg.SubtitleTrack) []utils.SubtitleTrack {
	streams := make([]SubtitleStream, 0, len(tracks))
	for _, t := range tracks {
		streams = append(streams, SubtitleStream{Index: t.Index, Codec: t.Codec, Language: t.Language, Title: t.Title})
	}
	return BuildSubtitleTracks(streams)
}

type subtitleTrackMapper struct {
	seenLang map[string]int
}

func (m *subtitleTrackMapper) track(stream SubtitleStream) utils.SubtitleTrack {
	index := stream.Index
	lang := strings.TrimSpace(stream.Language)
	return utils.SubtitleTrack{
		Index:    &index,
		Codec:    stream.Codec,
		Language: lang,
		Srclang:  m.srclang(lang, stream.Index),
		Title:    stream.Title,
		Embedded: true,
		Name:     "Track " + strconv.Itoa(stream.Index),
	}
}

// srclang must be unique per stream so Plyr/HTML can switch tracks with the same Language.
func (m *subtitleTrackMapper) srclang(language string, streamIndex int) string {
	lang := strings.TrimSpace(strings.ToLower(language))
	if lang != "" {
		count := m.seenLang[lang]
		m.seenLang[lang] = count + 1
		if count == 0 {
			return lang
		}
	}
	return fmt.Sprintf("x-track-%d", streamIndex)
}
