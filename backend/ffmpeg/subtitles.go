package ffmpeg

import (
	"context"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
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
	for _, stream := range tracks {
		index := stream.Index
		track := utils.SubtitleTrack{
			Index:    &index,
			Codec:    stream.Codec,
			Language: stream.Language,
			Title:    stream.Title,
			Embedded: true,
		}
		if track.Title != "" {
			track.Name = track.Title
		} else if track.Language != "" {
			track.Name = "Embedded (" + track.Language + ")"
		} else {
			track.Name = "Embedded Subtitle " + strconv.Itoa(stream.Index)
		}
		subtitles = append(subtitles, track)
	}
	return subtitles
}
