package indexing

import (
	"errors"
	"io/fs"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
)

// skippable reports whether a per-entry stat error means the entry may be omitted from the
// listing. Only permission and not-found errors are skippable; I/O errors (EIO, ESTALE) and
// timeouts still fail the listing so a flaky mount cannot make the scanner prune files that
// still exist. Skipped entries are treated as absent by the indexer (same as ls -l).
func skippable(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist)
}

func skipIsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

// unreadableWarned limits "skipping unreadable entry" warnings to once per path; routine scans
// revisit the same entry forever. The map is cleared once it holds maxUnreadableWarned paths.
var (
	unreadableMu        sync.Mutex
	unreadableWarned    = map[string]struct{}{}
	maxUnreadableWarned = 10000
)

func firstUnreadableWarning(path string) bool {
	unreadableMu.Lock()
	defer unreadableMu.Unlock()
	if _, seen := unreadableWarned[path]; seen {
		return false
	}
	if len(unreadableWarned) >= maxUnreadableWarned {
		clear(unreadableWarned)
	}
	unreadableWarned[path] = struct{}{}
	return true
}

func warnSkippedUnreadable(path string, err error) {
	if skipIsNotExist(err) {
		return // removed between readdir and stat
	}
	if !firstUnreadableWarning(path) {
		logger.Debugf("skipping unreadable entry %s: %v", path, err)
		return
	}
	logger.Warningf("skipping unreadable entry %s: %v", path, err)
}

// errAllEntriesVanished is returned when every name from readdir was skipped as not-found,
// which usually means the directory was renamed or replaced during the scan (not a true empty dir).
var errAllEntriesVanished = fs.ErrNotExist

func allNamesSkippedAsNotExist(nameCount, listedCount, skippedNotExist int) bool {
	return nameCount > 0 && listedCount == 0 && skippedNotExist == nameCount
}
