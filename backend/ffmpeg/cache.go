package ffmpeg

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	MediaCache           = cache.NewCache[[]utils.SubtitleTrack](24 * time.Hour) // subtitles get cached for 24 hours
	SubtitleContentCache = cache.NewCache[string](24 * time.Hour)                // subtitle content gets cached for 24 hours
	MetadataCache        = cache.NewCache[float64](24 * time.Hour)               // media duration gets cached for 24 hours
)
