package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

	got, err := extractToken(req)
	if err != nil {
		t.Fatalf("extractToken() error = %v", err)
	}
	if got != apiToken {
		t.Fatalf("extractToken() = %q, want %q", got, apiToken)
	}
}

func TestExtractTokenUsesCookieWhenNoAuthorizationHeader(t *testing.T) {
	const cookieToken = "session.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})

	got, err := extractToken(req)
	if err != nil {
		t.Fatalf("extractToken() error = %v", err)
	}
	if got != cookieToken {
		t.Fatalf("extractToken() = %q, want %q", got, cookieToken)
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

	got, err := extractToken(req)
	if err != nil {
		t.Fatalf("extractToken() error = %v", err)
	}
	if got != queryToken {
		t.Fatalf("extractToken() = %q, want %q", got, queryToken)
	}
}
