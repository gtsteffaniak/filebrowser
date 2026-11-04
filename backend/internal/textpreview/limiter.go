package textpreview

import (
	"context"
	"sync"
)

// defaultConcurrency is used when SetConcurrency has not run yet (tests, or an
// unset server.numImageProcessors).
const defaultConcurrency = 4

// Limiter bounds how many text previews are read and converted at once.
//
// It is deliberately separate from the image pipeline's semaphore in
// internal/preview: a text preview is a 16KB read plus an optional RTF
// conversion, so it should not compete with image decode/resize work and it
// should not be limited to the image processor count.
type Limiter struct {
	ch chan struct{}
}

// NewLimiter returns a limiter that admits up to n concurrent callers.
// n below 1 falls back to defaultConcurrency.
func NewLimiter(n int) *Limiter {
	if n < 1 {
		n = defaultConcurrency
	}
	return &Limiter{ch: make(chan struct{}, n)}
}

// Acquire takes a slot, waiting until one is free or ctx is done.
func (l *Limiter) Acquire(ctx context.Context) error {
	select {
	case l.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns a slot taken by Acquire.
func (l *Limiter) Release() {
	<-l.ch
}

var (
	defaultMu      sync.RWMutex
	defaultLimiter = NewLimiter(defaultConcurrency)
)

// SetConcurrency replaces the package-level limiter with one of size n.
// It is called once at startup, next to the preview generator.
func SetConcurrency(n int) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLimiter = NewLimiter(n)
}

// Acquire takes a slot on the package-level limiter.
func Acquire(ctx context.Context) error {
	defaultMu.RLock()
	l := defaultLimiter
	defaultMu.RUnlock()
	return l.Acquire(ctx)
}

// Release returns a slot taken by Acquire.
func Release() {
	defaultMu.RLock()
	l := defaultLimiter
	defaultMu.RUnlock()
	l.Release()
}
