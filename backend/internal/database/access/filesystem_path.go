package access

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// resolveFilesystemIndexPath maps an index path to the spelling stored on disk so
// access rules match case variants and other filesystem name aliases.
func resolveFilesystemIndexPath(sourceRoot string, indexPath utils.IndexPath) utils.IndexPath {
	rel := strings.TrimPrefix(strings.TrimSpace(indexPath.String()), "/")
	if rel == "" {
		return indexPath
	}
	parts := strings.Split(rel, "/")
	current := sourceRoot
	resolved := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		candidate := filepath.Join(current, part)
		info, err := os.Lstat(candidate)
		if err != nil {
			resolved = append(resolved, part)
			current = candidate
			continue
		}
		name := info.Name()
		resolved = append(resolved, name)
		current = filepath.Join(current, name)
	}
	return utils.IndexPathFromNormalized("/"+strings.Join(resolved, "/"), true)
}
