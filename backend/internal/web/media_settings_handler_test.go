package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func withMediaHandlerSettings(t *testing.T) {
	t.Helper()
	originalTranscode := settings.Config.Integrations.Media.Transcode
	originalFFmpeg := settings.Env.FFmpegAvailable
	t.Cleanup(func() {
		settings.Config.Integrations.Media.Transcode = originalTranscode
		settings.Env.FFmpegAvailable = originalFFmpeg
	})
	settings.Env.FFmpegAvailable = true
	settings.Config.Integrations.Media.Transcode = settings.MediaTranscode{
		Enabled:              true,
		MaxConcurrent:        3,
		MaxConcurrentPerUser: 2,
		Profiles:             settings.DefaultTranscodeProfiles(),
	}
}

func mediaHandlerContext(admin bool) *Context {
	return &Context{User: &users.User{FrontendUser: users.FrontendUser{
		Username:    "tester",
		Permissions: users.Permissions{Admin: admin},
	}}}
}

func callMediaGet(t *testing.T, admin bool) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/media", nil)
	status, err := settingsMediaGetHandler(rec, req, mediaHandlerContext(admin))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

// Non-admin users must be able to read the effective media values for the
// player, but only the player-relevant subset.
func TestSettingsMediaHandlerPlayerPayloadForNonAdmin(t *testing.T) {
	withMediaHandlerSettings(t)

	body := callMediaGet(t, false)
	if _, ok := body["transcode"]; ok {
		t.Fatal("non-admin must not receive the full transcode settings")
	}
	if _, ok := body["lockedFromConfigPaths"]; ok {
		t.Fatal("non-admin must not receive config lock metadata")
	}
	if _, ok := body["lockMessage"]; ok {
		t.Fatal("non-admin must not receive the config lock message")
	}
	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatalf("enabled = %v, want true", body["enabled"])
	}
	if limit, ok := body["maxConcurrentPerUser"].(float64); !ok || limit != 2 {
		t.Fatalf("maxConcurrentPerUser = %v, want 2", body["maxConcurrentPerUser"])
	}
	profiles, ok := body["profiles"].([]any)
	if !ok || len(profiles) != 3 {
		t.Fatalf("profiles = %v, want 3 enabled ids", body["profiles"])
	}
}

func TestSettingsMediaHandlerFullPayloadForAdmin(t *testing.T) {
	withMediaHandlerSettings(t)

	body := callMediaGet(t, true)
	transcode, ok := body["transcode"].(map[string]any)
	if !ok {
		t.Fatalf("admin must receive the full transcode settings, got %T", body["transcode"])
	}
	if _, ok := transcode["profiles"].(map[string]any); !ok {
		t.Fatalf("admin transcode payload must include profiles, got %v", transcode["profiles"])
	}
	// Admin payload exposes the full settings surface, not the player subset.
	for _, key := range []string{"convert", "debug", "extractEmbeddedSubtitles", "hardwareAcceleration"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("admin payload missing %q", key)
		}
	}
	if _, ok := body["enabled"]; ok {
		t.Fatal("the admin payload uses the full settings shape, not the player subset")
	}
}

// Disabling transcoding at runtime must be visible to non-admin players.
func TestSettingsMediaHandlerReflectsDisabledTranscode(t *testing.T) {
	withMediaHandlerSettings(t)
	settings.Config.Integrations.Media.Transcode.Enabled = false

	body := callMediaGet(t, false)
	if enabled, ok := body["enabled"].(bool); !ok || enabled {
		t.Fatalf("enabled = %v, want false", body["enabled"])
	}
}
