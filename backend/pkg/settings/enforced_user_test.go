package settings

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestValidateUserAgainstEnforcedDefaults_skipsAuthManagedAdmin(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "admin-from-oidc"},
	}
	u.Permissions.Admin = true
	defaults := UserDefaults{Account: UserDefaultsAccount{Permissions: UserDefaultsAccountPermissions{Admin: false}}}
	enforced := UserDefaultsEnforcement{
		Account: UserDefaultsAccountEnforcement{
			Permissions: UserDefaultsAccountPermissionsEnforcement{Admin: true},
		},
	}
	if err := ValidateUserAgainstEnforcedDefaults(u, defaults, enforced); err != nil {
		t.Fatalf("auth-granted admin should not be blocked by enforced defaults: %v", err)
	}
}

func TestValidateUserAgainstEnforcedDefaults_rejectsMismatch(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "demo"},
	}
	u.ShowHidden = false
	defaults := UserDefaults{Listing: UserDefaultsListing{ShowHidden: true}}
	enforced := UserDefaultsEnforcement{Listing: UserDefaultsListingEnforcement{ShowHidden: true}}
	err := ValidateUserAgainstEnforcedDefaults(u, defaults, enforced)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	var mismatch ErrEnforcedUserValueMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected ErrEnforcedUserValueMismatch, got %T", err)
	}
	u.ShowHidden = true
	if err := ValidateUserAgainstEnforcedDefaults(u, defaults, enforced); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
}

func TestValidateUserAgainstEnforcedDefaults_skipsAdmin(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "admin"},
	}
	u.Permissions.Admin = true
	u.ShowHidden = false
	defaults := UserDefaults{Listing: UserDefaultsListing{ShowHidden: true}}
	enforced := UserDefaultsEnforcement{Listing: UserDefaultsListingEnforcement{ShowHidden: true}}
	if err := ValidateUserAgainstEnforcedDefaults(u, defaults, enforced); err != nil {
		t.Fatalf("admin should bypass enforced value validation: %v", err)
	}
}

func TestApplyEnforcedDefaultsFrom_onlyEnforcedSubset(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				ShowHidden: false,
				DarkMode:   false,
			},
		},
	}
	defaults := UserDefaults{
		Listing: UserDefaultsListing{ShowHidden: true},
	}
	enforced := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{ShowHidden: true},
	}
	ApplyEnforcedDefaultsFrom(u, defaults, enforced)
	if !u.ShowHidden {
		t.Fatal("expected enforced ShowHidden true")
	}
	if u.DarkMode {
		t.Fatal("expected non-enforced DarkMode unchanged")
	}
}

func TestSyncEnforcedDefaultsOntoUser_detectsChange(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:         "alice",
			NonAdminEditable: users.NonAdminEditable{ShowHidden: false},
		},
	}
	defaults := UserDefaults{Listing: UserDefaultsListing{ShowHidden: true}}
	enforced := UserDefaultsEnforcement{Listing: UserDefaultsListingEnforcement{ShowHidden: true}}
	if !SyncEnforcedDefaultsOntoUser(u, defaults, enforced) {
		t.Fatal("expected sync to report change")
	}
	if !u.ShowHidden {
		t.Fatal("expected ShowHidden updated")
	}
	if SyncEnforcedDefaultsOntoUser(u, defaults, enforced) {
		t.Fatal("expected no change when already synced")
	}
}

func TestEnforcedFieldPaths(t *testing.T) {
	paths := EnforcedFieldPaths(UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{ShowHidden: true},
		UI:      UserDefaultsUIEnforcement{DarkMode: true},
	})
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
}

func TestProfileRoundTrip_expandCollapse(t *testing.T) {
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				ShowHidden:         true,
				DarkMode:           true,
				ShowToolsInSidebar: true,
				Preview: users.Preview{
					Image:              true,
					DisableHideSidebar: true,
				},
			},
			DisableSettings: true,
		},
	}
	p := ProfileFromUser(u)
	var u2 users.User
	u2.Username = "alice"
	ExpandProfileIntoUser(&u2, p)
	if !u2.ShowHidden || !u2.DarkMode || !u2.DisableSettings {
		t.Fatal("round trip lost fields")
	}
}

func TestEnforcedPaths_mergeFromDefaultsProfile(t *testing.T) {
	allTrue := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{ShowHidden: true},
	}
	paths := EnforcedPathSet(allTrue)
	patch, err := profilePatchForPaths(ProfileFromUserDefaults(UserDefaults{
		Listing: UserDefaultsListing{ShowHidden: true},
	}), paths)
	if err != nil {
		t.Fatal(err)
	}
	if !patch.Listing.ShowHidden {
		t.Fatal("expected patch to carry showHidden")
	}
}
