package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// newShareMoveTestStore opens an isolated SQLite store, points the package-level
// sqlDb at it, and resets the share caches so shares can be seeded via CreateShare.
func newShareMoveTestStore(t *testing.T) *sqldb.SQLStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	store, _, err := sqldb.NewSQLStoreWithOptions(dbPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	prevDB := sqlDb
	prevByHash := sharesByHash
	prevByPath := sharesByPath
	sqlDb = store
	sharesByHash = make(map[string]*share.Share)
	sharesByPath = make(map[string][]string)
	t.Cleanup(func() {
		_ = store.Close()
		sqlDb = prevDB
		sharesByHash = prevByHash
		sharesByPath = prevByPath
	})
	return store
}

func TestRewritePathPrefix(t *testing.T) {
	cases := []struct {
		name      string
		sharePath string
		want      string
		matches   bool
	}{
		{"file under moved dir", "/photos/2024/one.jpg", "/archive/photos/2024/one.jpg", true},
		{"directory keeps trailing slash", "/photos/2024/", "/archive/photos/2024/", true},
		{"exact dir match", "/photos/", "/archive/photos/", true},
		{"exact file-form match", "/photos", "/archive/photos", true},
		{"substring sibling does not match", "/backup/photos/", "/backup/photos/", false},
	}
	for _, tc := range cases {
		got, matches := utils.RewritePathPrefix(tc.sharePath, "/photos/", "/archive/photos/")
		if matches != tc.matches || got != tc.want {
			t.Errorf("%s: RewritePathPrefix(%q) = (%q, %v), want (%q, %v)",
				tc.name, tc.sharePath, got, matches, tc.want, tc.matches)
		}
	}
}

// TestUpdateSharesForMovedResource exercises the production updater end to end:
// seeded shares are rewritten in the cache, the path index, and SQL.
func TestUpdateSharesForMovedResource(t *testing.T) {
	store := newShareMoveTestStore(t)

	const oldSource = "/srv/photos"
	const newSource = "/srv/archive"
	seed := []share.Share{
		{ShareColumns: share.ShareColumns{Hash: "file", Path: "/photos/2024/one.jpg"}, SourcePath: oldSource},
		{ShareColumns: share.ShareColumns{Hash: "dir", Path: "/photos/2024/"}, SourcePath: oldSource},
		{ShareColumns: share.ShareColumns{Hash: "exact", Path: "/photos/"}, SourcePath: oldSource},
		{ShareColumns: share.ShareColumns{Hash: "sibling", Path: "/backup/photos/"}, SourcePath: oldSource},
		{ShareColumns: share.ShareColumns{Hash: "othersrc", Path: "/photos/2024/"}, SourcePath: "/srv/other"},
	}
	for i := range seed {
		s := seed[i]
		if err := CreateShare(&s); err != nil {
			t.Fatalf("seed share %s: %v", s.Hash, err)
		}
	}

	updated, err := UpdateSharesForMovedResource(oldSource, "/photos", newSource, "/archive/photos/")
	if err != nil {
		t.Fatal(err)
	}
	updatedSet := make(map[string]bool, len(updated))
	for _, h := range updated {
		updatedSet[h] = true
	}
	for _, want := range []string{"file", "dir", "exact"} {
		if !updatedSet[want] {
			t.Fatalf("expected share %s in updated hashes %v", want, updated)
		}
	}
	if len(updated) != 3 {
		t.Fatalf("expected 3 updated shares, got %v", updated)
	}

	check := func(hash, wantPath, wantSource string) {
		t.Helper()
		s, getErr := GetShare(hash)
		if getErr != nil {
			t.Fatalf("GetShare(%s): %v", hash, getErr)
		}
		if s.Path != wantPath || s.SourcePath != wantSource {
			t.Fatalf("share %s = (path %q, source %q), want (path %q, source %q)",
				hash, s.Path, s.SourcePath, wantPath, wantSource)
		}
	}
	check("file", "/archive/photos/2024/one.jpg", newSource)
	check("dir", "/archive/photos/2024/", newSource)
	check("exact", "/archive/photos/", newSource)
	check("sibling", "/backup/photos/", oldSource)
	check("othersrc", "/photos/2024/", "/srv/other")

	// Writes went through to SQL, not just the cache.
	persisted, err := store.ListAllShares()
	if err != nil {
		t.Fatal(err)
	}
	persistedPaths := make(map[string]string, len(persisted))
	for _, l := range persisted {
		persistedPaths[l.Hash] = l.Path
	}
	for hash, want := range map[string]string{
		"file":  "/archive/photos/2024/one.jpg",
		"dir":   "/archive/photos/2024/",
		"exact": "/archive/photos/",
	} {
		if persistedPaths[hash] != want {
			t.Fatalf("persisted share %s path = %q, want %q", hash, persistedPaths[hash], want)
		}
	}

	// The path index tracks the new location.
	moved, err := GetSharesByPath(newSource+"/", "/archive/photos/2024/")
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || moved[0].Hash != "dir" {
		t.Fatalf("GetSharesByPath(new dir) = %+v, want dir share", moved)
	}
}
