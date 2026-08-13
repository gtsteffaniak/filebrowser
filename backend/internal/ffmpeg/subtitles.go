package ffmpeg

import (
	"context"
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
	naming := &subtitleNaming{
		seenCombo: make(map[string]int),
	}
	for _, stream := range tracks {
		index := stream.Index
		track := utils.SubtitleTrack{
			Index:    &index,
			Codec:    stream.Codec,
			Language: strings.TrimSpace(stream.Language),
			Title:    stream.Title,
			Embedded: true,
			Name:     naming.name(stream),
		}
		subtitles = append(subtitles, track)
	}
	return subtitles
}

// subtitleNaming builds display names for the captions menu. Language is shown via Plyr's badge;
// names are "Track" unless the same language+title pair appears more than once.
type subtitleNaming struct {
	seenCombo map[string]int
}

func (n *subtitleNaming) name(stream goffmpeg.SubtitleTrack) string {
	title := strings.TrimSpace(stream.Title)
	lang := strings.TrimSpace(stream.Language)
	key := lang + "\x00" + title

	count := n.seenCombo[key]
	n.seenCombo[key] = count + 1
	if count == 0 {
		return "Track"
	}
	return "Track " + strconv.Itoa(count)
}
