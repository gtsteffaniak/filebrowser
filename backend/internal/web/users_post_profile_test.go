package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func createPostTestAdmin(t *testing.T) *users.User {
	t.Helper()
	admin := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			LoginMethod: users.LoginMethodOidc,
			Permissions: users.Permissions{Admin: true},
		},
	}
	if err := state.CreateUser(admin, ""); err != nil {
		t.Fatal(err)
	}
	return admin
}

// Issue #3322: profile/account values in POST /api/users must not be replaced by userDefaults.
func TestUsersPostKeepsRequestedProfileValues(t *testing.T) {
	setupTestEnv(t)

	defaults := settings.Config.UserDefaults
	defaults.UI.Locale = "en"
	defaults.UI.ThemeColor = "var(--blue)"
	darkTrue := true
	defaults.UI.DarkMode = &darkTrue
	defaults.Account.LockPassword = false
	defaults.Account.DisableSettings = false
	defaults.Account.ShowAdvancedProfile = false
	defaults.Listing.PromptRightCloseButton = false
	defaults.Listing.NewFileTemplate = []string{""}
	defaults.Listing.ShowHidden = true
	state.SetEffectiveUserDefaultsForTest(t, defaults)

	admin := createPostTestAdmin(t)

	body := []byte(`{"which":[],"data":{
		"username":"profiletest","password":"secret","loginMethod":"password",
		"locale":"zhCN","themeColor":"var(--green)","darkMode":false,
		"lockPassword":true,"disableSettings":true,"showAdvancedProfile":true,
		"promptRightCloseButton":true,"newFileTemplate":["custom.txt"],
		"preview":{"image":false},
		"permissions":{"admin":false,"api":true,"share":true,"realtime":false},
		"scopes":[{"name":"/","scope":"/","permissions":{"view":true,"download":true,"modify":true,"create":true,"delete":true}}]
	}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	status, err := usersPostHandler(rec, req, &Context{User: admin})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("usersPostHandler() status=%d err=%v", status, err)
	}

	got, err := state.GetUserByUsername("profiletest")
	if err != nil {
		t.Fatal(err)
	}
	if got.Locale != "zhCN" {
		t.Errorf("locale = %q, want zhCN", got.Locale)
	}
	if got.ThemeColor != "var(--green)" {
		t.Errorf("themeColor = %q, want var(--green)", got.ThemeColor)
	}
	if got.DarkMode {
		t.Errorf("darkMode = true, want false")
	}
	if !got.LockPassword {
		t.Errorf("lockPassword = false, want true")
	}
	if !got.DisableSettings {
		t.Errorf("disableSettings = false, want true")
	}
	if !got.ShowAdvancedProfile {
		t.Errorf("showAdvancedProfile = false, want true")
	}
	if !got.PromptRightCloseButton {
		t.Errorf("promptRightCloseButton = false, want true")
	}
	if len(got.NewFileTemplate) != 1 || got.NewFileTemplate[0] != "custom.txt" {
		t.Errorf("newFileTemplate = %v, want [custom.txt]", got.NewFileTemplate)
	}
	if got.Preview.Image {
		t.Errorf("preview.image = true, want false")
	}
	// Omitted fields still come from userDefaults.
	if !got.ShowHidden {
		t.Errorf("showHidden = false, want true from userDefaults")
	}
	if !got.Preview.Video {
		t.Errorf("preview.video = false, want true from userDefaults")
	}
	wantPerms := users.Permissions{Api: true, Share: true}
	if got.Permissions.Admin != wantPerms.Admin || got.Permissions.Api != wantPerms.Api ||
		got.Permissions.Share != wantPerms.Share || got.Permissions.Realtime != wantPerms.Realtime {
		t.Errorf("permissions = %+v, want %+v", got.Permissions, wantPerms)
	}
}

// Enforced userDefaults still win over request values on create.
func TestUsersPostEnforcedDefaultsWinOverRequest(t *testing.T) {
	setupTestEnv(t)

	defaults := settings.Config.UserDefaults
	defaults.UI.Locale = "en"
	state.SetEffectiveUserDefaultsForTest(t, defaults)
	if err := state.PatchUserDefaultsEnforced([]byte(`{"ui":{"locale":true}}`)); err != nil {
		t.Fatal(err)
	}

	admin := createPostTestAdmin(t)

	body := []byte(`{"which":[],"data":{"username":"enforced","password":"secret","loginMethod":"password","locale":"zhCN","lockPassword":true}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	status, err := usersPostHandler(rec, req, &Context{User: admin})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("usersPostHandler() status=%d err=%v", status, err)
	}
	got, err := state.GetUserByUsername("enforced")
	if err != nil {
		t.Fatal(err)
	}
	if got.Locale != "en" {
		t.Errorf("locale = %q, want enforced en", got.Locale)
	}
	if !got.LockPassword {
		t.Errorf("lockPassword = false, want true (not enforced)")
	}
}
