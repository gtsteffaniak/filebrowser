package utils

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	DiskUsageCache       = cache.NewCache[string](30*time.Second, 24*time.Hour)
	RealPathCache        = cache.NewCache[string](48*time.Hour, 72*time.Hour)
	SearchResultsCache   = cache.NewCache[string](15*time.Second, 1*time.Hour)
	OnlyOfficeCache      = cache.NewCache[string](48*time.Hour, 1*time.Hour)
	JwtCache             = cache.NewCache[string](1*time.Hour, 72*time.Hour)
	MediaCache           = cache.NewCache[[]ffmpeg.SubtitleTrack](24 * time.Hour) // subtitles get cached for 24 hours
	SubtitleContentCache = cache.NewCache[string](24 * time.Hour)                 // subtitle content gets cached for 24 hours
)
