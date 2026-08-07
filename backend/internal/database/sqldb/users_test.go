package sqldb

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestFinishUserLoad_legacyProfileRowPreservesAbsentSettingsFields(t *testing.T) {
	userDataJSON := []byte(`{
		"backendScopes": [],
		"loginMethod": "password",
		"version": 5,
		"showFirstLogin": true,
		"profile": {"ui":{"darkMode":true}},
		"settings": {"darkMode": false}
	}`)

	var loaded users.User
	if err := finishUserLoad(&loaded, userDataJSON); err != nil {
		t.Fatal(err)
	}
	if !loaded.ShowFirstLogin {
		t.Fatal("expected top-level showFirstLogin to be preserved when settings omits it")
	}
	if !loaded.DarkMode {
		t.Fatal("expected profile darkMode=true")
	}
	if len(loaded.PasskeyCredentials) != 0 {
		t.Fatalf("expected no passkeys when settings omits passkeyCredentials, got %#v", loaded.PasskeyCredentials)
	}

	withPasskeys := []byte(`{
		"backendScopes": [],
		"loginMethod": "password",
		"version": 5,
		"showFirstLogin": true,
		"profile": {"ui":{"darkMode":true}},
		"settings": {"passkeyCredentials":[{"id":"Y3JlZC0x"}]}
	}`)
	loaded = users.User{}
	if err := finishUserLoad(&loaded, withPasskeys); err != nil {
		t.Fatal(err)
	}
	if len(loaded.PasskeyCredentials) != 1 {
		t.Fatalf("expected passkey from settings overlay, got %#v", loaded.PasskeyCredentials)
	}

	explicitFalse := []byte(`{
		"backendScopes": [],
		"loginMethod": "password",
		"version": 5,
		"showFirstLogin": true,
		"profile": {"ui":{"darkMode":true}},
		"settings": {"showFirstLogin": false}
	}`)
	loaded = users.User{}
	if err := finishUserLoad(&loaded, explicitFalse); err != nil {
		t.Fatal(err)
	}
	if loaded.ShowFirstLogin {
		t.Fatal("expected settings.showFirstLogin=false to override top-level true")
	}

	explicitEmptySidebar := []byte(`{
		"backendScopes": [],
		"loginMethod": "password",
		"version": 5,
		"profile": {"ui":{"darkMode":true}},
		"settings": {"sidebarLinks":[]}
	}`)
	loaded = users.User{FrontendUser: users.FrontendUser{NonAdminEditable: users.NonAdminEditable{
		SidebarLinks: []users.SidebarLink{{Name: "Docs", Target: "/docs"}},
	}}}
	if err := finishUserLoad(&loaded, explicitEmptySidebar); err != nil {
		t.Fatal(err)
	}
	if len(loaded.SidebarLinks) != 0 {
		t.Fatalf("expected explicit empty sidebarLinks to clear links, got %#v", loaded.SidebarLinks)
	}
}

func TestUserDataRoundTrip_profileSettingsOverlay(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "users.db")
	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	original := &users.User{
		FrontendUser: users.FrontendUser{
			Username:       "legacy-v5",
			LoginMethod:    users.LoginMethodPassword,
			ShowFirstLogin: true,
			NonAdminEditable: users.NonAdminEditable{
				DarkMode: true,
				SidebarLinks: []users.SidebarLink{
					{Name: "Home", Target: "/files"},
				},
				Sorting: users.Sorting{By: "name", Asc: true},
			},
		},
		Version: users.ProfileStorageVersion,
		PasskeyCredentials: []users.WebAuthnCredential{
			{ID: "pk-1"},
		},
	}
	original.ID = 42

	if err = store.CreateUser(original); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.GetUserByUsername("legacy-v5")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ShowFirstLogin {
		t.Fatal("expected showFirstLogin preserved on round-trip")
	}
	if len(loaded.SidebarLinks) != 1 || loaded.SidebarLinks[0].Name != "Home" {
		t.Fatalf("sidebarLinks=%#v", loaded.SidebarLinks)
	}
	if len(loaded.PasskeyCredentials) != 1 {
		t.Fatalf("passkeyCredentials=%#v", loaded.PasskeyCredentials)
	}
	if loaded.Sorting.By != "name" || !loaded.Sorting.Asc {
		t.Fatalf("sorting=%#v", loaded.Sorting)
	}

	var persisted UserData
	raw, err := json.Marshal(userDataForPersist(original))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}

	legacyMissingOverlay := UserData{
		BackendScopes:  persisted.BackendScopes,
		LoginMethod:    persisted.LoginMethod,
		Version:        users.ProfileStorageVersion,
		ShowFirstLogin: true,
		Profile:        persisted.Profile,
		Settings:       json.RawMessage(`{"darkMode":false}`),
	}
	legacyRaw, err := json.Marshal(legacyMissingOverlay)
	if err != nil {
		t.Fatal(err)
	}

	var legacyLoaded users.User
	if err = finishUserLoad(&legacyLoaded, legacyRaw); err != nil {
		t.Fatal(err)
	}
	if !legacyLoaded.ShowFirstLogin {
		t.Fatal("legacy row without settings overlay fields should keep top-level showFirstLogin")
	}
}
