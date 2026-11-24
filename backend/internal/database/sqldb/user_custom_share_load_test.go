package sqldb

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// A stored user with no profile (saved before profiles existed) must load the admin default
// for custom share URLs, not the zero value.
func TestFinishUserLoadProfilelessUserInheritsCustomShareDefault(t *testing.T) {
	prev := settings.Config.UserDefaults.Account.AllowCustomShareURL
	t.Cleanup(func() { settings.Config.UserDefaults.Account.AllowCustomShareURL = prev })

	load := func(t *testing.T, def *bool) bool {
		t.Helper()
		settings.Config.UserDefaults.Account.AllowCustomShareURL = def
		u := &users.User{Username: "legacy"}
		if err := finishUserLoad(u, []byte(`{}`)); err != nil {
			t.Fatalf("finishUserLoad: %v", err)
		}
		return u.AllowCustomShareURL
	}

	f := false
	tr := true
	if got := load(t, &f); got {
		t.Fatal("admin default false was not applied to a profile-less stored user")
	}
	if got := load(t, &tr); !got {
		t.Fatal("admin default true was not applied to a profile-less stored user")
	}
	if got := load(t, nil); got {
		t.Fatal("unset default must fall back to disabled for a profile-less stored user")
	}
}
