package indexing

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func TestPrepareScanStateInitializesMaps(t *testing.T) {
	idx := &Index{
		Source: settings.Source{Name: "test", Path: t.TempDir()},
	}
	s := &Scanner{scanPath: "/music/", idx: idx}

	s.prepareScanState()

	idx.mu.Lock()
	idx.scanUpdatedPaths["/music/"] = true
	idx.mu.Unlock()

	if s.processedInodes == nil {
		t.Fatal("processedInodes should be initialized")
	}
	s.processedInodes[1] = struct{}{}
}

func TestIsPathGone(t *testing.T) {
	if isPathGone(nil) {
		t.Fatal("nil error should not be path gone")
	}
	if !isPathGone(os.ErrNotExist) {
		t.Fatal("os.ErrNotExist should be path gone")
	}
}
