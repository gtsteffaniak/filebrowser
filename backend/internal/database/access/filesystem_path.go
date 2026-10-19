package access

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// Access rules match by path. On case-insensitive filesystems, lookups use a
// case-folded rule index and may resolve request paths to indexed on-disk spelling.

// filesystemOps abstracts directory reads for segment resolution (overridden in tests).
type filesystemOps interface {
	ReadDir(name string) ([]fs.DirEntry, error)
}

type osFilesystemOps struct{}

func (osFilesystemOps) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

var fsOps filesystemOps = osFilesystemOps{}

// resolveIndexPathForAccess maps indexPath to the spelling used in the index (and
// on disk) when case-insensitive matching is required. Missing tail segments are
// preserved. Returns an error on ambiguity or unreadable directories (fail closed).
func resolveIndexPathForAccess(sourceRoot string, indexPath utils.IndexPath) (utils.IndexPath, error) {
	rel := strings.TrimPrefix(strings.TrimSpace(indexPath.String()), "/")
	if rel == "" {
		return indexPath, nil
	}

	idx := indexing.GetIndex(sourceRoot)
	sourceName := ""
	if idx != nil {
		sourceName = idx.Name
	}

	parts := strings.Split(rel, "/")
	resolved := make([]string, 0, len(parts))
	parentIndex := "/"
	parentFS := sourceRoot

	for _, part := range parts {
		if part == "" {
			continue
		}
		name, found, err := lookupSegmentName(sourceName, parentIndex, part)
		if err != nil {
			return utils.IndexPath{}, err
		}
		if !found {
			name, err = resolveSegmentFromDir(parentFS, part)
			if err != nil {
				return utils.IndexPath{}, err
			}
		}
		resolved = append(resolved, name)
		parentIndex = indexChildPath(parentIndex, name)
		parentFS = filepath.Join(parentFS, name)
	}

	return utils.IndexPathFromNormalized("/"+strings.Join(resolved, "/"), indexPath.IsDir), nil
}

func lookupSegmentName(sourceName, parentIndexPath, segment string) (string, bool, error) {
	if sourceName == "" {
		return "", false, nil
	}
	db := indexing.GetIndexDB()
	if db == nil {
		return "", false, nil
	}
	children, err := db.GetDirectoryChildren(sourceName, parentIndexPath)
	if err != nil {
		return "", false, err
	}
	var foldMatches []string
	for _, child := range children {
		if child.Name == segment {
			return child.Name, true, nil
		}
		if strings.EqualFold(child.Name, segment) {
			foldMatches = append(foldMatches, child.Name)
		}
	}
	switch len(foldMatches) {
	case 0:
		return "", false, nil
	case 1:
		return foldMatches[0], true, nil
	default:
		return "", false, fmt.Errorf("ambiguous case-insensitive match for %q under %q", segment, parentIndexPath)
	}
}

func resolveSegmentFromDir(parentFS, segment string) (string, error) {
	entries, err := fsOps.ReadDir(parentFS)
	if err != nil {
		return "", err
	}
	var foldMatches []string
	for _, e := range entries {
		if e.Name() == segment {
			return e.Name(), nil
		}
		if strings.EqualFold(e.Name(), segment) {
			foldMatches = append(foldMatches, e.Name())
		}
	}
	switch len(foldMatches) {
	case 0:
		return segment, nil
	case 1:
		return foldMatches[0], nil
	default:
		return "", fmt.Errorf("ambiguous case-insensitive match for %q under %q", segment, parentFS)
	}
}

// resolveAccessIndexPath resolves indexPath when case-folded rules may apply.
func (s *Storage) resolveAccessIndexPath(sourcePath string, indexPath utils.IndexPath) (utils.IndexPath, error) {
	if !s.needsCaseFoldResolution(sourcePath, indexPath) {
		return indexPath, nil
	}
	return resolveIndexPathForAccess(sourcePath, indexPath)
}

func indexChildPath(parent, name string) string {
	if parent == "/" {
		return "/" + name + "/"
	}
	return strings.TrimSuffix(parent, "/") + "/" + name + "/"
}
