package access_test

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func TestSessionAndApiTokenMetadata(t *testing.T) {
	store, _ := createTestStorage(t)

	if err := store.AddSessionToken("session-token", 42); err != nil {
		t.Fatal(err)
	}
	sessionInfo, ok := store.GetHashedTokenInfo("session-token")
	if !ok {
		t.Fatal("expected session token mapping")
	}
	if !sessionInfo.IsSession || sessionInfo.UserID != 42 {
		t.Fatalf("session token info = %+v, want user 42 session=true", sessionInfo)
	}

	if err := store.AddApiToken("api-token", 42); err != nil {
		t.Fatal(err)
	}
	apiInfo, ok := store.GetHashedTokenInfo("api-token")
	if !ok {
		t.Fatal("expected api token mapping")
	}
	if apiInfo.IsSession {
		t.Fatalf("api token info = %+v, want session=false", apiInfo)
	}
}

func TestRevokeTokenIsImmediate(t *testing.T) {
	store, _ := createTestStorage(t)

	if err := store.AddSessionToken("tok", 1); err != nil {
		t.Fatal(err)
	}
	if store.IsTokenRevoked("tok") {
		t.Fatal("token must not be revoked before RevokeToken")
	}
	if err := store.RevokeToken("tok"); err != nil {
		t.Fatal(err)
	}
	if !store.IsTokenRevoked("tok") {
		t.Fatal("RevokeToken must take effect immediately")
	}
	if _, ok := store.GetHashedTokenInfo("tok"); ok {
		t.Fatal("immediate revocation must remove the owner mapping")
	}
}

func TestRetireTokenHonorsGraceWindow(t *testing.T) {
	store, _ := createTestStorage(t)

	if err := store.AddSessionToken("tok", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.RetireToken("tok"); err != nil {
		t.Fatal(err)
	}
	if store.IsTokenRevoked("tok") {
		t.Fatal("retired token must remain valid during the grace window")
	}
	if _, ok := store.GetHashedTokenInfo("tok"); !ok {
		t.Fatal("retired token mapping must remain during the grace window")
	}

	// Simulate the grace window elapsing.
	store.RevokedTokens[utils.HashSHA256("tok")] = time.Now().Add(-3 * time.Minute).Unix()
	if !store.IsTokenRevoked("tok") {
		t.Fatal("retired token must be revoked once the grace window elapses")
	}
}
