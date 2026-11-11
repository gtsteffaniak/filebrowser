package web

import (
	"cmp"
	"slices"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

type mergeGroupKey struct {
	size   int64
	digest string
}

// mergeGroupsByChecksum merges groups that share the same size and sample digest.
func mergeGroupsByChecksum(groups []duplicateGroupWithChecksums) []duplicateGroupWithChecksums {
	if len(groups) <= 1 {
		return groups
	}

	keyToGroups := make(map[mergeGroupKey][]int)
	for i, group := range groups {
		if group.Checksum == "" {
			continue
		}
		key := mergeGroupKey{size: group.Size, digest: group.Checksum}
		keyToGroups[key] = append(keyToGroups[key], i)
	}

	parent := make([]int, len(groups))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	for _, indices := range keyToGroups {
		for i := 1; i < len(indices); i++ {
			union(indices[0], indices[i])
		}
	}

	groupsByRoot := make(map[int][]int)
	for i := range groups {
		root := find(i)
		groupsByRoot[root] = append(groupsByRoot[root], i)
	}

	merged := make([]duplicateGroupWithChecksums, 0, len(groupsByRoot))
	for _, indices := range groupsByRoot {
		if len(indices) == 1 {
			merged = append(merged, groups[indices[0]])
			continue
		}

		combinedFiles := make([]*indexing.SearchResult, 0)
		var combinedSize int64
		var combinedChecksum string

		for _, idx := range indices {
			group := groups[idx]
			combinedFiles = append(combinedFiles, group.Files...)
			if combinedChecksum == "" {
				combinedChecksum = group.Checksum
			}
			if combinedSize == 0 {
				combinedSize = group.Size
			}
		}

		if len(combinedFiles) >= 2 {
			slices.SortFunc(combinedFiles, func(a, b *indexing.SearchResult) int {
				return strings.Compare(a.Path, b.Path)
			})
			merged = append(merged, duplicateGroupWithChecksums{
				Size:     combinedSize,
				Files:    combinedFiles,
				Checksum: combinedChecksum,
			})
		}
	}

	slices.SortFunc(merged, func(a, b duplicateGroupWithChecksums) int {
		if c := cmp.Compare(b.Size, a.Size); c != 0 {
			return c
		}
		if c := strings.Compare(a.Checksum, b.Checksum); c != 0 {
			return c
		}
		if len(a.Files) == 0 || len(b.Files) == 0 {
			return 0
		}
		return strings.Compare(a.Files[0].Path, b.Files[0].Path)
	})

	return merged
}
