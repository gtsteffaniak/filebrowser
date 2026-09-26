package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// BackfillHashedTokensFromUserRecords registers SHA256(raw JWT) → user_id for stored API tokens.
func BackfillHashedTokensFromUserRecords() error {
	if sqlDb == nil {
		return nil
	}
	list, err := sqlDb.ListUsers()
	if err != nil {
		return err
	}
	added := 0
	for _, user := range list {
		if user == nil || user.ID == 0 {
			continue
		}
		n, err := backfillUserTokenHashes(user)
		if err != nil {
			return fmt.Errorf("backfill hashed token for user %s: %w", user.Username, err)
		}
		added += n
	}
	if added > 0 {
		logger.Infof("Backfilled %d hashed token mapping(s) from user API token records", added)
	}
	return nil
}

// BackfillUserTokenHashesOnStore writes hash mappings for a user's stored token strings (migration helper).
func BackfillUserTokenHashesOnStore(store *sqldb.SQLStore, user *users.User) error {
	if store == nil || user == nil || user.ID == 0 {
		return nil
	}
	for _, tok := range user.Tokens {
		raw := tok.Token
		if raw == "" {
			raw = tok.Key
		}
		if raw == "" {
			continue
		}
		if err := store.SaveHashedToken(utils.HashSHA256(raw), user.ID, false); err != nil {
			return err
		}
	}
	return nil
}

// backfillUserTokenHashes maps each stored named API token to its owner. A
// persistence failure is returned so initialization aborts instead of leaving a
// mapping that silently stops working after restart.
func backfillUserTokenHashes(user *users.User) (int, error) {
	if accessDb == nil || user == nil || user.ID == 0 {
		return 0, nil
	}
	added := 0
	for _, tok := range user.Tokens {
		raw := tok.Token
		if raw == "" {
			raw = tok.Key
		}
		if raw == "" {
			continue
		}
		if _, ok := accessDb.GetUserIDFromToken(raw); ok {
			continue
		}
		if err := AddApiToken(raw, user.ID); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
