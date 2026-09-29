package web
import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// dirTreeSize returns the total size in bytes of all regular files under dir.
// Symlinks contribute their own size, not the size of their target.
func dirTreeSize(dir string) (int64, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return 0, err
	}
	var total int64
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil // unreadable entry: do not fail the whole walk over one stat error
		}
		total += info.Size()
		return nil
	})
	return total, walkErr
}

// sourceQuotaFor returns the storage quota in bytes that user holds for source.
// 0 means unlimited.
func sourceQuotaFor(user *users.User, source string) int64 {
	if user == nil {
		return 0
	}
	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Infof("quota check: no index for source %q; unlimited", source)
		return 0
	}
	quota := int64(0)
	for _, scope := range user.BackendScopes {
		if scope.Path == idx.Path {
			quota = scope.MaxStorageBytes
		}
	}
	logger.Infof("quota check: user %s source %s: idx.Path=%q quota=%d scopes=%d",
		user.Username, source, idx.Path, quota, len(user.BackendScopes))
	return quota
}

// sourceUsage returns the total bytes stored under user's scope directory in source.
func sourceUsage(user *users.User, source string) (int64, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return 0, fmt.Errorf("source %s not found", source)
	}
	scope, err := user.GetScopeForSourceName(source)
	if err != nil {
		return 0, err
	}
	usage, err := dirTreeSize(filepath.Join(idx.Path, scope))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return usage, nil
}

// checkSourceQuota rejects writes that would push user's usage in source past
// their per-source storage quota. incoming is the number of bytes the write
// adds; replaced is the size of an existing file the write overwrites (counted
// as removed first). A quota of 0 means unlimited and skips the check.
func checkSourceQuota(user *users.User, source string, incoming, replaced int64) (int, error) {
	quota := sourceQuotaFor(user, source)
	if quota == 0 {
		return 0, nil
	}
	usage, err := sourceUsage(user, source)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not measure storage usage for source %s: %w", source, err)
	}
	if usage-replaced+incoming > quota {
		return http.StatusRequestEntityTooLarge, fmt.Errorf(
			"storage quota exceeded for source %s (limit %d bytes, current usage %d bytes, this write adds %d bytes)",
			source, quota, usage-replaced, incoming,
		)
	}
	return 0, nil
}
