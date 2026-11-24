package state

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestCreateShareRejectsPersistedExpiredHash(t *testing.T) {
	store := newAuthSigningKeyTestStore(t)

	// An expired share that was persisted but not loaded into the cache, as after a restart.
	expired := &share.Share{
		ShareColumns: share.ShareColumns{
			Hash:   "expired-custom-hash",
			Path:   "/owner/",
			Expire: time.Now().Add(-time.Hour).Unix(),
		},
		SourcePath: "/src",
		UserID:     1,
		Version:    1,
	}
	if err := store.SaveShare(expired); err != nil {
		t.Fatal(err)
	}

	if !ShareHashExists(expired.Hash) {
		t.Fatal("expected ShareHashExists to report the persisted expired share")
	}

	attacker := &share.Share{
		ShareColumns: share.ShareColumns{
			Hash:   expired.Hash,
			Path:   "/attacker/",
			Expire: 0,
		},
		SourcePath: "/src",
		UserID:     2,
		Version:    1,
	}
	if err := CreateShare(attacker); err == nil {
		t.Fatal("expected CreateShare to reject a hash that is persisted but not cached")
	}

	row, err := store.GetShareByHash(expired.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if row.UserID != 1 || row.Path != "/owner/" {
		t.Fatalf("persisted share was overwritten: userID=%d path=%q", row.UserID, row.Path)
	}
}
