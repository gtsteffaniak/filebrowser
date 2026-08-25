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
