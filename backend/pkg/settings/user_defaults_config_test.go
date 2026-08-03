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
		"listing.showHidden":        {},
		"listing.viewMode":          {},
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

func TestMergeEnforcedPatchAllowsConfigLockedValuePath(t *testing.T) {
	merged, err := MergeEnforcedPatchJSON(UserDefaultsEnforcement{}, []byte(`{"listing":{"showHidden":true}}`))
	if err != nil {
		t.Fatalf("merge enforced patch: %v", err)
	}
	if !merged.Listing.ShowHidden {
		t.Fatal("expected listing.showHidden enforcement true")
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

func TestApplyConfigSpecifiedPathsToUserDefaults_overlaysConfigLeaves(t *testing.T) {
	prevSpecified := Env.ConfigUserDefaultsSpecified
	prevPaths := Env.ConfigUserDefaultsSpecifiedPaths
	t.Cleanup(func() {
		Env.ConfigUserDefaultsSpecified = prevSpecified
		Env.ConfigUserDefaultsSpecifiedPaths = prevPaths
	})

	Env.ConfigUserDefaultsSpecified = true
	Env.ConfigUserDefaultsSpecifiedPaths = []string{"account.permissions.share"}

	stored := UserDefaults{
		Account: UserDefaultsAccount{
			Permissions: UserDefaultsAccountPermissions{
				Share: false,
				Api:   true,
			},
			LockPassword: true,
		},
		Listing: UserDefaultsListing{
			ShowHidden: false,
		},
	}
	config := UserDefaults{
		Account: UserDefaultsAccount{
			Permissions: UserDefaultsAccountPermissions{
				Share: true,
			},
		},
	}

	merged, err := ApplyConfigSpecifiedPathsToUserDefaults(stored, config)
	if err != nil {
		t.Fatalf("ApplyConfigSpecifiedPathsToUserDefaults: %v", err)
	}
	if !merged.Account.Permissions.Share {
		t.Fatal("expected share permission from config")
	}
	if !merged.Account.Permissions.Api {
		t.Fatal("expected api permission preserved from stored defaults")
	}
	if !merged.Account.LockPassword {
		t.Fatal("expected lockPassword preserved from stored defaults")
	}
	if merged.Listing.ShowHidden {
		t.Fatal("expected unrelated listing defaults preserved")
	}
}

func TestUserDefaultsPatchJSONForPaths_extractsNestedLeaves(t *testing.T) {
	source := UserDefaults{
		Account: UserDefaultsAccount{
			Permissions: UserDefaultsAccountPermissions{
				Share: true,
			},
		},
	}
	patchJSON, err := UserDefaultsPatchJSONForPaths(source, []string{"account.permissions.share"})
	if err != nil {
		t.Fatalf("UserDefaultsPatchJSONForPaths: %v", err)
	}
	merged, err := MergeUserDefaultsPatchJSON(UserDefaults{
		Account: UserDefaultsAccount{
			Permissions: UserDefaultsAccountPermissions{Share: false},
		},
	}, patchJSON)
	if err != nil {
		t.Fatalf("MergeUserDefaultsPatchJSON: %v", err)
	}
	if !merged.Account.Permissions.Share {
		t.Fatal("expected share true after patch merge")
	}
}

func TestUserDefaultsLockedFromConfig(t *testing.T) {
	prevSpecified := Env.ConfigUserDefaultsSpecified
	prevPaths := Env.ConfigUserDefaultsSpecifiedPaths
	t.Cleanup(func() {
		Env.ConfigUserDefaultsSpecified = prevSpecified
		Env.ConfigUserDefaultsSpecifiedPaths = prevPaths
	})

	Env.ConfigUserDefaultsSpecified = false
	Env.ConfigUserDefaultsSpecifiedPaths = nil
	if UserDefaultsLockedFromConfig() {
		t.Fatal("expected unlocked when config has no userDefaults paths")
	}
	Env.ConfigUserDefaultsSpecified = true
	Env.ConfigUserDefaultsSpecifiedPaths = []string{"listing.showHidden"}
	if !UserDefaultsLockedFromConfig() {
		t.Fatal("expected locked when config specifies userDefaults paths")
	}
	if UserDefaultsConfigLockMessage == "" {
		t.Fatal("expected non-empty lock message")
	}
}

func TestUserDefaultsFieldConfigLockMessage(t *testing.T) {
	got := UserDefaultsFieldConfigLockMessage("listing.showHidden")
	want := `user default "listing.showHidden" is locked from config file`
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestMergeUserDefaultsPatchJSON_listingOverridePreservesDefaultPreview(t *testing.T) {
	base := UserDefaults{
		Listing: UserDefaultsListing{
			QuickDownload: true,
			ShowHidden:    false,
		},
		Preview: UserDefaultsPreview{
			Image: boolPtr(true),
		},
	}
	patchJSON := []byte(`{"listing":{"showHidden":true}}`)
	merged, err := MergeUserDefaultsPatchJSON(base, patchJSON)
	if err != nil {
		t.Fatalf("MergeUserDefaultsPatchJSON: %v", err)
	}
	if !merged.Listing.ShowHidden {
		t.Fatalf("expected ShowHidden true after merge")
	}
	if !merged.Listing.QuickDownload {
		t.Fatalf("expected QuickDownload preserved from base")
	}
	if merged.Preview.Image == nil || !*merged.Preview.Image {
		t.Fatalf("expected preview.image preserved from base")
	}
}

func TestValidateSinglePropertyUserDefaultsPatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		patch   string
		wantErr bool
	}{
		{"one bool", `{"listing":{"showHidden":true}}`, false},
		{"one string", `{"listing":{"hideFileExt":".exe"}}`, false},
		{"enforced shape", `{"account":{"lockPassword":true}}`, false},
		{"two fields", `{"listing":{"showHidden":true,"singleClick":true}}`, true},
		{"two sections", `{"listing":{"showHidden":true},"sidebar":{"sticky":true}}`, true},
		{"empty", `{}`, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSinglePropertyUserDefaultsPatch([]byte(tc.patch))
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
