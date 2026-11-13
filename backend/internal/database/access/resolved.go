package access

import (
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// PermittedAtRealPath reports whether username may access realPath after symlink
// resolution. DENY rules apply to the target index path, not only the link path.
func (s *Storage) PermittedAtRealPath(index *indexing.Index, username, realPath string, isDir bool) bool {
	if s == nil || index == nil || username == "" || realPath == "" {
		return false
	}
	indexPath := index.MakeIndexPath(realPath, isDir).String()
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
