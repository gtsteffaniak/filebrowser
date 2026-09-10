package state

import (
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestQuotaFlusherConcurrentFlushAndStop(t *testing.T) {
	quotaFlusherMu.Lock()
	oldFlusher := quotaFlusher
	quotaFlusher = nil
	quotaFlusherMu.Unlock()

	oldCounters := quotaCounters
	initQuotaMaps()
	defer func() {
		StopQuotaFlusher()
		quotasMux.Lock()
		quotaCounters = oldCounters
		quotasMux.Unlock()
		quotaFlusherMu.Lock()
		quotaFlusher = oldFlusher
		quotaFlusherMu.Unlock()
	}()

	quotaCounters["test-quota"] = &quotaCounterMem{UsedBytes: 100, Dirty: true}

	flusher := &quotaCounterFlusher{
		dirtyIDs:      map[string]struct{}{"test-quota": {}},
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		flushInterval: time.Hour,
		maxBuffers:    1,
	}
	quotaFlusherMu.Lock()
	quotaFlusher = flusher
	quotaFlusherMu.Unlock()
	go flusher.loop()

	holdQuotas := make(chan struct{})
	releaseQuotas := make(chan struct{})
	go func() {
		quotasMux.Lock()
		close(holdQuotas)
		<-releaseQuotas
		quotasMux.Unlock()
	}()
	<-holdQuotas

	stopDone := make(chan struct{})
	go func() {
		defer close(stopDone)
		StopQuotaFlusher()
	}()

	const total = 200
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			markQuotaCounterDirty("test-quota")
			signalQuotaFlush()
		}()
	}
	wg.Wait()
	close(releaseQuotas)

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("StopQuotaFlusher deadlocked with concurrent flush/mutation while quotasMux was held")
	}
}

func TestQuotaFlusherReplacementWhileQuotasLocked(t *testing.T) {
	quotaFlusherMu.Lock()
	oldFlusher := quotaFlusher
	quotaFlusher = nil
	quotaFlusherMu.Unlock()

	oldCounters := quotaCounters
	initQuotaMaps()
	defer func() {
		StopQuotaFlusher()
		quotasMux.Lock()
		quotaCounters = oldCounters
		quotasMux.Unlock()
		quotaFlusherMu.Lock()
		quotaFlusher = oldFlusher
		quotaFlusherMu.Unlock()
	}()

	quotaCounters["test-quota"] = &quotaCounterMem{UsedBytes: 50, Dirty: true}

	startQuotaFlusher(settings.QuotasConfig{
		FlushIntervalSeconds: 3600,
		FlushMaxBuffers:      1,
	})

	holdQuotas := make(chan struct{})
	releaseQuotas := make(chan struct{})
	go func() {
		quotasMux.Lock()
		close(holdQuotas)
		<-releaseQuotas
		quotasMux.Unlock()
	}()
	<-holdQuotas

	replaceDone := make(chan struct{})
	go func() {
		defer close(replaceDone)
		startQuotaFlusher(settings.QuotasConfig{
			FlushIntervalSeconds: 3600,
			FlushMaxBuffers:      1,
		})
	}()

	const total = 100
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			markQuotaCounterDirty("test-quota")
		}()
	}
	wg.Wait()
	close(releaseQuotas)

	select {
	case <-replaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("startQuotaFlusher deadlocked while replacing flusher with quotasMux held")
	}
}
