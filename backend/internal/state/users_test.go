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
			Username:   "alice",
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
			Username:    "alice",
			Permissions: users.Permissions{Admin: true},
		},
		BackendScopes: []users.BackendScope{{Path: "/data/a", Scope: "/", Permissions: scopePerms}},
		Version:       users.SourcePermissionsMigrationVersion,
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

func TestCreateUserKeepsRequestedPermissions(t *testing.T) {
	original := settings.Config
	t.Cleanup(func() { settings.Config = original })
	settings.Config = settings.SetDefaults(true)
	source := &settings.Source{Path: "/data/a", Name: "a", Config: settings.SourceConfig{
		DefaultEnabled:     true,
		DefaultPermissions: users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true},
	}}
	settings.Config.Server.Sources = []*settings.Source{source}
	settings.Config.Server.SourceMap = map[string]*settings.Source{source.Path: source}
	settings.Config.Server.NameToSource = map[string]*settings.Source{source.Name: source}
	settings.InitializeUserResolvers()

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	requested := users.SourceFilePermissions{View: true, Download: true, Delete: true}
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:       "alice",
			Permissions:    users.Permissions{Admin: true, Share: true, Api: true, Realtime: true},
			FrontendScopes: []users.FrontendScope{{Name: "a", Scope: "/", Permissions: &requested}},
		},
	}
	if err = CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	wantPerms := users.Permissions{Admin: true, Share: true, Api: true, Realtime: true}
	if loaded.Permissions != wantPerms {
		t.Fatalf("permissions = %+v, want %+v", loaded.Permissions, wantPerms)
	}
	if len(loaded.BackendScopes) == 0 {
		t.Fatal("expected a scope for the requested source")
	}
	got := loaded.BackendScopes[0].Permissions
	if got.View != requested.View || got.Download != requested.Download || got.Modify != requested.Modify ||
		got.Create != requested.Create || got.Delete != requested.Delete {
		t.Fatalf("scope permissions = %+v, want %+v", got, requested)
	}
}

// Non-admin create with another account flag set must not keep admin-level source file
// permissions that applyUserSettingsDefaults would assign while defaults still had Admin.
func TestCreateUserNonAdminDoesNotKeepAdminScopePermissions(t *testing.T) {
	original := settings.Config
	t.Cleanup(func() { settings.Config = original })
	settings.Config = settings.SetDefaults(true)
	sourceDefaults := users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true}
	source := &settings.Source{Path: "/data/a", Name: "a", Config: settings.SourceConfig{
		DefaultEnabled:     true,
		DefaultPermissions: sourceDefaults,
	}}
	settings.Config.Server.Sources = []*settings.Source{source}
	settings.Config.Server.SourceMap = map[string]*settings.Source{source.Path: source}
	settings.Config.Server.NameToSource = map[string]*settings.Source{source.Name: source}
	settings.InitializeUserResolvers()

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	userDefaultsMu.Lock()
	previousDefaults := userDefaultsDefault
	userDefaultsDefault.Account.Permissions.Admin = true
	userDefaultsDefault.Account.Permissions.Share = false
	userDefaultsMu.Unlock()
	t.Cleanup(func() {
		userDefaultsMu.Lock()
		userDefaultsDefault = previousDefaults
		userDefaultsMu.Unlock()
	})

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "bob",
			Permissions: users.Permissions{Admin: false, Share: true},
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("bob")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Permissions.Admin {
		t.Fatal("expected non-admin account permissions")
	}
	if !loaded.Permissions.Share {
		t.Fatal("expected requested share permission to be kept")
	}
	if len(loaded.BackendScopes) == 0 {
		t.Fatal("expected default-enabled source scope")
	}
	got := loaded.BackendScopes[0].Permissions
	adminPerms := settings.AdminSourceFilePermissions()
	if got.Delete == adminPerms.Delete && got.Modify == adminPerms.Modify && got.Create == adminPerms.Create {
		t.Fatalf("non-admin kept admin scope permissions: %+v", got)
	}
	want := settings.DefaultSourceFilePermissions()
	if got.View != want.View || got.Download != want.Download || got.Modify != want.Modify ||
		got.Create != want.Create || got.Delete != want.Delete {
		t.Fatalf("scope permissions = %+v, want non-admin defaults %+v", got, want)
	}
}

func setupCreateUserDefaultsTest(t *testing.T) {
	t.Helper()
	original := settings.Config
	t.Cleanup(func() { settings.Config = original })
	settings.Config = settings.SetDefaults(true)
	settings.Config.UserDefaults.Account.Permissions.Share = true
	settings.Config.UserDefaults.Account.Permissions.Api = true
	settings.Config.UserDefaults.Account.Permissions.Realtime = true
	userDefaultsMu.Lock()
	previousUserDefaults := userDefaultsDefault
	userDefaultsDefault.Account.Permissions.Share = true
	userDefaultsDefault.Account.Permissions.Api = true
	userDefaultsDefault.Account.Permissions.Realtime = true
	userDefaultsMu.Unlock()
	t.Cleanup(func() {
		userDefaultsMu.Lock()
		userDefaultsDefault = previousUserDefaults
		userDefaultsMu.Unlock()
	})
	source := &settings.Source{Path: "/data/a", Name: "a", Config: settings.SourceConfig{
		DefaultEnabled:     true,
		DefaultPermissions: users.SourceFilePermissions{View: true, Download: true},
	}}
	settings.Config.Server.Sources = []*settings.Source{source}
	settings.Config.Server.SourceMap = map[string]*settings.Source{source.Path: source}
	settings.Config.Server.NameToSource = map[string]*settings.Source{source.Name: source}
	settings.InitializeUserResolvers()

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
}

// Explicit all-false account permissions must not inherit Admin from user defaults.
func TestCreateUserAllFalsePermissionsDoesNotInheritAdminDefaults(t *testing.T) {
	setupCreateUserDefaultsTest(t)

	userDefaultsMu.Lock()
	previousDefaults := userDefaultsDefault
	userDefaultsDefault.Account.Permissions.Admin = true
	userDefaultsDefault.Account.Permissions.Share = true
	userDefaultsDefault.Account.Permissions.Api = true
	userDefaultsMu.Unlock()
	t.Cleanup(func() {
		userDefaultsMu.Lock()
		userDefaultsDefault = previousDefaults
		userDefaultsMu.Unlock()
	})

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "carol",
			Permissions: users.MarkPermissionsConfigured(users.Permissions{}),
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("carol")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Permissions.Admin || loaded.Permissions.Share || loaded.Permissions.Api || loaded.Permissions.Realtime {
		t.Fatalf("expected all-false account permissions, got %+v", loaded.Permissions)
	}
}

func TestCreateUserAppliesDefaultGlobalPermissionsWhenOmitted(t *testing.T) {
	setupCreateUserDefaultsTest(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "bob",
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("bob")
	if err != nil {
		t.Fatal(err)
	}
	want := users.Permissions{Share: true, Api: true, Realtime: true}
	if loaded.Permissions != want {
		t.Fatalf("permissions = %+v, want %+v", loaded.Permissions, want)
	}
}

func TestCreateUserPartialGlobalPermissionsMask(t *testing.T) {
	setupCreateUserDefaultsTest(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "dave",
			Permissions: users.Permissions{Admin: true},
		},
		PermissionsFieldMask: users.GlobalPermissionsFieldMask{Admin: true},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("dave")
	if err != nil {
		t.Fatal(err)
	}
	want := users.Permissions{Admin: true, Share: true, Api: true, Realtime: true}
	if loaded.Permissions != want {
		t.Fatalf("permissions = %+v, want %+v", loaded.Permissions, want)
	}
}

func TestCreateUserExplicitFalseOverridesDefault(t *testing.T) {
	setupCreateUserDefaultsTest(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "erin",
			Permissions: users.Permissions{Share: false, Api: false},
		},
		PermissionsFieldMask: users.GlobalPermissionsFieldMask{Share: true, Api: true},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("erin")
	if err != nil {
		t.Fatal(err)
	}
	want := users.Permissions{Share: false, Api: false, Realtime: true}
	if loaded.Permissions != want {
		t.Fatalf("permissions = %+v, want %+v", loaded.Permissions, want)
	}
}
