package web

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func TestReplaceSessionTokenKeepsOldTokenWhenMintFails(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "rotation-user", users.Permissions{Api: true})
	oldToken := testSessionToken(t, user, time.Hour)

	invalidUser := *user
	invalidUser.ID = 0
	if _, err := replaceSessionToken(oldToken, &invalidUser); err == nil {
		t.Fatal("expected mint failure for user without an id")
	}
	if state.IsTokenRevoked(oldToken) {
		t.Fatal("old token must not be retired when replacement minting fails")
	}
	if _, _, ok := state.HashedTokenOwner(oldToken); !ok {
		t.Fatal("old token mapping must survive a failed replacement")
	}
}

func TestReplaceSessionTokenRetiresOldWithGrace(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "rotation-user-2", users.Permissions{Api: true})
	oldToken := testSessionToken(t, user, time.Hour)

	newToken, err := replaceSessionToken(oldToken, user)
	if err != nil {
		t.Fatal(err)
	}
	if newToken == oldToken {
		t.Fatal("expected a new session token")
	}
	if _, isSession, ok := state.HashedTokenOwner(newToken); !ok || !isSession {
		t.Fatalf("new token must be registered as a session: ok=%v isSession=%v", ok, isSession)
	}
	// The Playwright failures came from revoking the old token while in-flight
	// requests still carried it. It must stay valid during the grace window.
	if state.IsTokenRevoked(oldToken) {
		t.Fatal("old token must remain valid during the retirement grace window")
	}
	if _, _, ok := state.HashedTokenOwner(oldToken); !ok {
		t.Fatal("old token mapping must remain during the grace window")
	}
}
