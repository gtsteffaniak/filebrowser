package ffmpeg

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	MediaCache           = cache.NewCache[[]utils.SubtitleTrack](24 * time.Hour) // subtitle track lists from ffprobe
	SubtitleContentCache = cache.NewCache[string](24 * time.Hour)                // extracted embedded subtitle content
	MetadataCache        = cache.NewCache[float64](24 * time.Hour)               // media duration from ffprobe
	ProbeCache           = cache.NewCache[StreamInfo](24 * time.Hour)            // ffprobe stream info
)
