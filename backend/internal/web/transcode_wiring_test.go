package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestTranscodeManagerNilWhenDisabled(t *testing.T) {
	originalAvailable := settings.Env.FFmpegAvailable
	originalTranscode := settings.Config.Integrations.Media.Transcode
	originalManager := transcoding.Default()
	t.Cleanup(func() {
		settings.Env.FFmpegAvailable = originalAvailable
		settings.Config.Integrations.Media.Transcode = originalTranscode
		transcoding.SetDefault(originalManager)
	})

	settings.Env.FFmpegAvailable = false
	settings.Config.Integrations.Media.Transcode.Enabled = false
	transcoding.SetDefault(nil)

	setupTestEnv(t)

	if transcoding.Default() != nil {
		t.Fatal("transcode manager must be nil when transcoding is disabled")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(`{"source":"s","path":"/a.mp4"}`))
	rec := httptest.NewRecorder()
	status, err := transcodeStartHandler(rec, req, &Context{})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 while transcoding is disabled (err=%v)", status, err)
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

	prevConfig := settings.Config
	originalManager := transcoding.Default()
	t.Cleanup(func() {
		settings.SwapConfigForTest(prevConfig)
		transcoding.SetDefault(originalManager)
	})

	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "transcoding"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "transcoding"), 0o755); err == nil {
		t.Fatal("test setup: expected directory creation to fail on a file path")
	}
	cfg := settings.Config
	cfg.Server.CacheDir = cacheDir
	cfg.Integrations.Media.Transcode.Enabled = true
	settings.Env.FFmpegAvailable = true
	settings.SwapConfigForTest(cfg)

	if first := transcoding.Default(); first != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = first.Close(closeCtx)
	}

	if _, err := app.WireServicesWithContext(context.Background(), state.Default()); err != nil {
		t.Fatalf("wiring must tolerate an unusable transcode cache dir: %v", err)
	}
	mgr := transcoding.Default()
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mgr.Close(closeCtx)
	})
	if mgr == nil {
		t.Fatal("manager must be wired when transcoding is enabled")
	}
}
