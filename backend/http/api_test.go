package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// TestApiTokenHandlersIncludeShortToken verifies that both the list and detail
// token endpoints return a shortToken equal to utils.WebdavShortPassword(token.Token).
func TestApiTokenHandlersIncludeShortToken(t *testing.T) {
	setupTestEnv(t)

	tokens := map[string]users.AuthToken{
		"first":  {Token: "first-full-token-value", Name: "first"},
		"second": {Token: "second-full-token-value", Name: "second"},
	}
	user := &users.User{
		ID:          1,
		Username:    "apiuser",
		Permissions: users.Permissions{Api: true},
		Tokens:      tokens,
	}
	if err := store.Users.Save(user, true, true); err != nil {
		t.Fatal("failed to save user:", err)
	}

	t.Run("listApiTokensHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/token/list", http.NoBody)
		recorder := httptest.NewRecorder()
		data := &requestContext{user: user}

		status, err := listApiTokensHandler(recorder, req, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, status)
		}

		var got []AuthTokenFrontend
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != len(tokens) {
			t.Fatalf("expected %d tokens, got %d", len(tokens), len(got))
		}

		// Index response entries by name so we can compare each against the stored
		// token, and detect duplicate or omitted names.
		byName := make(map[string]AuthTokenFrontend, len(got))
		for _, tok := range got {
			if _, dup := byName[tok.Name]; dup {
				t.Errorf("duplicate token name in response: %q", tok.Name)
			}
			byName[tok.Name] = tok
		}

		for name, stored := range tokens {
			resp, ok := byName[name]
			if !ok {
				t.Errorf("token %q missing from response", name)
				continue
			}
			if resp.Token != stored.Token {
				t.Errorf("token %q: expected Token %q, got %q", name, stored.Token, resp.Token)
			}
			want := utils.WebdavShortPassword(stored.Token)
			if resp.ShortToken != want {
				t.Errorf("token %q: expected shortToken %q, got %q", name, want, resp.ShortToken)
			}
		}
	})

	t.Run("getApiTokenHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/token?name=first", http.NoBody)
		recorder := httptest.NewRecorder()
		data := &requestContext{user: user}

		status, err := getApiTokenHandler(recorder, req, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, status)
		}

		var got AuthTokenFrontend
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Token != tokens["first"].Token {
			t.Fatalf("expected token %q, got %q", tokens["first"].Token, got.Token)
		}
		want := utils.WebdavShortPassword(got.Token)
		if got.ShortToken != want {
			t.Errorf("expected shortToken %q, got %q", want, got.ShortToken)
		}
	})
}
