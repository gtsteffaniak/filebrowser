package activity

import (
	"sync"
	"testing"
	"time"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type mockActivityStore struct {
	mu      sync.Mutex
	inserts [][]activitydb.Entry
}

func (m *mockActivityStore) BulkInsertActivity(entries []activitydb.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := append([]activitydb.Entry(nil), entries...)
	m.inserts = append(m.inserts, copied)
	return nil
}

func (m *mockActivityStore) PurgeActivityBefore(cutoffUnix int64) (int64, error) {
	return 0, nil
}

func (m *mockActivityStore) batchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, b := range m.inserts {
		total += len(b)
	}
	return total
}

func TestRecorderFlushesOnMaxBuffer(t *testing.T) {
	store := &mockActivityStore{}
	globalMu.Lock()
	old := globalRecorder
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalRecorder = old
		globalMu.Unlock()
	}()

	r := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 4),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     3,
		flushInterval: time.Hour,
		enabled:       true,
	}
	globalMu.Lock()
	globalRecorder = r
	globalMu.Unlock()
	go r.loop()

	for i := 0; i < 3; i++ {
		Record(activitydb.Entry{
			UserID:    1,
			EventType: activitydb.EventDownload,
			CreatedAt: time.Now().Unix(),
		})
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.batchCount() >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Stop()

	if store.batchCount() < 3 {
		t.Fatalf("expected at least 3 flushed rows, got %d", store.batchCount())
	}
}

func TestRecorderTimerFlush(t *testing.T) {
	store := &mockActivityStore{}
	globalMu.Lock()
	old := globalRecorder
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalRecorder = old
		globalMu.Unlock()
	}()

	r := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 4),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     10000,
		flushInterval: 50 * time.Millisecond,
		enabled:       true,
	}
	globalMu.Lock()
	globalRecorder = r
	globalMu.Unlock()
	go r.loop()

	Record(activitydb.Entry{
		UserID:    2,
		EventType: activitydb.EventUpload,
		CreatedAt: time.Now().Unix(),
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.batchCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Stop()

	if store.batchCount() < 1 {
		t.Fatal("expected timer flush to persist entry")
	}
}

func TestRecorderAcceptsAnonymousUserID(t *testing.T) {
	store := &mockActivityStore{}
	globalMu.Lock()
	old := globalRecorder
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalRecorder = old
		globalMu.Unlock()
	}()

	r := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 4),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     10000,
		flushInterval: 50 * time.Millisecond,
		enabled:       true,
	}
	globalMu.Lock()
	globalRecorder = r
	globalMu.Unlock()
	go r.loop()

	Record(activitydb.Entry{
		UserID:    0,
		EventType: activitydb.EventDownload,
		CreatedAt: time.Now().Unix(),
		Details:   activitydb.Details{ShareHash: "legacy-test"},
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.batchCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Stop()

	if store.batchCount() < 1 {
		t.Fatal("expected anonymous (user_id 0) entry to be recorded")
	}
}

func TestInitializeDisabledNoOp(t *testing.T) {
	Initialize(nil, settings.Database{Activity: settings.ActivityConfig{Disabled: true}})
	if BufferLen() != 0 {
		t.Fatalf("expected disabled recorder buffer len 0")
	}
	Stop()
}

func TestRecorderRecordStopRace(t *testing.T) {
	store := &mockActivityStore{}
	globalMu.Lock()
	old := globalRecorder
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalRecorder = old
		globalMu.Unlock()
	}()

	r := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 512),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     10000,
		flushInterval: time.Hour,
		enabled:       true,
	}
	globalMu.Lock()
	globalRecorder = r
	globalMu.Unlock()
	go r.loop()

	// Seed one valid entry so the flush assertion is deterministic.
	Record(activitydb.Entry{
		UserID:    1,
		EventType: activitydb.EventDownload,
		CreatedAt: time.Now().Unix(),
	})

	stopDone := make(chan struct{})
	go func() {
		defer close(stopDone)
		Stop()
	}()

	const total = 200
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			Record(activitydb.Entry{
				UserID:    1,
				EventType: activitydb.EventDownload,
				CreatedAt: time.Now().Unix(),
			})
		}()
	}
	wg.Wait()
	<-stopDone

	if BufferLen() != 0 {
		t.Fatalf("expected empty buffer after shutdown, got %d stranded entries", BufferLen())
	}
	if got := store.batchCount(); got == 0 {
		t.Fatal("expected flushed rows after Record/Stop race, got none")
	}
}
