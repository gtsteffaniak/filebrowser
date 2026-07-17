package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestApplyNamedApiTokenGlobalCaps(t *testing.T) {
	owner := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			Permissions: users.Permissions{
				Admin: true,
				Api:   true,
				Share: true,
			},
		},
		ID: 1,
	}

	t.Run("session token unchanged", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{
			BelongsTo:   1,
			Permissions: users.Permissions{Admin: false},
		}, "WEB_TOKEN_abcd")
		if !user.Permissions.Admin {
			t.Fatal("session token should keep DB admin")
		}
	})

	t.Run("minimal api token unchanged", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{}, "my-key")
		if !user.Permissions.Admin {
			t.Fatal("minimal token should keep DB admin")
		}
	})

	t.Run("custom api token caps globals", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{
			BelongsTo: 1,
			Permissions: users.Permissions{
				Admin:  false,
				Api:    true,
				Share:  true,
				Modify: true,
			},
		}, "customized")
		if user.Permissions.Admin {
			t.Fatal("custom token should cap admin from JWT")
		}
		if !user.Permissions.Api || !user.Permissions.Share {
			t.Fatalf("expected api and share, got %#v", user.Permissions)
		}
	})
}
