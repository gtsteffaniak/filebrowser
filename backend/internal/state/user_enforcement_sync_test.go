package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestApplyEnforcedSyncToUser_mutatesProfile(t *testing.T) {
	userDefaultsMu.Lock()
	userDefaultsDefault = settings.UserDefaults{
		Listing: settings.UserDefaultsListing{ShowHidden: true},
	}
	userDefaultsEnforcedDefault = settings.UserDefaultsEnforcement{
		Listing: settings.UserDefaultsListingEnforcement{ShowHidden: true},
	}
	userDefaultsMu.Unlock()

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			NonAdminEditable: users.NonAdminEditable{
				ShowHidden: false,
			},
		},
	}
	if !ApplyEnforcedSyncToUser(u) {
		t.Fatal("expected enforced sync to change profile")
	}
	if !u.ShowHidden {
		t.Fatal("expected ShowHidden true after enforced sync")
	}
	if ApplyEnforcedSyncToUser(u) {
		t.Fatal("expected no further changes when already aligned")
	}
}
