package indexing

import (
	"sync"
	"testing"
	"time"
)

func TestLockDAVMutationsExcludesConcurrentMutation(t *testing.T) {
	idx := &Index{}
	release := LockDAVMutations(nil, idx, idx)
	acquired := make(chan struct{})
	go func() { unlock := LockDAVMutations(idx); close(acquired); unlock() }()
	select {
	case <-acquired:
		t.Fatal("mutation passed held gate")
	case <-time.After(30 * time.Millisecond):
	}
	release()
	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("gate was not released")
	}
}

func TestLockDAVMutationsCrossSourceOrder(t *testing.T) {
	a, b := &Index{}, &Index{}
	var wg sync.WaitGroup
	start, done := make(chan struct{}), make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(reverse bool) {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				var release func()
				if reverse {
					release = LockDAVMutations(b, a)
				} else {
					release = LockDAVMutations(a, b)
				}
				release()
			}
		}(i == 1)
	}
	go func() { wg.Wait(); close(done) }()
	close(start)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opposite source orders deadlocked")
	}
}
