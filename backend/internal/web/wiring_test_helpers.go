package web

import (
	"context"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
)

// stopTranscodeManagerOnCleanup shuts down the default transcode manager when the
// test ends. Every WireServices call starts a janitor that reads media settings;
// without this, parallel tests race when another test restores Config.
func stopTranscodeManagerOnCleanup(t *testing.T) {
	if mgr := transcoding.Default(); mgr != nil {
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mgr.Close(closeCtx)
		})
	}
}
