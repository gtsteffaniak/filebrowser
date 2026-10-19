package access

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// filesystemOps abstracts directory reads for alias resolution (overridden in tests).
type filesystemOps interface {
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	SameFile(a, b fs.FileInfo) bool
}

type osFilesystemOps struct{}

func (osFilesystemOps) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (osFilesystemOps) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (osFilesystemOps) SameFile(a, b fs.FileInfo) bool           { return os.SameFile(a, b) }

var fsOps filesystemOps = osFilesystemOps{}

// resolveFilesystemIndexPath maps an index path to the spelling stored on disk so
// access rules match case variants and other filesystem name aliases.
// Returns an error when the filesystem cannot be read; callers must deny access (fail closed).
func resolveFilesystemIndexPath(sourceRoot string, indexPath utils.IndexPath) (utils.IndexPath, error) {
	rel := strings.TrimPrefix(strings.TrimSpace(indexPath.String()), "/")
	if rel == "" {
		return indexPath, nil
	}

	parts := strings.Split(rel, "/")
	current := sourceRoot
	resolved := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		name, next, err := resolvePathSegment(current, part)
		if err != nil {
			return utils.IndexPath{}, err
		}
		resolved = append(resolved, name)
		current = next
	}
	return utils.IndexPathFromNormalized("/"+strings.Join(resolved, "/"), indexPath.IsDir), nil
}

func resolvePathSegment(parent, part string) (resolvedName, nextParent string, err error) {
	candidate := filepath.Join(parent, part)
	_, lerr := fsOps.Lstat(candidate)
	if errors.Is(lerr, fs.ErrNotExist) {
		return part, candidate, nil
	}
	if lerr != nil {
		return "", "", lerr
	}

	entries, rerr := fsOps.ReadDir(parent)
	if rerr != nil {
		return "", "", rerr
	}

	for _, e := range entries {
		if e.Name() == part {
			name := e.Name()
			return name, filepath.Join(parent, name), nil
		}
	}

	candInfo, lerr := fsOps.Lstat(candidate)
	if lerr != nil {
		if errors.Is(lerr, fs.ErrNotExist) {
			return part, candidate, nil
		}
		return "", "", lerr
	}

	foldMatches, err := matchingEntryNames(entries, parent, part, candInfo, true)
	if err != nil {
		return "", "", err
	}
	if len(foldMatches) == 1 {
		name := foldMatches[0]
		return name, filepath.Join(parent, name), nil
	}
	if len(foldMatches) > 1 {
		return "", "", fmt.Errorf("ambiguous case-insensitive match for %q under %q", part, parent)
	}

	sameMatches, err := matchingEntryNames(entries, parent, part, candInfo, false)
	if err != nil {
		return "", "", err
	}
	if len(sameMatches) == 1 {
		name := sameMatches[0]
		return name, filepath.Join(parent, name), nil
	}
	if len(sameMatches) > 1 {
		return "", "", fmt.Errorf("ambiguous filesystem alias for %q under %q", part, parent)
	}

	return "", "", fmt.Errorf("filesystem alias for %q under %q could not be resolved", part, parent)
}

func matchingEntryNames(entries []fs.DirEntry, parent, part string, candInfo fs.FileInfo, caseFoldOnly bool) ([]string, error) {
	matches := make([]string, 0, 1)
	for _, e := range entries {
		if caseFoldOnly && !strings.EqualFold(e.Name(), part) {
			continue
		}
		entryPath := filepath.Join(parent, e.Name())
		ei, eerr := fsOps.Lstat(entryPath)
		if eerr != nil {
			if errors.Is(eerr, fs.ErrNotExist) {
				continue
			}
			return nil, eerr
		}
		if fsOps.SameFile(candInfo, ei) {
			matches = append(matches, e.Name())
		}
	}
	return matches, nil
}
