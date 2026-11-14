package access

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// PermittedAtRealPath reports whether username may access realPath after symlink
// resolution. DENY rules apply to the target index path, not only the link path.
func (s *Storage) PermittedAtRealPath(index *indexing.Index, username, realPath string, isDir bool) bool {
	if s == nil || index == nil || username == "" || realPath == "" {
		return false
	}
	lexical := realPath
	if !strings.HasPrefix(realPath, index.Path) {
		root, err := filepath.EvalSymlinks(index.Path)
		if err != nil {
			return false
		}
		rel, err := filepath.Rel(root, realPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false
		}
		lexical = index.Path
		if rel != "." {
			lexical = filepath.Join(index.Path, rel)
		}
	}
	indexPath := index.MakeIndexPath(lexical, isDir).String()
	return s.Permitted(index.Path, utils.IndexPathFromNormalized(indexPath, true), username)
}

// IsDirAtRealPath reports whether an existing path is a directory; missing paths are treated as files.
func IsDirAtRealPath(realPath string) bool {
	st, err := os.Stat(realPath)
	if err != nil {
		return false
	}
	return st.IsDir()
}
