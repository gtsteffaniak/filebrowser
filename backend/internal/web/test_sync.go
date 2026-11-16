package web

import (
	"sync"
	"testing"
)

// webGlobalTestMu serializes tests that mutate process-wide settings.Config or state.
var webGlobalTestMu sync.Mutex

func lockWebGlobals(t testing.TB) {
	t.Helper()
	webGlobalTestMu.Lock()
	t.Cleanup(webGlobalTestMu.Unlock)
}
