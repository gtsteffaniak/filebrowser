package settings

// Helpers in this file are compiled into the binary but are only ever called
// from tests, mirroring pkg/indexing/test_helpers.go.

// SwapConfigForTest replaces the global Config and returns the previous value.
//
// The swap happens under the media lock because background readers — the
// transcode manager's janitor reads the cache size and retention limits on every
// eviction — would otherwise observe a torn Config while a test restores it.
func SwapConfigForTest(next Settings) Settings {
	mediaMu.Lock()
	defer mediaMu.Unlock()
	prev := Config
	Config = next
	return prev
}
