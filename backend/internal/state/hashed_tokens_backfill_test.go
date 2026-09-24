package state

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
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
