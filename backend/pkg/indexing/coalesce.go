package indexing

import "sync"

// pathCoalescer runs background work at most once per key at a time. A job submitted while
// one for the same key is running replaces any job already queued behind it; the latest
// queued job runs when the current one finishes. Concurrent requests for the same folder
// therefore hold at most one running and one queued goroutine instead of one per request,
// and the newest data is still persisted.
type pathCoalescer struct {
	mu      sync.Mutex
	pending map[string]*func()
}

// submit runs fn in the background, coalesced with other work for key.
func (c *pathCoalescer) submit(key string, fn func()) {
	c.mu.Lock()
	if c.pending == nil {
		c.pending = make(map[string]*func())
	}
	if slot, running := c.pending[key]; running {
		*slot = fn
		c.mu.Unlock()
		return
	}
	slot := new(func())
	c.pending[key] = slot
	c.mu.Unlock()

	go func() {
		for {
			fn()
			c.mu.Lock()
			next := *slot
			if next == nil {
				delete(c.pending, key)
				c.mu.Unlock()
				return
			}
			*slot = nil
			c.mu.Unlock()
			fn = next
		}
	}()
}
