package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func setupShareDefaultsHandlerTest(t *testing.T) *users.User {
	t.Helper()
	setupTestEnv(t)
	admin := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "share_defaults_admin",
			Permissions: users.Permissions{Admin: true},
		},
	}
	if err := state.CreateUser(admin, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := state.GetUserByUsername("share_defaults_admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	got.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(&got, "", "permissions"); err != nil {
		t.Fatalf("UpdateUser permissions: %v", err)
	}
	return &got
}

func TestSettingsShareDefaultsPatchHandler_rejectsInvalidType(t *testing.T) {
	admin := setupShareDefaultsHandlerTest(t)
	body := []byte(`{"maxBandwidth":"fast"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/settings/share-defaults", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := settingsShareDefaultsPatchHandler(rec, req, &Context{User: admin})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v want 400", status, err)
	}
}

func TestSettingsShareDefaultsPatchHandler_rejectsOversizedBody(t *testing.T) {
	admin := setupShareDefaultsHandlerTest(t)
	body := []byte(`{"title":"` + strings.Repeat("x", int(maxSettingsPatchBodySize)) + `"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/settings/share-defaults", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := settingsShareDefaultsPatchHandler(rec, req, &Context{User: admin})
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d err=%v want 413", status, err)
	}
}

func TestSettingsShareDefaultsPatchHandler_acceptsCombinedPayload(t *testing.T) {
	admin := setupShareDefaultsHandlerTest(t)
	body := []byte(`{"shareType":"upload","enforced":{"allowModify":true}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/settings/share-defaults", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := settingsShareDefaultsPatchHandler(rec, req, &Context{User: admin})
	if status != http.StatusOK {
		t.Fatalf("status=%d err=%v want 200", status, err)
	}
	got := state.GetShareDefaults()
	if got.ShareType != "upload" {
		t.Fatalf("shareType=%q want upload", got.ShareType)
	}
	enforced := state.GetEnforcedShareDefaults()
	if !enforced.AllowModify {
		t.Fatal("expected allowModify enforced")
	}
}
