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

// objectField returns obj[key] as a JSON object, failing the test if it is missing or not an object.
func objectField(t *testing.T, obj map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := obj[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want object", key, obj[key])
	}
	return child
}

// keysOf returns the sorted keys of a JSON object.
func keysOf(obj map[string]any) []string {
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
	assertKeys(t, "top-level", keysOf(body), []string{"transcode", "imagePreview"})
	transcode := objectField(t, body, "transcode")
	assertKeys(t, "transcode", keysOf(transcode), []string{"enabled", "maxConcurrentPerUser", "enabledProfiles"})

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
// same names, so the admin payload is a strict superset of the user payload.
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

	adminTranscode := objectField(t, admin, "transcode")
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
	for key := range objectField(t, user, "transcode") {
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

	transcode := objectField(t, callMediaGet(t, false), "transcode")
	if enabled, ok := transcode["enabled"].(bool); !ok || enabled {
		t.Fatalf("transcode.enabled = %v, want false", transcode["enabled"])
	}
}
