package ffmpeg

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestGetMediaDurationConcurrentNoDeadlock ensures probe-tier concurrency does not deadlock.
// With MaxProbe=2 (see initIntegrationService), four concurrent GetMediaDuration calls
// queue and complete. Probe concurrency is what we test here—not hardware encoder smoke
// probes (those belong in go-ffmpeg).
func TestGetMediaDurationConcurrentNoDeadlock(t *testing.T) {
	svc := initIntegrationService(t)
	path, _ := copyTestFixtureMP4(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const workers = 4
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GetMediaDuration(ctx, path); err != nil {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(25 * time.Second):
		t.Fatal("concurrent GetMediaDuration calls deadlocked with MaxProbe=2")
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("GetMediaDuration() error = %v", err)
		}
	}
}