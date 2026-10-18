package access

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-cache/cache"
)

// filesystemPathCache caches resolved index paths per source root and input path.
var filesystemPathCache = cache.NewCache[string](1 * time.Minute)

// filesystemOps abstracts directory reads for alias resolution (overridden in tests).
type filesystemOps interface {
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	SameFile(a, b fs.FileInfo) bool
}

type osFilesystemOps struct{}

func (osFilesystemOps) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (osFilesystemOps) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (osFilesystemOps) SameFile(a, b fs.FileInfo) bool { return os.SameFile(a, b) }

var fsOps filesystemOps = osFilesystemOps{}

// resolveFilesystemIndexPath maps an index path to the spelling stored on disk so
// access rules match case variants and other filesystem name aliases.
// Returns an error when the filesystem cannot be read; callers must deny access (fail closed).
func resolveFilesystemIndexPath(sourceRoot string, indexPath utils.IndexPath) (utils.IndexPath, error) {
	rel := strings.TrimPrefix(strings.TrimSpace(indexPath.String()), "/")
	if rel == "" {
		return indexPath, nil
	}
	cacheKey := sourceRoot + "\x00" + indexPath.String()
	if cached, ok := filesystemPathCache.Get(cacheKey); ok {
		return utils.IndexPathFromNormalized(cached, indexPath.IsDir), nil
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
	out := utils.IndexPathFromNormalized("/"+strings.Join(resolved, "/"), indexPath.IsDir)
	filesystemPathCache.Set(cacheKey, out.String())
	return out, nil
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

	for _, e := range entries {
		entryPath := filepath.Join(parent, e.Name())
		ei, eerr := fsOps.Lstat(entryPath)
		if eerr != nil {
			if errors.Is(eerr, fs.ErrNotExist) {
				continue
			}
			return "", "", eerr
		}
		if fsOps.SameFile(candInfo, ei) {
			name := e.Name()
			return name, filepath.Join(parent, name), nil
		}
	}

	for _, e := range entries {
		if strings.EqualFold(e.Name(), part) {
			name := e.Name()
			return name, filepath.Join(parent, name), nil
		}
	}

	return "", "", fmt.Errorf("filesystem alias for %q under %q could not be resolved", part, parent)
}
