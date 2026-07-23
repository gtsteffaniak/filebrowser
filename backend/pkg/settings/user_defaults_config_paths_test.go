package settings

import "testing"

func TestCollectMapLeafPaths_nestedUserDefaults(t *testing.T) {
	raw := map[string]interface{}{
		"listing": map[string]interface{}{
			"showHidden": true,
			"viewMode":   "grid",
		},
		"account": map[string]interface{}{
			"permissions": map[string]interface{}{
				"share": true,
			},
		},
	}
	paths := CollectMapLeafPaths(raw, "")
	want := map[string]struct{}{
		"listing.showHidden":       {},
		"listing.viewMode":         {},
		"account.permissions.share": {},
	}
	if len(paths) != len(want) {
		t.Fatalf("paths=%v want %d leaves", paths, len(want))
	}
	for _, p := range paths {
		if _, ok := want[p]; !ok {
			t.Fatalf("unexpected path %q in %v", p, paths)
		}
	}
}

func TestApplyEnforcementFromPaths(t *testing.T) {
	var enforced UserDefaultsEnforcement
	ApplyEnforcementFromPaths(&enforced, []string{
		"listing.showHidden",
		"account.permissions.share",
	})
	if !enforced.Listing.ShowHidden {
		t.Fatal("expected listing.showHidden enforced")
	}
	if !enforced.Account.Permissions.Share {
		t.Fatal("expected account.permissions.share enforced")
	}
	if enforced.Listing.SingleClick {
		t.Fatal("unexpected listing.singleClick enforced")
	}
}

func TestValidateUserDefaultsPatchNotConfigLocked(t *testing.T) {
	Env.ConfigUserDefaultsSpecified = true
	Env.ConfigUserDefaultsSpecifiedPaths = []string{"listing.showHidden"}
	t.Cleanup(func() {
		Env.ConfigUserDefaultsSpecified = false
		Env.ConfigUserDefaultsSpecifiedPaths = nil
	})

	if err := ValidateUserDefaultsPatchNotConfigLocked([]byte(`{"listing":{"singleClick":true}}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateUserDefaultsPatchNotConfigLocked([]byte(`{"listing":{"showHidden":false}}`)); err == nil {
		t.Fatal("expected config lock error")
	}
}

func TestIsUserDefaultLockedFromConfig(t *testing.T) {
	Env.ConfigUserDefaultsSpecifiedPaths = []string{"ui.darkMode"}
	t.Cleanup(func() {
		Env.ConfigUserDefaultsSpecifiedPaths = nil
	})
	if !IsUserDefaultLockedFromConfig("ui.darkMode") {
		t.Fatal("expected ui.darkMode locked")
	}
	if IsUserDefaultLockedFromConfig("ui.locale") {
		t.Fatal("expected ui.locale unlocked")
	}
}
