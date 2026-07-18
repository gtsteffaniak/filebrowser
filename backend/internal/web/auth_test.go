package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestExtractTokenPrefersAuthorizationOverCookie(t *testing.T) {
	const apiToken = "aaa.bbb.ccc"
	const cookieToken = "revoked.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})
	req.Header.Set("Authorization", "Bearer "+apiToken)

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != apiToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, apiToken)
	}
}

func TestExtractTokenUsesCookieWhenNoAuthorizationHeader(t *testing.T) {
	const cookieToken = "session.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != cookieToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, cookieToken)
	}
}

func TestExtractTokenUsesAuthQueryBeforeCookie(t *testing.T) {
	const queryToken = "query.header.token"
	const cookieToken = "session.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users?auth="+queryToken, nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != queryToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, queryToken)
	}
}

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
