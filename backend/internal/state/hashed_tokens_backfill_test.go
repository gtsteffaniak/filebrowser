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

func (failingTokenPersister) SaveHashedToken(string, uint64, bool) error {
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
