package web

import (
	"sync"
	"time"
)

// wopiLockDuration is how long a WOPI lock lasts without a refresh, as the
// protocol specifies.
const wopiLockDuration = 30 * time.Minute

// wopiLockTable holds the locks editors take on files. It lives in memory, like
// the OnlyOffice document keys: FileBrowser is a single process, and a lock
// lost on restart is re-taken by the editor on its next LOCK or tolerated by
// PutFile (see checkPut).
type wopiLockTable struct {
	mu    sync.Mutex
	locks map[string]wopiLock
	now   func() time.Time
}

type wopiLock struct {
	id      string
	expires time.Time
}

func newWopiLockTable() *wopiLockTable {
	return &wopiLockTable{locks: map[string]wopiLock{}, now: time.Now}
}

var wopiLocks = newWopiLockTable()

// current returns the live lock id on fileID, or "" when unlocked. Caller
// holds t.mu.
func (t *wopiLockTable) current(fileID string) string {
	l, ok := t.locks[fileID]
	if !ok {
		return ""
	}
	if t.now().After(l.expires) {
		delete(t.locks, fileID)
		return ""
	}
	return l.id
}

// Each operation returns ok, and on a conflict the lock currently held ("" when
// none), which the handler sends back in X-WOPI-Lock with a 409.

// lock takes the lock, or refreshes it when the same id already holds it.
func (t *wopiLockTable) lock(fileID, id string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.current(fileID)
	if cur != "" && cur != id {
		return false, cur
	}
	t.locks[fileID] = wopiLock{id: id, expires: t.now().Add(wopiLockDuration)}
	return true, ""
}

// unlockAndRelock swaps oldID for id, atomically.
func (t *wopiLockTable) unlockAndRelock(fileID, oldID, id string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.current(fileID)
	if cur != oldID || cur == "" {
		return false, cur
	}
	t.locks[fileID] = wopiLock{id: id, expires: t.now().Add(wopiLockDuration)}
	return true, ""
}

func (t *wopiLockTable) refresh(fileID, id string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.current(fileID)
	if cur == "" || cur != id {
		return false, cur
	}
	t.locks[fileID] = wopiLock{id: id, expires: t.now().Add(wopiLockDuration)}
	return true, ""
}

func (t *wopiLockTable) unlock(fileID, id string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.current(fileID)
	if cur == "" || cur != id {
		return false, cur
	}
	delete(t.locks, fileID)
	return true, ""
}

func (t *wopiLockTable) get(fileID string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.current(fileID)
}

// checkPut decides whether a PutFile carrying lockID may write. A file locked
// by someone else refuses. An unlocked file accepts: the protocol would have
// the host refuse a non-empty file written without a lock, but an editor that
// outlived a FileBrowser restart holds a lock this table forgot, and refusing
// it would lose the user's changes. The per-document write mutex still
// serialises the writes themselves.
func (t *wopiLockTable) checkPut(fileID, lockID string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.current(fileID)
	if cur != "" && cur != lockID {
		return false, cur
	}
	return true, ""
}
