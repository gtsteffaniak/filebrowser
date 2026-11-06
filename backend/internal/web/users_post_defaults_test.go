package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Omitted permissions and omitted profile keys come from userDefaults; a partially
// specified fileLoading object only overrides the keys it contains.
func TestUsersPostOmittedFieldsFallBackToDefaults(t *testing.T) {
	setupTestEnv(t)

	defaults := settings.Config.UserDefaults
	defaults.UI.Locale = "de"
	defaults.Account.Permissions.Share = true
	defaults.FileLoading.MaxConcurrent = 7
	defaults.FileLoading.UploadChunkSize = 9
	state.SetEffectiveUserDefaultsForTest(t, defaults)

	admin := createPostTestAdmin(t)

	body := []byte(`{"which":[],"data":{"username":"partial","password":"secret","loginMethod":"password",
		"fileLoading":{"maxConcurrentUpload":3}}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	status, err := usersPostHandler(httptest.NewRecorder(), req, &Context{User: admin})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("usersPostHandler() status=%d err=%v", status, err)
	}
	got, err := state.GetUserByUsername("partial")
	if err != nil {
		t.Fatal(err)
	}
	if got.Locale != "de" {
		t.Errorf("locale = %q, want default de", got.Locale)
	}
	if !got.Permissions.Share {
		t.Errorf("share = false, want true from default permissions")
	}
	if got.FileLoading.MaxConcurrent != 3 {
		t.Errorf("maxConcurrentUpload = %d, want requested 3", got.FileLoading.MaxConcurrent)
	}
	if got.FileLoading.UploadChunkSize != 9 {
		t.Errorf("uploadChunkSizeMb = %d, want default 9", got.FileLoading.UploadChunkSize)
	}
}
