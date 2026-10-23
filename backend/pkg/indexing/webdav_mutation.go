package indexing

import (
	"reflect"
	"sort"
)

// LockDAVMutations serializes filesystem mutations sharing a source, across
// WebDAV and API callers. Callers must retain the gate through metadata changes
// and must not recursively acquire it. Distinct indexes are locked in pointer
// order to avoid cross-source MOVE deadlocks. Nil and duplicate inputs are safe.
//
// This conservative gate trades concurrent writes within a source for reliable
// arbitration of recursive operations; it does not coordinate other processes
// or separately configured sources that expose the same physical files.
func LockDAVMutations(indices ...*Index) func() {
	unique := make(map[*Index]bool, len(indices))
	ordered := make([]*Index, 0, len(indices))
	for _, idx := range indices {
		if idx != nil && !unique[idx] {
			unique[idx] = true
			ordered = append(ordered, idx)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		return reflect.ValueOf(ordered[i]).Pointer() < reflect.ValueOf(ordered[j]).Pointer()
	})
	for _, idx := range ordered {
		idx.DAVMutationMu.Lock()
	}
	return func() {
		for i := len(ordered) - 1; i >= 0; i-- {
			ordered[i].DAVMutationMu.Unlock()
		}
	}
}
