package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// The transcode manager must be wired even when ffmpeg is unavailable at startup.
// It is registered nil in that case today, and a later runtime enable (an admin
// PATCH of the media settings) never creates one, so every transcode endpoint
// would keep returning 503 until the next restart.
func TestTranscodeManagerIsWiredWithoutFFmpeg(t *testing.T) {
	originalAvailable := settings.Env.FFmpegAvailable
	originalTranscode := settings.Config.Integrations.Media.Transcode
	originalManager := transcoding.Default()
	t.Cleanup(func() {
		settings.Env.FFmpegAvailable = originalAvailable
		settings.Config.Integrations.Media.Transcode = originalTranscode
		transcoding.SetDefault(originalManager)
	})

	// Startup with ffmpeg missing and transcoding disabled.
	settings.Env.FFmpegAvailable = false
	settings.Config.Integrations.Media.Transcode = settings.MediaTranscode{Enabled: false}

	setupTestEnv(t)

	mgr := transcoding.Default()
	if mgr == nil {
		t.Fatal("transcode manager must be wired even when ffmpeg is unavailable")
	}
	if mgr.Enabled() {
		t.Fatal("manager must report disabled while ffmpeg is unavailable")
	}

	// Requests are gated on availability, not on the manager being nil.
	req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(`{"source":"s","path":"/a.mp4"}`))
	rec := httptest.NewRecorder()
	status, err := transcodeStartHandler(rec, req, &Context{})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 while ffmpeg is unavailable (err=%v)", status, err)
	}
	if err != transcoding.ErrDisabled {
		t.Fatalf("err = %v, want ErrDisabled", err)
	}
}
