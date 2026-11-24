package testutil

import (
	"os"
	"os/exec"
	"testing"
)

// SkipIfNoFFmpeg skips the test when ffmpeg and ffprobe are not available on PATH,
// or when a custom ffmpeg path is set without ffprobe. Default `go test` must pass
// on machines without media binaries; CI installs ffmpeg for integration coverage.
func SkipIfNoFFmpeg(t *testing.T) {
	t.Helper()
	if os.Getenv("GOFFMPEG_FFMPEG_PATH") != "" {
		if os.Getenv("GOFFMPEG_FFPROBE_PATH") != "" {
			return
		}
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

// SkipIfNoTool skips when name is not on PATH.
func SkipIfNoTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available", name)
	}
}