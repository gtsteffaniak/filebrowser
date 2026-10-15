package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
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

func TestCreateUserCreateUserDirRespectsFinalAdminPermission(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	root := t.TempDir()
	source := &settings.Source{
		Path: root,
		Config: settings.SourceConfig{
			CreateUserDir: true,
		},
	}
	settings.Config.Server.Sources = []*settings.Source{source}
	settings.Config.Server.SourceMap = map[string]*settings.Source{root: source}

	tests := []struct {
		name      string
		admin     bool
		username  string
		wantScope string
		wantDir   bool
	}{
		{name: "admin keeps source root", admin: true, username: "new-admin", wantScope: "/", wantDir: false},
		{name: "non-admin gets personal directory", admin: false, username: "new-user", wantScope: "/new-user", wantDir: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userDefaultsMu.Lock()
			previousDefaults := userDefaultsDefault
			userDefaultsDefault.Account.Permissions.Admin = tc.admin
			userDefaultsMu.Unlock()
			t.Cleanup(func() {
				userDefaultsMu.Lock()
				userDefaultsDefault = previousDefaults
				userDefaultsMu.Unlock()
			})

			user := &users.User{
				FrontendUser:  users.FrontendUser{Username: tc.username},
				BackendScopes: []users.BackendScope{{Path: root, Scope: "/"}},
			}
			if err := CreateUser(user, "password"); err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			saved, err := GetUserByUsername(tc.username)
			if err != nil {
				t.Fatalf("GetUserByUsername: %v", err)
			}
			if saved.Permissions.Admin != tc.admin {
				t.Fatalf("saved admin=%v, want %v", saved.Permissions.Admin, tc.admin)
			}
			if got := saved.BackendScopes[0].Scope; got != tc.wantScope {
				t.Fatalf("saved scope=%q, want %q", got, tc.wantScope)
			}

			if err := files.MakeUserDirs(&saved, true); err != nil {
				t.Fatalf("MakeUserDirs: %v", err)
			}
			_, statErr := os.Stat(filepath.Join(root, tc.username))
			if tc.wantDir && statErr != nil {
				t.Fatalf("user directory does not exist: %v", statErr)
			}
			if !tc.wantDir && !os.IsNotExist(statErr) {
				t.Fatalf("user directory stat error=%v, want not-exist", statErr)
			}
		})
	}
}

func TestUpdateUserPatchPreservesTOTPSecret(t *testing.T) {
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
	if err = UpdateUser(u, "", "totpSecret", "totpNonce", "otpEnabled"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				Locale: "de",
			},
		},
	}
	if err = UpdateUser(patch, "", "locale"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TOTPSecret != "persisted-secret" || loaded.TOTPNonce != "persisted-nonce" {
		t.Fatalf("expected TOTP preserved after locale patch, got secret=%q nonce=%q", loaded.TOTPSecret, loaded.TOTPNonce)
	}
	if loaded.Locale != "de" {
		t.Fatalf("expected locale updated, got %q", loaded.Locale)
	}
}

func TestUpdateUserRejectsEmptyFields(t *testing.T) {
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
		},
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				Locale: "de",
			},
		},
	}
	if err = UpdateUser(patch, ""); err == nil {
		t.Fatal("expected error when no fields are specified")
	}
}

func TestUpdateUserRejectsAllField(t *testing.T) {
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

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				Locale: "de",
			},
		},
	}
	if err = UpdateUser(patch, "", "all"); err == nil {
		t.Fatal("expected error when which contains all")
	}
}

func TestUpdateUserRejectsUnknownField(t *testing.T) {
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

	patch := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username: "alice",
		},
	}
	if err = UpdateUser(patch, "", "notARealField"); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestUpdateUserRejectsPasswordWithoutPlaintext(t *testing.T) {
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
		},
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	patch := &users.User{ID: u.ID, FrontendUser: users.FrontendUser{Username: "alice"}}
	if err = UpdateUser(patch, "", "password"); err == nil {
		t.Fatal("expected error when password is in which but plaintext is empty")
	}
}

func TestUpdateUserClearsTOTPWhenOtpDisabled(t *testing.T) {
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
			OtpEnabled: true,
		},
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}
	u.TOTPSecret = "persisted-secret"
	u.TOTPNonce = "persisted-nonce"
	if err = UpdateUser(u, "", "totpSecret", "totpNonce", "otpEnabled"); err != nil {
		t.Fatal(err)
	}

	disable := &users.User{
		ID: u.ID,
		FrontendUser: users.FrontendUser{
			Username:   "alice",
			OtpEnabled: false,
		},
	}
	if err = UpdateUser(disable, "", "otpEnabled"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TOTPSecret != "" || loaded.TOTPNonce != "" {
		t.Fatalf("expected TOTP cleared when otpEnabled disabled, got secret=%q nonce=%q", loaded.TOTPSecret, loaded.TOTPNonce)
	}
	if loaded.OtpEnabled {
		t.Fatal("expected otpEnabled false")
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

