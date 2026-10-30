package indexing

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPathCoalescerRunsLatestAndBoundsConcurrency(t *testing.T) {
	var c pathCoalescer
	release := make(chan struct{})
	started := make(chan struct{})
	var running, maxRunning, runs atomic.Int32
	var last atomic.Int32
	var wg sync.WaitGroup
	job := func(id int32) func() {
		return func() {
			n := running.Add(1)
			for {
				m := maxRunning.Load()
				if n <= m || maxRunning.CompareAndSwap(m, n) {
					break
				}
			}
			if id == 0 {
				close(started)
				<-release
			}
			last.Store(id)
			runs.Add(1)
			running.Add(-1)
			wg.Done()
		}
	}
	wg.Add(1)
	c.submit("/a/", job(0))
	<-started
	for i := int32(1); i <= 5; i++ {
		c.submit("/a/", job(i))
	}
	wg.Add(1) // only the latest queued job (5) runs after the first
	close(release)
	wg.Wait()
	if got := runs.Load(); got != 2 {
		t.Fatalf("runs = %d, want 2 (running job + latest queued)", got)
	}
	if got := last.Load(); got != 5 {
		t.Fatalf("last job = %d, want 5", got)
	}
	if got := maxRunning.Load(); got != 1 {
		t.Fatalf("max concurrent = %d, want 1", got)
	}
	// The final job's wg.Done runs before the coalescer drops the key, so poll briefly.
	var left int
	for deadline := time.Now().Add(2 * time.Second); ; {
		c.mu.Lock()
		left = len(c.pending)
		c.mu.Unlock()
		if left == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if left != 0 {
		t.Fatalf("pending keys left = %d, want 0", left)
	}
}
