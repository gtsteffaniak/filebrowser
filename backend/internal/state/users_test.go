package state

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestCreateUserValidateUsername(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	tests := []struct {
		name     string
		username string
		wantErr  string
	}{
		{name: "empty username", username: "", wantErr: "username is empty"},
		{name: "reserved anonymous", username: "anonymous", wantErr: "reserved"},
		{name: "valid username", username: "alice", wantErr: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &users.User{
				FrontendUser: users.FrontendUser{
					Username: tc.username,
				},
			}
			err := CreateUser(u, "password")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("CreateUser(%q): unexpected error: %v", tc.username, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CreateUser(%q): expected error containing %q", tc.username, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("CreateUser(%q): got %v, want error containing %q", tc.username, err, tc.wantErr)
			}
		})
	}
}

func TestPreserveServerManagedFieldsKeepsTokens(t *testing.T) {
	old := &users.User{
		Version: 3,
		Tokens: map[string]users.AuthToken{
			"ci-key": {Name: "ci-key", Token: "jwt-abc"},
		},
		PinnedItems: users.PinnedItems{"src": {"idx": {"a.txt"}}},
	}
	incoming := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
	}

	preserveServerManagedFields(old, incoming)

	if incoming.Tokens == nil || incoming.Tokens["ci-key"].Token != "jwt-abc" {
		t.Fatalf("expected tokens preserved, got %#v", incoming.Tokens)
	}
	if incoming.PinnedItems == nil {
		t.Fatal("expected pinned items preserved")
	}
	if incoming.Version != 3 {
		t.Fatalf("expected version preserved, got %d", incoming.Version)
	}
}

func TestPreserveServerManagedFieldsClearsTOTPWhenDisabled(t *testing.T) {
	old := &users.User{
		TOTPSecret: "secret",
		TOTPNonce:  "nonce",
	}
	incoming := &users.User{
		FrontendUser: users.FrontendUser{OtpEnabled: false},
	}

	preserveServerManagedFields(old, incoming)

	if incoming.TOTPSecret != "" || incoming.TOTPNonce != "" {
		t.Fatalf("expected TOTP not preserved when disabled, got secret=%q nonce=%q", incoming.TOTPSecret, incoming.TOTPNonce)
	}
}

func TestPreserveServerManagedFieldsPreservesTOTPOnFullSave(t *testing.T) {
	old := &users.User{
		TOTPSecret: "secret",
		TOTPNonce:  "nonce",
	}
	incoming := &users.User{
		FrontendUser: users.FrontendUser{
			Username:   "alice",
			OtpEnabled: true,
		},
	}

	preserveServerManagedFields(old, incoming)

	if incoming.TOTPSecret != "secret" || incoming.TOTPNonce != "nonce" {
		t.Fatalf("expected TOTP preserved on full save, got secret=%q nonce=%q", incoming.TOTPSecret, incoming.TOTPNonce)
	}
}

func TestUpdateUserFullPatchPreservesTOTPSecret(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				Locale: "en",
			},
		},
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}
	u.TOTPSecret = "persisted-secret"
	u.TOTPNonce = "persisted-nonce"
	u.OtpEnabled = true
	if err = UpdateUser(u, "", "TOTPSecret", "TOTPNonce", "OtpEnabled"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username:   "alice",
			OtpEnabled: true,
			NonAdminEditable: users.NonAdminEditable{
				Locale: "de",
			},
		},
	}
	if err = UpdateUser(patch, "", "all"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TOTPSecret != "persisted-secret" || loaded.TOTPNonce != "persisted-nonce" {
		t.Fatalf("expected TOTP preserved after full patch, got secret=%q nonce=%q", loaded.TOTPSecret, loaded.TOTPNonce)
	}
	if !loaded.OtpEnabled {
		t.Fatal("expected OtpEnabled to remain true after full patch")
	}
}

func TestUpdateUserPatchPreservesBackendSourcePermissions(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	scopePerms := users.SourceFilePermissions{View: true, Download: true, Modify: true}
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			Permissions: users.Permissions{Admin: true},
		},
		BackendScopes: []users.BackendScope{{Path: "/data/a", Scope: "/", Permissions: scopePerms}},
		Version:     users.SourcePermissionsMigrationVersion,
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "alice",
		},
		BackendScopes: []users.BackendScope{{Path: "/data/a", Scope: "/", Permissions: scopePerms}},
		Version:       users.SourcePermissionsMigrationVersion,
	}
	if err = UpdateUser(patch, "", "backendScopes"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.BackendScopes) == 0 || !reloaded.BackendScopes[0].Permissions.View || !reloaded.BackendScopes[0].Permissions.Modify {
		t.Fatalf("expected perms preserved after patch, got %#v", reloaded.BackendScopes)
	}
}

