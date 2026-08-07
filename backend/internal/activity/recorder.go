package activity

import (
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Store persists buffered activity entries.
type Store interface {
	BulkInsertActivity(entries []activitydb.Entry) error
	PurgeActivityBefore(cutoffUnix int64) (int64, error)
}

// Recorder buffers activity entries and flushes them to SQLite in batches.
type Recorder struct {
	store Store

	mu     sync.Mutex
	buffer []activitydb.Entry

	flushCh chan struct{}
	stopCh  chan struct{}
	doneCh  chan struct{}

	maxBuffer     int
	flushInterval time.Duration
	retentionDays int
	enabled       bool
	stopped       bool
}

var (
	globalRecorder *Recorder
	globalMu       sync.RWMutex
)

// Initialize starts the global activity recorder.
func Initialize(store *sqldb.SQLStore, cfg settings.Database) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalRecorder != nil {
		globalRecorder.Stop()
	}

	act := cfg.Activity
	maxBuffer := act.MaxBufferSize
	if maxBuffer <= 0 {
		maxBuffer = 10000
	}
	flushSeconds := act.FlushIntervalSeconds
	if flushSeconds <= 0 {
		flushSeconds = 10
	}
	retentionDays := act.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	r := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 256),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     maxBuffer,
		flushInterval: time.Duration(flushSeconds) * time.Second,
		retentionDays: retentionDays,
		enabled:       !act.Disabled,
	}
	globalRecorder = r
	go r.loop()

	if r.enabled {
		if n, err := r.purgeExpired(); err != nil {
			logger.Warningf("activity retention purge on startup failed: %v", err)
		} else if n > 0 {
			logger.Infof("activity retention purge removed %d rows on startup", n)
		}
	}
}

// Record appends an activity entry to the buffer (non-blocking).
func Record(entry activitydb.Entry) {
	globalMu.RLock()
	r := globalRecorder
	globalMu.RUnlock()
	if r == nil || !r.enabled {
		return
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if !entry.EventType.Valid() {
		return
	}

	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.buffer = append(r.buffer, entry)
	shouldFlush := len(r.buffer) >= r.maxBuffer
	r.mu.Unlock()

	if shouldFlush {
		r.signalFlush()
	}
}

// Stop flushes pending entries and stops the background loop.
func Stop() {
	globalMu.Lock()
	r := globalRecorder
	globalRecorder = nil
	globalMu.Unlock()
	if r != nil {
		r.Stop()
	}
}

// Stop flushes and shuts down the recorder instance.
func (r *Recorder) Stop() {
	close(r.stopCh)
	<-r.doneCh
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	r.flush()
}

func (r *Recorder) loop() {
	defer close(r.doneCh)
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()

	purgeTicker := time.NewTicker(24 * time.Hour)
	defer purgeTicker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.flush()
		case <-r.flushCh:
			r.flush()
		case <-purgeTicker.C:
			if r.enabled {
				if n, err := r.purgeExpired(); err != nil {
					logger.Warningf("activity retention purge failed: %v", err)
				} else if n > 0 {
					logger.Debugf("activity retention purge removed %d rows", n)
				}
			}
		}
	}
}

func (r *Recorder) signalFlush() {
	select {
	case r.flushCh <- struct{}{}:
	default:
	}
}

func (r *Recorder) flush() {
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.mu.Unlock()
		return
	}
	batch := r.buffer
	r.buffer = make([]activitydb.Entry, 0, 256)
	r.mu.Unlock()

	if err := r.store.BulkInsertActivity(batch); err != nil {
		logger.Errorf("activity flush failed (%d entries): %v", len(batch), err)
		time.Sleep(100 * time.Millisecond)
		if retryErr := r.store.BulkInsertActivity(batch); retryErr == nil {
			return
		}
		r.mu.Lock()
		// Re-queue at front, cap to avoid unbounded growth on persistent failure.
		combined := append(batch, r.buffer...)
		if len(combined) > r.maxBuffer {
			combined = combined[len(combined)-r.maxBuffer:]
		}
		r.buffer = combined
		r.mu.Unlock()
	}
}

func (r *Recorder) purgeExpired() (int64, error) {
	cutoff := time.Now().Add(-time.Duration(r.retentionDays) * 24 * time.Hour).Unix()
	return r.store.PurgeActivityBefore(cutoff)
}

// FlushNow forces a flush (for tests).
func FlushNow() {
	globalMu.RLock()
	r := globalRecorder
	globalMu.RUnlock()
	if r != nil {
		r.flush()
	}
}

// BufferLen returns current buffer size (for tests).
func BufferLen() int {
	globalMu.RLock()
	r := globalRecorder
	globalMu.RUnlock()
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buffer)
}
