package web

import (
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func configureCosmosProxyAuth(t *testing.T) {
	t.Helper()
	settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{
		AuthCommon: settings.AuthCommon{
			Enabled:     true,
			GroupsClaim: "x-cosmos-role",
			UserGroups:  []string{"2", "1"},
			AdminGroup:  "2",
		},
		Header: "X-Cosmos-User",
	}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{}
	})
}

func proxyRequest(t *testing.T, username, role string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if username != "" {
		req.Header.Set("X-Cosmos-User", username)
	}
	if role != "" {
		req.Header.Set("X-Cosmos-Role", role)
	}
	return req
}

func TestSetupProxyUser_CosmosGuestDenied(t *testing.T) {
	setupTestEnv(t)
	configureCosmosProxyAuth(t)

	_, err := SetupProxyUser(proxyRequest(t, "guest-user", "0"), &Context{}, "guest-user")
	if err == nil {
		t.Fatal("expected guest role to be denied")
	}
	if _, getErr := state.GetUserByUsername("guest-user"); getErr == nil {
		t.Fatal("guest user should not be created")
	}
}

func TestSetupProxyUser_CosmosRegularUserAllowed(t *testing.T) {
	setupTestEnv(t)
	configureCosmosProxyAuth(t)

	user, err := SetupProxyUser(proxyRequest(t, "regular-user", "1"), &Context{}, "regular-user")
	if err != nil {
		t.Fatalf("SetupProxyUser: %v", err)
	}
	if user.Permissions.Admin {
		t.Fatal("expected regular user without admin")
	}
}

func TestSetupProxyUser_CosmosAdminPromoted(t *testing.T) {
	setupTestEnv(t)
	configureCosmosProxyAuth(t)

	user, err := SetupProxyUser(proxyRequest(t, "admin-user", "2"), &Context{}, "admin-user")
	if err != nil {
		t.Fatalf("SetupProxyUser: %v", err)
	}
	if !user.Permissions.Admin {
		t.Fatal("expected admin user")
	}
}

func TestSetupProxyUser_CosmosMissingRoleDenied(t *testing.T) {
	setupTestEnv(t)
	configureCosmosProxyAuth(t)

	_, err := SetupProxyUser(proxyRequest(t, "no-role-user", ""), &Context{}, "no-role-user")
	if err == nil {
		t.Fatal("expected missing role header to be denied when userGroups is set")
	}
}

func TestSetupProxyUser_NoGroupConfigAllowsAll(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{
		AuthCommon: settings.AuthCommon{Enabled: true},
		Header:     "X-Cosmos-User",
	}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{}
	})

	user, err := SetupProxyUser(proxyRequest(t, "legacy-user", ""), &Context{}, "legacy-user")
	if err != nil {
		t.Fatalf("SetupProxyUser: %v", err)
	}
	if user.LoginMethod != users.LoginMethodProxy {
		t.Fatalf("login method = %q, want proxy", user.LoginMethod)
	}
}

func TestSetupProxyUser_AdminUsernameStillPromotes(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{
		AuthCommon: settings.AuthCommon{Enabled: true},
		Header:     "X-Cosmos-User",
	}
	settings.Config.Auth.AdminUsername = "legacy-admin"
	t.Cleanup(func() {
		settings.Config.Auth.Methods.ProxyAuth = settings.ProxyAuthConfig{}
		settings.Config.Auth.AdminUsername = ""
	})

	user, err := SetupProxyUser(proxyRequest(t, "legacy-admin", ""), &Context{}, "legacy-admin")
	if err != nil {
		t.Fatalf("SetupProxyUser: %v", err)
	}
	if !user.Permissions.Admin {
		t.Fatal("expected admin promotion via auth.adminUsername")
	}
}
