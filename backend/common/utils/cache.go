package utils

import (
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	DiskUsageCache     = cache.NewCache(30*time.Second, 24*time.Hour)
	RealPathCache      = cache.NewCache(48*time.Hour, 72*time.Hour)
	SearchResultsCache = cache.NewCache(15*time.Second, 1*time.Hour)
	OnlyOfficeCache    = cache.NewCache(48*time.Hour, 1*time.Hour)
	JwtCache           = cache.NewCache(1*time.Hour, 72*time.Hour)
	MediaCache         = cache.NewCache(24 * time.Hour) // subtitles get cached for 24 hours
)
