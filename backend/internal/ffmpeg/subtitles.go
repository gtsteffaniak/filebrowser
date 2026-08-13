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
	seenLanguages := make(map[string]bool)
	for _, stream := range tracks {
		index := stream.Index
		track := utils.SubtitleTrack{
			Index:    &index,
			Codec:    stream.Codec,
			Language: uniqueSubtitleLanguage(stream.Language, stream.Index, seenLanguages),
			Title:    stream.Title,
			Embedded: true,
		}
		baseName := "Embedded Subtitle " + strconv.Itoa(stream.Index)
		if track.Title != "" {
			track.Name = baseName + " (" + track.Title + ")"
		} else if stream.Language != "" {
			track.Name = baseName + " (" + stream.Language + ")"
		} else {
			track.Name = baseName
		}
		subtitles = append(subtitles, track)
	}
	return subtitles
}

// uniqueSubtitleLanguage returns a BCP 47 language tag that is unique per stream.
// Plyr and HTML <track srclang> match tracks by language; duplicate or empty values
// prevent switching between multiple embedded tracks (e.g. two undefined-language subs).
func uniqueSubtitleLanguage(language string, streamIndex int, seen map[string]bool) string {
	lang := strings.TrimSpace(language)
	if lang == "" {
		lang = "und"
	}
	if seen[lang] {
		return fmt.Sprintf("%s-%d", lang, streamIndex)
	}
	seen[lang] = true
	return lang
}
