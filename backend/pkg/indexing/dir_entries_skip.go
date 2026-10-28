package indexing

import (
	"errors"
	"io/fs"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
)

// Skip policy for per-entry stat errors while listing a directory.
//
// One entry that cannot be stat'ed should not fail the whole listing. On a CIFS or SMB mount a Windows
// junction can point at a target the share user cannot read, and the whole folder would otherwise
// disappear from the UI and the index. Like `ls -l`, we skip just that entry.
//
// Only permission and not-exist errors skip an entry. Any other error (EIO, ESTALE, timeouts) still
// fails the listing: the scanner prunes entries it does not see, so skipping on a flaky mount would
// delete index rows for files that still exist.
//
// A skipped entry is treated as absent by the scanner: it is pruned from the index and its size is
// left out of the folder total, the same as `ls` shows it as unreadable.

// maxUnreadableWarned bounds the set of paths already warned about. Routine scans revisit the same
// entries forever, so the set is cleared when it fills rather than growing without limit.
var maxUnreadableWarned = 10000

var (
	unreadableMu     sync.Mutex
	unreadableWarned = map[string]struct{}{}
)

// skipEntry reports whether a stat error for dirPath/name should skip that entry rather than fail the
// listing. Permission errors are logged once per path (then at debug level), so a persistently
// unreadable entry does not flood the log on every scan.
func skipEntry(dirPath, name string, err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if !errors.Is(err, fs.ErrPermission) {
		return false
	}
	path := dirPath + "/" + name
	unreadableMu.Lock()
	_, seen := unreadableWarned[path]
	if !seen {
		if len(unreadableWarned) >= maxUnreadableWarned {
			clear(unreadableWarned)
		}
		unreadableWarned[path] = struct{}{}
	}
	unreadableMu.Unlock()
	if seen {
		logger.Debugf("skipping unreadable entry %s: %v", path, err)
	} else {
		logger.Warningf("skipping unreadable entry %s: %v", path, err)
	}
	return true
}

// checkNotAllVanished fails a listing in which every one of total names was skipped as not-exist.
// That is what a directory renamed or removed mid-scan looks like when entries are stat'ed by path.
// Returning an empty success there would make the scanner prune all of its children.
func checkNotAllVanished(dirPath string, total, vanished int) error {
	if total > 0 && vanished == total {
		return &fs.PathError{Op: "readdir", Path: dirPath, Err: fs.ErrNotExist}
	}
	return nil
}
