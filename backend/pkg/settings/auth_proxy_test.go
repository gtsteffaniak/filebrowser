package settings

import "testing"

func TestValidateProxyAuth_RequiresGroupsClaimWhenUserGroupsSet(t *testing.T) {
	Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{
		AuthCommon: AuthCommon{
			Enabled:    true,
			UserGroups: []string{"users"},
		},
		Header: "X-Forwarded-User",
	}
	t.Cleanup(func() { Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{} })

	if err := ValidateProxyAuth(); err == nil {
		t.Fatal("expected error when userGroups is set without groupsClaim")
	}
}

func TestValidateProxyAuth_RequiresGroupsClaimWhenAdminGroupSet(t *testing.T) {
	Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{
		AuthCommon: AuthCommon{
			Enabled:    true,
			AdminGroup: "admins",
		},
		Header: "X-Forwarded-User",
	}
	t.Cleanup(func() { Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{} })

	if err := ValidateProxyAuth(); err == nil {
		t.Fatal("expected error when adminGroup is set without groupsClaim")
	}
}

func TestValidateProxyAuth_ValidWithGroupsClaim(t *testing.T) {
	Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{
		AuthCommon: AuthCommon{
			Enabled:     true,
			GroupsClaim: "x-cosmos-role",
			UserGroups:  []string{"1", "2"},
			AdminGroup:  "2",
		},
		Header: "X-Forwarded-User",
	}
	t.Cleanup(func() { Config.Auth.Methods.ProxyAuth = ProxyAuthConfig{} })

	if err := ValidateProxyAuth(); err != nil {
		t.Fatalf("ValidateProxyAuth: %v", err)
	}
}
