package settings

import "sync"

// Helpers in this file are compiled into the binary but are only ever called
// from tests, mirroring pkg/indexing/test_helpers.go.

var configTestMu sync.Mutex

// SwapConfigForTest replaces the global Config and returns the previous value.
func SwapConfigForTest(next Settings) Settings {
	configTestMu.Lock()
	defer configTestMu.Unlock()
	prev := Config
	Config = next
	return prev
}
