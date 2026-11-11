package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

func TestMergeGroupsByChecksumUsesSizeAndDigest(t *testing.T) {
	groups := []duplicateGroupWithChecksums{
		{Size: 100, Checksum: "aaaa", Files: []*indexing.SearchResult{{Path: "/a"}, {Path: "/b"}}},
		{Size: 200, Checksum: "aaaa", Files: []*indexing.SearchResult{{Path: "/c"}, {Path: "/d"}}},
		{Size: 100, Checksum: "aaaa", Files: []*indexing.SearchResult{{Path: "/e"}, {Path: "/f"}}},
	}
	merged := mergeGroupsByChecksum(groups)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged groups, got %d", len(merged))
	}
}
