package access_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

type failingRevokePersister struct{}

func (failingRevokePersister) SaveAccessRule(string, string, *access.AccessRule) error { return nil }
func (failingRevokePersister) DeleteAccessRule(string, string) error                   { return nil }
func (failingRevokePersister) SaveGroup(string, access.StringSet) error                { return nil }
func (failingRevokePersister) DeleteGroup(string) error                                { return nil }
func (failingRevokePersister) SaveRevokedToken(string, int64) error                    { return nil }
func (failingRevokePersister) PersistImmediateTokenRevocation(string) error {
	return errors.New("simulated revocation persistence failure")
}
func (failingRevokePersister) DeleteRevokedToken(string) error      { return nil }
func (failingRevokePersister) SaveHashedToken(string, uint64, bool) error { return nil }
func (failingRevokePersister) DeleteHashedToken(string) error         { return nil }
func (failingRevokePersister) DeleteHashedTokensByUserID(uint64) error { return nil }

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

func TestRevokeTokenRollsBackMemoryOnPersistenceFailure(t *testing.T) {
	store, _ := createTestStorage(t)
	store.SetSQLStore(failingRevokePersister{})

	if err := store.AddSessionToken("tok", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeToken("tok"); err == nil {
		t.Fatal("expected RevokeToken to propagate persistence failure")
	}
	if store.IsTokenRevoked("tok") {
		t.Fatal("failed revocation must not leave in-memory revoked state")
	}
	if _, ok := store.GetHashedTokenInfo("tok"); !ok {
		t.Fatal("failed revocation must restore owner mapping")
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
