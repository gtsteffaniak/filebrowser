package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

// resetShareCacheForAliasTest gives the test a fresh, empty share cache (the test store does not
// load persisted shares) and restores empty maps afterward.
func resetShareCacheForAliasTest(t *testing.T) {
	t.Helper()
	sharesMux.Lock()
	sharesByHash = make(map[string]*share.Share)
	sharesByAlias = make(map[string]string)
	sharesByPath = make(map[string][]string)
	sharesMux.Unlock()
	t.Cleanup(func() {
		sharesMux.Lock()
		sharesByHash = make(map[string]*share.Share)
		sharesByAlias = make(map[string]string)
		sharesByPath = make(map[string][]string)
		sharesMux.Unlock()
	})
}

func newAliasTestShare(hash, alias string) *share.Share {
	return &share.Share{
		ShareColumns: share.ShareColumns{
			Hash:  hash,
			Alias: alias,
			Path:  "/owner/",
		},
		SourcePath: "/src",
		UserID:     1,
		Version:    1,
	}
}

func TestUpdateShareAliasWriteThrough(t *testing.T) {
	store := newAuthSigningKeyTestStore(t)
	resetShareCacheForAliasTest(t)
	const hash = "alias-test-random-hash-0001"
	if err := CreateShare(newAliasTestShare(hash, "first-custom-alias-0001")); err != nil {
		t.Fatal(err)
	}

	// Change the alias: old alias stops resolving, new alias and random hash both resolve.
	if err := UpdateShare(hash, func(link *share.Share) error {
		link.Alias = "second-custom-alias-0002"
		return nil
	}); err != nil {
		t.Fatalf("UpdateShare set alias: %v", err)
	}
	if _, err := GetShare("first-custom-alias-0001"); err == nil {
		t.Fatal("old alias still resolves after change")
	}
	got, err := GetShare("second-custom-alias-0002")
	if err != nil || got.Hash != hash {
		t.Fatalf("new alias did not resolve to hash: err=%v", err)
	}
	if _, err := GetShare(hash); err != nil {
		t.Fatalf("random hash stopped resolving after alias change: %v", err)
	}
	row, err := store.GetShareByHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if row.Alias != "second-custom-alias-0002" {
		t.Fatalf("persisted alias=%q want second-custom-alias-0002", row.Alias)
	}

	// Clearing the alias reverts the presented identifier to the random hash.
	if err := UpdateShare(hash, func(link *share.Share) error {
		link.Alias = ""
		return nil
	}); err != nil {
		t.Fatalf("UpdateShare clear alias: %v", err)
	}
	if _, err := GetShare("second-custom-alias-0002"); err == nil {
		t.Fatal("cleared alias still resolves")
	}
	cleared, err := GetShare(hash)
	if err != nil {
		t.Fatalf("random hash after clear: %v", err)
	}
	if presented := share.PresentedHash(&cleared); presented != hash {
		t.Fatalf("presented=%q want random hash after clear", presented)
	}
	row, err = store.GetShareByHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if row.Alias != "" {
		t.Fatalf("persisted alias=%q want empty after clear", row.Alias)
	}
}

func TestUpdateShareAliasRejectsConflicts(t *testing.T) {
	newAuthSigningKeyTestStore(t)
	resetShareCacheForAliasTest(t)
	const ownHash = "alias-conflict-owner-hash-01"
	const otherHash = "alias-conflict-other-hash-02"
	if err := CreateShare(newAliasTestShare(ownHash, "own-custom-alias-000001")); err != nil {
		t.Fatal(err)
	}
	if err := CreateShare(newAliasTestShare(otherHash, "other-custom-alias-00002")); err != nil {
		t.Fatal(err)
	}

	setAlias := func(alias string) error {
		return UpdateShare(ownHash, func(link *share.Share) error {
			link.Alias = alias
			return nil
		})
	}

	// Another share's alias, another share's random hash, and the share's own hash are all rejected.
	for _, taken := range []string{"other-custom-alias-00002", otherHash, ownHash} {
		if err := setAlias(taken); err == nil {
			t.Fatalf("expected alias %q to be rejected", taken)
		}
	}
	// A rejected change must leave the previous alias intact in both cache and lookup.
	got, err := GetShare("own-custom-alias-000001")
	if err != nil || got.Hash != ownHash {
		t.Fatalf("own alias lost after rejected change: err=%v", err)
	}
}

func TestPrepSharesForFrontendExposesCustomHash(t *testing.T) {
	newAuthSigningKeyTestStore(t)
	resetShareCacheForAliasTest(t)
	const hash = "prep-alias-random-hash-0001"
	const alias = "prep-alias-custom-value-01"
	if err := CreateShare(newAliasTestShare(hash, alias)); err != nil {
		t.Fatal(err)
	}
	link, err := GetShare(alias)
	if err != nil {
		t.Fatal(err)
	}
	out := PrepSharesForFrontend(nil, nil, "", "", &link)
	if len(out) != 1 {
		t.Fatalf("expected 1 share, got %d", len(out))
	}
	if out[0].CustomHash != alias {
		t.Fatalf("CustomHash=%q want %q", out[0].CustomHash, alias)
	}
	if out[0].Hash != alias {
		t.Fatalf("Hash=%q want presented alias %q", out[0].Hash, alias)
	}
}
