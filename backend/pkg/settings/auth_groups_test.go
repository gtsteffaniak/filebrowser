package settings

import "testing"

func TestApplyAuthCommonDefaults(t *testing.T) {
	empty := AuthCommon{}
	applyAuthCommonDefaults(&empty)
	if empty.GroupsClaim != "groups" {
		t.Fatalf("default groupsClaim = %q, want groups", empty.GroupsClaim)
	}

	custom := AuthCommon{GroupsClaim: "memberOf"}
	applyAuthCommonDefaults(&custom)
	if custom.GroupsClaim != "memberOf" {
		t.Fatalf("custom groupsClaim = %q, want memberOf", custom.GroupsClaim)
	}
}

func TestResolveGroupsClaim(t *testing.T) {
	if got := ResolveGroupsClaim(""); got != "groups" {
		t.Fatalf("ResolveGroupsClaim(\"\") = %q, want groups", got)
	}
	if got := ResolveGroupsClaim("memberOf"); got != "memberOf" {
		t.Fatalf("ResolveGroupsClaim(memberOf) = %q, want memberOf", got)
	}
}

func TestResolveLdapGroupsClaim(t *testing.T) {
	if got := ResolveLdapGroupsClaim(""); got != "memberOf" {
		t.Fatalf("ResolveLdapGroupsClaim(\"\") = %q, want memberOf", got)
	}
	if got := ResolveLdapGroupsClaim("groups"); got != "groups" {
		t.Fatalf("ResolveLdapGroupsClaim(groups) = %q, want groups", got)
	}
}

func TestApplyLdapAuthCommonDefaultsUsesMemberOf(t *testing.T) {
	ldapCfg := LdapConfig{AuthCommon: AuthCommon{}}
	if ldapCfg.GroupsClaim == "" {
		ldapCfg.GroupsClaim = defaultLdapGroupsClaim
	}
	applyAuthCommonDefaults(&ldapCfg.AuthCommon)
	if ldapCfg.GroupsClaim != "memberOf" {
		t.Fatalf("ldap groupsClaim = %q, want memberOf", ldapCfg.GroupsClaim)
	}
}
