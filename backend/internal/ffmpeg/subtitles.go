package ffmpeg

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

// DetectEmbeddedSubtitles detects embedded subtitle streams.
func DetectEmbeddedSubtitles(videoPath string, modtime time.Time) []utils.SubtitleTrack {
	key := "embedded_subtitles:" + videoPath + ":" + modtime.Format(time.RFC3339)
	if cached, ok := MediaCache.Get(key); ok {
		return cached
	}

	svc := Get()
	if svc == nil || svc.inner == nil {
		return nil
	}

	tracks, err := svc.inner.DetectSubtitles(context.Background(), videoPath)
	if err != nil {
		logger.Debug("ffprobe failed for file: " + videoPath + ", error: " + err.Error())
		return nil
	}

	subtitles := mapSubtitleTracks(tracks)
	MediaCache.Set(key, subtitles)
	return subtitles
}

func mapSubtitleTracks(tracks []goffmpeg.SubtitleTrack) []utils.SubtitleTrack {
	subtitles := make([]utils.SubtitleTrack, 0, len(tracks))
	mapper := &subtitleTrackMapper{
		seenLang: make(map[string]int),
	}
	for _, stream := range tracks {
		subtitles = append(subtitles, mapper.track(stream))
	}
	return subtitles
}

type subtitleTrackMapper struct {
	seenLang map[string]int
}

func (m *subtitleTrackMapper) track(stream goffmpeg.SubtitleTrack) utils.SubtitleTrack {
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
