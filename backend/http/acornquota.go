package http

import (
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/chainfs"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const quotaCacheTTL = 10 * time.Minute

type quotaCacheEntry struct {
	quotaBytes int64
	expiresAt  time.Time
}

var (
	quotaCacheMu sync.RWMutex
	quotaCache   = make(map[string]quotaCacheEntry)
)

// getUserQuotaBytes returns the effective quota for a user by querying acorn.tools.
// Results are cached per-user for 10 minutes. Falls back to the config default
// when in bypass mode, when the API is unavailable, or when the API returns 0.
func getUserQuotaBytes(username string) int64 {
	if settings.Env.ChainFsBypass || settings.Env.AcornToolsSecret == "" {
		return settings.Config.UserDefaults.DefaultQuotaBytes
	}

	quotaCacheMu.RLock()
	entry, ok := quotaCache[username]
	quotaCacheMu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.quotaBytes
	}

	access, err := chainfs.CheckAcornToolsAccess(settings.Env.AcornToolsURL, settings.Env.AcornToolsSecret, username)
	if err != nil {
		logger.Errorf("acornquota: failed to get quota for %s: %v", username, err)
		return settings.Config.UserDefaults.DefaultQuotaBytes
	}

	quota := access.QuotaBytes
	if quota <= 0 {
		quota = settings.Config.UserDefaults.DefaultQuotaBytes
	}

	quotaCacheMu.Lock()
	quotaCache[username] = quotaCacheEntry{quotaBytes: quota, expiresAt: time.Now().Add(quotaCacheTTL)}
	quotaCacheMu.Unlock()

	return quota
}
