package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"golang.org/x/net/webdav"
)

type mutationTarget struct {
	index       *indexing.Index
	path        string
	removeEntry bool
}

// acquireMutation holds temporary exclusive, infinite-depth locks until the
// filesystem operation finishes. Infinite depth also protects directory children.
// These callers have no WebDAV token and must not bypass a client's lock.
func acquireMutation(targets ...mutationTarget) (func(), error) {
	indices := make([]*indexing.Index, 0, len(targets))
	for _, target := range targets {
		indices = append(indices, target.index)
	}
	releaseGates := indexing.LockDAVMutations(indices...)
	acquired := false
	defer func() {
		if !acquired {
			releaseGates()
		}
	}()

	type lease struct {
		ls          webdav.LockSystem
		token, path string
	}
	leases := []lease{}
	cleanup := func() {
		for i := len(leases) - 1; i >= 0; i-- {
			_ = leases[i].ls.Unlock(time.Now(), leases[i].token)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].path < targets[j].path })
	for _, target := range targets {
		if target.index == nil {
			cleanup()
			return nil, fmt.Errorf("missing mutation index")
		}
		if target.index.WebdavLock == nil {
			continue
		}
		key, err := mutationLockPath(target.index, target.path, target.removeEntry)
		if err != nil {
			cleanup()
			return nil, err
		}
		duplicate := false
		for _, l := range leases {
			if l.ls == target.index.WebdavLock && l.path == key {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		token, err := target.index.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: key, Duration: -1, ZeroDepth: false})
		if err != nil {
			cleanup()
			return nil, err
		}
		leases = append(leases, lease{target.index.WebdavLock, token, key})
	}
	// Confirm both ends together when they use the same lock system. Keeping the
	// confirmation holds prevents even an Unlock from releasing an active lease.
	releases := []func(){}
	for i := 0; i < len(leases); i++ {
		l := leases[i]
		other := ""
		conditions := []webdav.Condition{{Token: l.token}}
		if i+1 < len(leases) && leases[i+1].ls == l.ls {
			i++
			other = leases[i].path
			conditions = append(conditions, webdav.Condition{Token: leases[i].token})
		}
		release, err := l.ls.Confirm(time.Now(), l.path, other, conditions...)
		if err != nil {
			for j := len(releases) - 1; j >= 0; j-- {
				releases[j]()
			}
			cleanup()
			return nil, webdav.ErrLocked
		}
		releases = append(releases, release)
	}
	acquired = true
	return func() {
		defer releaseGates()
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
		cleanup()
	}, nil
}

// mutationLockPath permits aliases in the configured source root (for example
// macOS /var -> /private/var), but rejects aliases inside its exposed tree.
func mutationLockPath(idx *indexing.Index, target string, removal ...bool) (string, error) {
	root, err := filepath.Abs(idx.Path)
	if err != nil {
		return "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		relative, err = filepath.Rel(canonicalRoot, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("%w: mutation outside source", webdav.ErrLocked)
		}
	}
	physical := filepath.Join(canonicalRoot, relative)
	for part := physical; part != canonicalRoot; part = filepath.Dir(part) {
		info, statErr := os.Lstat(part)
		if statErr != nil && !os.IsNotExist(statErr) {
			return "", statErr
		}
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 && !(part == physical && len(removal) > 0 && removal[0]) {
			return "", fmt.Errorf("%w: symlink mutation alias", webdav.ErrLocked)
		}
	}
	return filepath.ToSlash(physical), nil
}

// AcquireDAVMutation protects a staged API write that commits without calling
// WriteFile or MoveResource. The caller must retain the lease through commit.
func AcquireDAVMutation(source, realPath string) (func(), error) {
	return acquireMutation(mutationTarget{index: indexing.GetIndex(source), path: realPath})
}
