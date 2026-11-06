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

// A create request must not mutate the shared in-memory userDefaults (json.Unmarshal
// reuses slice backing arrays).
func TestUsersPostDoesNotMutateUserDefaults(t *testing.T) {
	setupTestEnv(t)

	defaults := settings.Config.UserDefaults
	defaults.Listing.NewFileTemplate = []string{"default.txt"}
	state.SetEffectiveUserDefaultsForTest(t, defaults)

	admin := createPostTestAdmin(t)

	body := []byte(`{"which":[],"data":{"username":"nomutate","password":"secret","loginMethod":"password",
		"newFileTemplate":["custom.txt"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	status, err := usersPostHandler(httptest.NewRecorder(), req, &Context{User: admin})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("usersPostHandler() status=%d err=%v", status, err)
	}
	if got := state.EffectiveUserDefaults().Listing.NewFileTemplate; len(got) != 1 || got[0] != "default.txt" {
		t.Errorf("userDefaults newFileTemplate = %v, want [default.txt]", got)
	}
}
