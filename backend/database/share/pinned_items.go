package share

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

// PinnedItems maps share-relative directory paths to pinned item names.
type PinnedItems map[string][]string

// Add records a pinned item name under shareRelDir.
func (p PinnedItems) Add(shareRelDir, name string) {
	if p == nil {
		return
	}
	items := p[shareRelDir]
	if slices.Contains(items, name) {
		return
	}
	p[shareRelDir] = append(items, name)
}

// Remove deletes a pinned item name and prunes empty map entries.
func (p PinnedItems) Remove(shareRelDir, name string) {
	if p == nil {
		return
	}
	items := p[shareRelDir]
	idx := slices.Index(items, name)
	if idx < 0 {
		return
	}
	items = append(items[:idx], items[idx+1:]...)
	if len(items) == 0 {
		delete(p, shareRelDir)
	} else {
		p[shareRelDir] = items
	}
}

// NamesForDirectory returns pinned item names for a share-relative directory path.
func (p PinnedItems) NamesForDirectory(shareRelDir string) []string {
	if len(p) == 0 {
		return nil
	}
	names := p[shareRelDir]
	if len(names) == 0 {
		return nil
	}
	out := make([]string, len(names))
	copy(out, names)
	return out
}

// EnsurePinnedItems returns a non-nil map for mutation.
func (l *Link) EnsurePinnedItems() PinnedItems {
	if l.PinnedItems == nil {
		l.PinnedItems = make(PinnedItems)
	}
	return l.PinnedItems
}

// ShareRelativeDir maps a source index directory path to a share-relative directory path.
// link.Path is the index path of the share root (e.g. "/share/").
func (l *Link) ShareRelativeDir(indexDirPath string) (string, error) {
	shareRootIndex := utils.AddTrailingSlashIfNotExists(l.Path)
	indexDirPath = utils.AddTrailingSlashIfNotExists(indexDirPath)
	if !strings.HasPrefix(indexDirPath, shareRootIndex) {
		return "", fmt.Errorf("path is outside share scope")
	}

	rel := strings.TrimPrefix(indexDirPath, shareRootIndex)
	if rel == "" {
		return "/", nil
	}
	return utils.AddTrailingSlashIfNotExists("/" + strings.TrimPrefix(rel, "/")), nil
}
