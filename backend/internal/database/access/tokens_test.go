package access_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
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
func (failingRevokePersister) PersistTokenRetirement(string, int64, []string) error {
	return nil
}
func (failingRevokePersister) DeleteRevokedToken(string) error { return nil }
func (failingRevokePersister) SaveHashedToken(string, uint64, bool, int64) error {
	return nil
}
func (failingRevokePersister) DeleteHashedToken(string) error          { return nil }
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

// signedTestJWT mints a bare HS256 JWT carrying only an exp claim. The registry
// only reads claims unverified, so any key works.
func signedTestJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()})
	tokenString, err := token.SignedString([]byte("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	return tokenString
}

func TestExpiredTokenResolvesWithinGrace(t *testing.T) {
	store, _ := createTestStorage(t)
	token := signedTestJWT(t, time.Now().Add(-time.Hour))
	if err := store.AddSessionToken(token, 7); err != nil {
		t.Fatal(err)
	}
	info, ok := store.GetHashedTokenInfo(token)
	if !ok || info.UserID != 7 || !info.IsSession {
		t.Fatalf("recently expired token must still resolve within grace, got %+v ok=%v", info, ok)
	}
	if info.ExpiresAt == 0 {
		t.Fatal("expiry must be recorded from the token's exp claim")
	}
}

func TestExpiredTokenPastGraceNeverRegistered(t *testing.T) {
	store, _ := createTestStorage(t)
	token := signedTestJWT(t, time.Now().Add(-2*access.ExpiredTokenGrace))
	if err := store.AddSessionToken(token, 7); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GetHashedTokenInfo(token); ok {
		t.Fatal("token expired past grace must not resolve")
	}
	if _, ok := store.HashedTokens[utils.HashSHA256(token)]; ok {
		t.Fatal("token expired past grace must not be registered")
	}
}

func TestAddHashedTokenPrunesExpired(t *testing.T) {
	store, _, sqlStore := createTestStorageWithSQL(t)
	deadExp := time.Now().Add(-2 * access.ExpiredTokenGrace).Unix()
	deadHash := utils.HashSHA256(signedTestJWT(t, time.Unix(deadExp, 0)))
	store.HashedTokens[deadHash] = access.HashedTokenInfo{UserID: 9, IsSession: true, ExpiresAt: deadExp}
	if err := sqlStore.SaveHashedToken(deadHash, 9, true, deadExp); err != nil {
		t.Fatal(err)
	}

	live := signedTestJWT(t, time.Now().Add(time.Hour))
	if err := store.AddSessionToken(live, 7); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.HashedTokens[deadHash]; ok {
		t.Fatal("registering a new token must prune expired in-memory mappings")
	}
	if _, err := sqlStore.GetUserIDByTokenHash(deadHash); err == nil {
		t.Fatal("registering a new token must prune expired persisted mappings")
	}
}
