package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestApplyProfileToUser_UseCustomShareDefaults(t *testing.T) {
	t.Run("legacy profile without key defaults to enabled", func(t *testing.T) {
		u := &users.User{Username: "alice"}
		if err := ApplyProfileToUser(u, []byte(`{"account":{}}`)); err != nil {
			t.Fatal(err)
		}
		if !u.UseCustomShare {
			t.Fatal("expected UseCustomShare to default to true for profiles without the key")
		}
	})

	t.Run("explicit false is respected", func(t *testing.T) {
		u := &users.User{Username: "alice"}
		if err := ApplyProfileToUser(u, []byte(`{"account":{"useCustomShare":false}}`)); err != nil {
			t.Fatal(err)
		}
		if u.UseCustomShare {
			t.Fatal("expected UseCustomShare to be false when explicitly disabled")
		}
	})

	t.Run("round trips through profile JSON", func(t *testing.T) {
		u := &users.User{Username: "alice", UseCustomShare: false}
		b, err := ProfileJSONFromUser(u)
		if err != nil {
			t.Fatal(err)
		}
		var out users.User
		if err := ApplyProfileToUser(&out, b); err != nil {
			t.Fatal(err)
		}
		if out.UseCustomShare {
			t.Fatal("expected false to survive a profile round trip")
		}
	})
}
