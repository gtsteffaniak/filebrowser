package settings

import "testing"

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
