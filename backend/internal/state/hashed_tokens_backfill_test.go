package state

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// failingTokenPersister fails only SaveHashedToken so backfill error handling can
// be exercised without a live database failure.
type failingTokenPersister struct {
	access.SQLPersister
}

func (failingTokenPersister) SaveHashedToken(string, uint64, bool, int64) error {
	return errors.New("simulated hashed token persistence failure")
}

func TestBackfillHashedTokensPropagatesPersistenceFailure(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	const username = "backfill-user"
	user := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := CreateUser(user, "password"); err != nil {
		t.Fatal(err)
	}
	if err := AddUserToken(username, users.AuthToken{Name: "ci-key", Token: "raw-jwt-token"}); err != nil {
		t.Fatal(err)
	}

	// Force persistence of the backfilled mapping to fail.
	accessDb.SetSQLStore(failingTokenPersister{})

	if err := BackfillHashedTokensFromUserRecords(); err == nil {
		t.Fatal("expected backfill to propagate hashed token persistence failure")
	}
}

// Legacy ApiKeys entries (pre-2.0.8 token format) must also gain a
// hashed_tokens row so strict server-side auth accepts them.
func TestBackfillHashedTokensCoversLegacyApiKeys(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	const username = "backfill-legacy-user"
	user := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := CreateUser(user, "password"); err != nil {
		t.Fatal(err)
	}
	stored, err := GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	legacy := stored
	legacy.ApiKeys = map[string]users.AuthToken{
		"legacy-key": {Name: "legacy-key", Key: "raw-legacy-jwt", Token: "raw-legacy-jwt"},
	}

	added, err := backfillUserTokenHashes(&legacy)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("expected 1 backfilled mapping, got %d", added)
	}
	if _, _, ok := HashedTokenOwner("raw-legacy-jwt"); !ok {
		t.Fatal("legacy ApiKeys token must resolve an owner after backfill")
	}

	// Backfill is idempotent: a second run adds nothing.
	added, err = backfillUserTokenHashes(&legacy)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Fatalf("expected idempotent backfill, added %d", added)
	}
}

// Rows migrated before expiry tracking carry expires_at=0 ("never expires").
// Backfill must reconcile them: live tokens get their real expiry recorded and
// tokens dead past the grace window have their mapping removed entirely.
func TestBackfillReconcilesLegacyTokenExpiry(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	const username = "backfill-expiry-user"
	user := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := CreateUser(user, "password"); err != nil {
		t.Fatal(err)
	}
	stored, err := GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}

	mint := func(exp time.Time) string {
		t.Helper()
		tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": exp.Unix(),
		}).SignedString([]byte("test-key"))
		if err != nil {
			t.Fatal(err)
		}
		return tokenString
	}
	liveExp := time.Now().Add(time.Hour).Unix()
	liveRaw := mint(time.Unix(liveExp, 0))
	deadRaw := mint(time.Now().Add(-2 * ExpiredTokenGrace))

	// Simulate mappings migrated before expiry tracking (expires_at=0).
	for _, raw := range []string{liveRaw, deadRaw} {
		hash := utils.HashSHA256(raw)
		accessDb.HashedTokens[hash] = access.HashedTokenInfo{UserID: stored.ID}
		if err := sqlDb.SaveHashedToken(hash, stored.ID, false, 0); err != nil {
			t.Fatal(err)
		}
	}

	legacy := stored
	legacy.Tokens = map[string]users.AuthToken{
		"live": {Name: "live", Token: liveRaw},
		"dead": {Name: "dead", Token: deadRaw},
	}

	added, err := backfillUserTokenHashes(&legacy)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("expected 1 reconciled mapping, got %d", added)
	}

	// Live token: expiry refreshed from the JWT claim, in memory and in SQL.
	info, ok := accessDb.GetHashedTokenInfo(liveRaw)
	if !ok || info.ExpiresAt != liveExp {
		t.Fatalf("live token info = %+v ok=%v, want expires_at=%d", info, ok, liveExp)
	}
	records, err := sqlDb.GetAllHashedTokens()
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := records[utils.HashSHA256(liveRaw)]
	if !ok || rec.ExpiresAt != liveExp {
		t.Fatalf("persisted live token = %+v ok=%v, want expires_at=%d", rec, ok, liveExp)
	}

	// Dead token: mapping removed from memory and SQL.
	if _, _, ok := HashedTokenOwner(deadRaw); ok {
		t.Fatal("expired-past-grace token mapping must be removed")
	}
	if _, ok := records[utils.HashSHA256(deadRaw)]; ok {
		t.Fatal("expired-past-grace token row must be deleted from sql")
	}

	// Idempotent: a second run changes nothing.
	added, err = backfillUserTokenHashes(&legacy)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Fatalf("expected idempotent backfill, added %d", added)
	}
}
