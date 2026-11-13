package web

import (
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// accessPermittedAtRealPath reports whether username may access the resolved on-disk path
// (after symlink resolution). DENY rules apply to the target index path, not only the link.
func accessPermittedAtRealPath(idx *indexing.Index, username, realPath string, isDir bool) bool {
	if idx == nil || username == "" || realPath == "" {
		return false
	}
	indexPath := idx.MakeIndexPath(realPath, isDir).String()
	return state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(indexPath, true), username)
}

// isDirAtRealPath returns whether an existing path is a directory; missing paths are treated as files.
func isDirAtRealPath(realPath string) bool {
	st, err := os.Stat(realPath)
	if err != nil {
		return false
	}
	return st.IsDir()
}
