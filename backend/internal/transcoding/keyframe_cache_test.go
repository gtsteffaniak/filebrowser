package transcoding

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestKeyframeCacheProbesOncePerFileVersion(t *testing.T) {
	c := &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}
	var calls atomic.Int32
	probe := func(context.Context) ([]float64, error) {
		calls.Add(1)
		return []float64{0, 4}, nil
	}
	key := keyframeKey{path: "/a.mkv", size: 10, modTime: 1}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := c.get(context.Background(), key, probe); err != nil || len(got) != 2 {
				t.Errorf("got %v, %v", got, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("probe calls = %d, want 1", calls.Load())
	}
	if _, err := c.get(context.Background(), keyframeKey{path: "/a.mkv", size: 10, modTime: 2}, probe); err != nil || calls.Load() != 2 {
		t.Fatalf("a modified file must be probed again (calls=%d, err=%v)", calls.Load(), err)
	}
}

func TestKeyframeCacheDoesNotCacheFailures(t *testing.T) {
	c := &keyframeCache{entries: map[keyframeKey]*keyframeEntry{}}
	key := keyframeKey{path: "/b.mkv"}
	if _, err := c.get(context.Background(), key, func(context.Context) ([]float64, error) { return nil, errors.New("boom") }); err == nil {
		t.Fatal("expected error")
	}
	got, err := c.get(context.Background(), key, func(context.Context) ([]float64, error) { return []float64{0}, nil })
	if err != nil || len(got) != 1 {
		t.Fatalf("retry after failure: %v, %v", got, err)
	}
}
