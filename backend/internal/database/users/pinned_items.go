package users

import "slices"

// Add records a pinned item name under sourcePath and indexDirPath.
func (p PinnedItems) Add(sourcePath, indexDirPath, name string) {
	if p == nil {
		return
	}
	if p[sourcePath] == nil {
		p[sourcePath] = make(map[string][]string)
	}
	items := p[sourcePath][indexDirPath]
	if slices.Contains(items, name) {
		return
	}
	p[sourcePath][indexDirPath] = append(items, name)
}

// Remove deletes a pinned item name and prunes empty map entries.
func (p PinnedItems) Remove(sourcePath, indexDirPath, name string) {
	if p == nil {
		return
	}
	byDir, ok := p[sourcePath]
	if !ok {
		return
	}
	items := byDir[indexDirPath]
	idx := slices.Index(items, name)
	if idx < 0 {
		return
	}
	items = append(items[:idx], items[idx+1:]...)
	if len(items) == 0 {
		delete(byDir, indexDirPath)
	} else {
		byDir[indexDirPath] = items
	}
	if len(byDir) == 0 {
		delete(p, sourcePath)
	}
}

// PinnedNamesForDirectory returns pinned item names for a source/index directory path.
func (u *User) PinnedNamesForDirectory(sourcePath, indexDirPath string) []string {
	if u == nil || len(u.PinnedItems) == 0 {
		return nil
	}
	byDir, ok := u.PinnedItems[sourcePath]
	if !ok {
		return nil
	}
	names := byDir[indexDirPath]
	if len(names) == 0 {
		return nil
	}
	out := make([]string, len(names))
	copy(out, names)
	return out
}

// EnsurePinnedItems returns a non-nil map for mutation.
func (u *User) EnsurePinnedItems() PinnedItems {
	if u.PinnedItems == nil {
		u.PinnedItems = make(PinnedItems)
	}
	return u.PinnedItems
}
