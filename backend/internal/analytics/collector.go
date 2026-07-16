package analytics

import (
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	cacheMu        sync.RWMutex
	cachedEnvelope []byte
)

func init() {
	indexing.OnSourceRootFullScanComplete = NotifySourceFullScanComplete
}

// NotifySourceFullScanComplete rebuilds the in-memory snapshot after a source root full scan.
func NotifySourceFullScanComplete(sourceName string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if rebuildSnapshotLocked(false) {
		logger.Debugf("analytics deployment snapshot updated after full scan of source %q", sourceName)
	}
}

// Ready reports whether a snapshot is cached and available to preview or publish.
func Ready() bool {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return len(cachedEnvelope) > 0
}

// RefreshSnapshot rebuilds the cached snapshot from current server state.
func RefreshSnapshot() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	rebuildSnapshotLocked(false)
}

func rebuildSnapshotLocked(publishable bool) bool {
	envelope, err := buildSnapshot(publishable)
	if err != nil {
		logger.Debugf("analytics snapshot build skipped: %v", err)
		return false
	}

	cachedEnvelope = envelope
	return true
}

func cachedSnapshotCopy() ([]byte, error) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if len(cachedEnvelope) == 0 {
		return nil, fmt.Errorf("analytics snapshot not ready yet; waiting for a source full scan or use diagnostic preview")
	}
	out := make([]byte, len(cachedEnvelope))
	copy(out, cachedEnvelope)
	return out, nil
}

func invalidateSnapshotCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedEnvelope = nil
}

// InvalidateCache clears the in-memory snapshot (e.g. when analytics is disabled).
func InvalidateCache() {
	invalidateSnapshotCache()
}

// PreviewSnapshot returns the cached JSON envelope, building it on demand when needed.
func PreviewSnapshot() ([]byte, error) {
	if body, err := cachedSnapshotCopy(); err == nil {
		return body, nil
	}
	RefreshSnapshot()
	return cachedSnapshotCopy()
}

// PublishSnapshot returns a snapshot suitable for publishing to the analytics API.
func PublishSnapshot() ([]byte, error) {
	if _, err := snapshotVersion(true); err == nil {
		if body, err := cachedSnapshotCopy(); err == nil {
			return body, nil
		}
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	envelope, err := buildSnapshot(true)
	if err != nil {
		return nil, err
	}
	cachedEnvelope = envelope
	out := make([]byte, len(envelope))
	copy(out, envelope)
	return out, nil
}
