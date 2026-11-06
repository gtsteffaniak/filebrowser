package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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
	settings.Config.Integrations.Media.Transcode = settings.FullAdminTranscode{
		UserTranscode: settings.UserTranscode{
			Enabled:              true,
			MaxConcurrentPerUser: 2,
		},
		MaxConcurrent: 3,
		Profiles:      settings.DefaultTranscodeProfiles(),
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

// keysOf returns the sorted keys of a JSON object.
func keysOf(t *testing.T, v any, path string) []string {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want object", path, v)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertKeys(t *testing.T, path string, got []string, want []string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("%s keys = %v, want %v", path, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s keys = %v, want %v", path, got, want)
		}
	}
}

// Non-admin users receive exactly the PUBLIC payload. Any new key fails this test,
// which forces a conscious decision about whether it belongs in UserMedia.
func TestSettingsMediaHandlerPlayerPayloadForNonAdmin(t *testing.T) {
	withMediaHandlerSettings(t)

	body := callMediaGet(t, false)
	assertKeys(t, "top-level", keysOf(t, body, "body"), []string{"transcode", "imagePreview"})
	assertKeys(t, "transcode", keysOf(t, body["transcode"], "transcode"),
		[]string{"enabled", "maxConcurrentPerUser", "enabledProfiles"})

	transcode := body["transcode"].(map[string]any)
	if enabled, ok := transcode["enabled"].(bool); !ok || !enabled {
		t.Fatalf("transcode.enabled = %v, want true", transcode["enabled"])
	}
	if limit, ok := transcode["maxConcurrentPerUser"].(float64); !ok || limit != 2 {
		t.Fatalf("transcode.maxConcurrentPerUser = %v, want 2", transcode["maxConcurrentPerUser"])
	}
	profiles, ok := transcode["enabledProfiles"].([]any)
	if !ok || len(profiles) != 3 {
		t.Fatalf("transcode.enabledProfiles = %v, want 3 enabled ids", transcode["enabledProfiles"])
	}
}

// Admins receive the full stored config. The keys shared with users must have the
// same names and values, so the admin payload is a strict superset of the user payload.
func TestSettingsMediaHandlerFullPayloadForAdmin(t *testing.T) {
	withMediaHandlerSettings(t)

	admin := callMediaGet(t, true)
	user := callMediaGet(t, false)

	// lockedFromConfigPaths is omitted when config pins nothing, so it is not required here.
	for _, key := range []string{"transcode", "imagePreview", "convert", "debug", "hardwareAcceleration", "accelDevice"} {
		if _, ok := admin[key]; !ok {
			t.Fatalf("admin payload missing %q", key)
		}
	}
	if _, ok := admin["enabled"]; ok {
		t.Fatal("admin payload must not repeat transcode.enabled at the top level")
	}

	adminTranscode := admin["transcode"].(map[string]any)
	for _, key := range []string{"maxConcurrent", "profiles", "cacheMaxSizeMB", "encoderBackend", "readRate"} {
		if _, ok := adminTranscode[key]; !ok {
			t.Fatalf("admin transcode missing %q", key)
		}
	}
	if _, ok := adminTranscode["profiles"].(map[string]any); !ok {
		t.Fatalf("admin transcode.profiles must be an object, got %T", adminTranscode["profiles"])
	}

	// Superset rule: every key a user receives must also exist for admins. Values may
	// differ by design: admins see stored config (so the UI can save it back unchanged),
	// users see effective values (for example enabled is false when ffmpeg is missing).
	userTranscode := user["transcode"].(map[string]any)
	for key := range userTranscode {
		if _, ok := adminTranscode[key]; !ok {
			t.Fatalf("admin transcode missing shared key %q", key)
		}
	}
	if admin["imagePreview"] == nil {
		t.Fatal("imagePreview must be present for admins")
	}
}

// Disabling transcoding at runtime must be visible to non-admin players.
func TestSettingsMediaHandlerReflectsDisabledTranscode(t *testing.T) {
	withMediaHandlerSettings(t)
	settings.Config.Integrations.Media.Transcode.Enabled = false

	body := callMediaGet(t, false)
	transcode := body["transcode"].(map[string]any)
	if enabled, ok := transcode["enabled"].(bool); !ok || enabled {
		t.Fatalf("transcode.enabled = %v, want false", transcode["enabled"])
	}
}
