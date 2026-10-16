package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
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

// Wiring must not fail when the transcode cache directory cannot be created. The
// cache tree is created on demand, so a disabled (or currently unusable) cache
// path must not abort startup through logger.Fatalf.
func TestWireServicesToleratesUnavailableTranscodeCache(t *testing.T) {
	setupTestEnv(t)

	originalCacheDir := settings.Config.Server.CacheDir
	originalManager := transcoding.Default()
	t.Cleanup(func() {
		settings.Config.Server.CacheDir = originalCacheDir
		transcoding.SetDefault(originalManager)
	})

	// A file where the transcoding directory would go makes directory creation fail.
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "transcoding"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "transcoding"), 0o755); err == nil {
		t.Fatal("test setup: expected directory creation to fail on a file path")
	}
	settings.Config.Server.CacheDir = cacheDir

	if _, err := app.WireServicesWithContext(context.Background(), state.Default()); err != nil {
		t.Fatalf("wiring must tolerate an unusable transcode cache dir: %v", err)
	}
	if transcoding.Default() == nil {
		t.Fatal("manager must still be wired")
	}
}
