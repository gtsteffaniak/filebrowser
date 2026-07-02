package settings

import (
	"path/filepath"
	"time"
)

func normalizeMediaCache() {
	if Config.Integrations.Media.CacheDurationMins < 0 {
		Config.Integrations.Media.CacheDurationMins = 0
	}
}

// MediaCacheDurationMins returns configured transcode disk cache TTL in minutes (0 = disabled).
// Config is retained for compatibility; transcode caching is not implemented.
func MediaCacheDurationMins() int {
	return Config.Integrations.Media.CacheDurationMins
}

// MediaDiskCacheEnabled reports whether transcode disk cache is configured.
// Config is retained for compatibility; transcode caching is not implemented.
func MediaDiskCacheEnabled() bool {
	return MediaCacheDurationMins() > 0
}

// MediaCacheDuration returns the configured TTL as a duration.
func MediaCacheDuration() time.Duration {
	mins := MediaCacheDurationMins()
	if mins <= 0 {
		return 0
	}
	return time.Duration(mins) * time.Minute
}

// TranscodeCacheDir returns the transcode disk cache root (unused; config compatibility only).
func TranscodeCacheDir() string {
	return filepath.Join(Config.Server.CacheDir, "transcode")
}

// PrepareTranscodeCacheDir is a no-op; transcode disk cache is not implemented.
func PrepareTranscodeCacheDir() error {
	return nil
}
