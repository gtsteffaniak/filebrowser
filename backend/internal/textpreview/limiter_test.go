package textpreview

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLimiterFallsBackToDefault(t *testing.T) {
	for _, n := range []int{0, -3} {
		l := NewLimiter(n)
		if got := cap(l.ch); got != defaultConcurrency {
			t.Fatalf("NewLimiter(%d) capacity = %d, want %d", n, got, defaultConcurrency)
		}
	}
}

func TestLimiterBoundsConcurrency(t *testing.T) {
	const limit = 2
	l := NewLimiter(limit)

	var running, maxRunning atomic.Int32
	var wg sync.WaitGroup
	release := make(chan struct{})

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(context.Background()); err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			defer l.Release()
			cur := running.Add(1)
			for {
				prev := maxRunning.Load()
				if cur <= prev || maxRunning.CompareAndSwap(prev, cur) {
					break
				}
			}
			<-release
			running.Add(-1)
		}()
	}

	// Let the admitted goroutines block on release; the rest must be waiting.
	time.Sleep(50 * time.Millisecond)
	if got := running.Load(); got != limit {
		t.Fatalf("running = %d, want %d", got, limit)
	}
	close(release)
	wg.Wait()

	if got := maxRunning.Load(); got != limit {
		t.Fatalf("max concurrent = %d, want %d", got, limit)
	}
	if got := len(l.ch); got != 0 {
		t.Fatalf("slots still held after release: %d", got)
	}
}

func TestLimiterAcquireHonoursCancellation(t *testing.T) {
	l := NewLimiter(1)
	if err := l.Acquire(context.Background()); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer l.Release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Acquire(ctx); err == nil {
		t.Fatal("expected an error when the context is already cancelled")
	}
}

// A caller that never releases must not wedge later callers forever; the
// package-level limiter also has to survive SetConcurrency swapping it.
func TestSetConcurrencySwapsLimiter(t *testing.T) {
	original := defaultLimiter
	t.Cleanup(func() {
		defaultMu.Lock()
		defaultLimiter = original
		defaultMu.Unlock()
	})

	SetConcurrency(3)
	if err := Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire after SetConcurrency: %v", err)
	}
	Release()

	defaultMu.RLock()
	got := cap(defaultLimiter.ch)
	defaultMu.RUnlock()
	if got != 3 {
		t.Fatalf("capacity = %d, want 3", got)
	}
}
