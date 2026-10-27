package indexing

import (
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"testing"
)

func TestSkippable(t *testing.T) {
	if !skippable(fs.ErrPermission) || !skippable(fs.ErrNotExist) {
		t.Fatal("expected permission and not-exist to be skippable")
	}
	errIO := errors.New("input/output error")
	if skippable(errIO) {
		t.Fatal("I/O errors must fail the listing")
	}
}

func TestUnreadableWarnedBounded(t *testing.T) {
	unreadableMu.Lock()
	unreadableWarned = map[string]struct{}{}
	maxUnreadableWarned = 3
	unreadableMu.Unlock()
	t.Cleanup(func() {
		unreadableMu.Lock()
		unreadableWarned = map[string]struct{}{}
		maxUnreadableWarned = 10000
		unreadableMu.Unlock()
	})

	for i := 0; i < 5; i++ {
		firstUnreadableWarning(fmt.Sprintf("path-%d", i))
	}
	unreadableMu.Lock()
	n := len(unreadableWarned)
	unreadableMu.Unlock()
	if n > 3 {
		t.Fatalf("warn set grew to %d, want at most 3 after reset", n)
	}
}

func TestUnreadableWarnedConcurrent(t *testing.T) {
	unreadableMu.Lock()
	unreadableWarned = map[string]struct{}{}
	unreadableMu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			firstUnreadableWarning("shared-path")
		}()
	}
	wg.Wait()
	unreadableMu.Lock()
	n := len(unreadableWarned)
	unreadableMu.Unlock()
	if n != 1 {
		t.Fatalf("expected one warned path, got %d", n)
	}
}

func TestAllNamesSkippedAsNotExist(t *testing.T) {
	if !allNamesSkippedAsNotExist(3, 0, 3) {
		t.Fatal("expected all-not-exist skip to be detected")
	}
	if allNamesSkippedAsNotExist(3, 0, 2) {
		t.Fatal("mixed skip reasons must not trigger vanish guard")
	}
	if allNamesSkippedAsNotExist(0, 0, 0) {
		t.Fatal("empty directory must not trigger vanish guard")
	}
}
