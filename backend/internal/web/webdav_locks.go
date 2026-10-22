package web

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

// scopeLockStore is shared by all views of a source. Lock keys must describe
// physical paths, not paths relative to an individual user's scope.
type scopeLockStore struct {
	mu    sync.Mutex
	base  webdav.LockSystem
	roots map[string]scopedLockRecord
}

type scopedLockRecord struct {
	root    string
	expires time.Time
	held    int
}

func lockRecord(root string, now time.Time, duration time.Duration) scopedLockRecord {
	record := scopedLockRecord{root: root}
	if duration >= 0 {
		record.expires = now.Add(duration)
	}
	return record
}

var scopeLockStores sync.Map // webdav.LockSystem -> *scopeLockStore

type scopedLockSystem struct {
	store              *scopeLockStore
	scope              string
	initErr            error
	recursiveMutation  bool
	enforceRequest     bool
	requestMethod      string
	requestSource      string
	requestDestination string
}

func newScopedLockSystem(base webdav.LockSystem, scopePath string, indexRoot ...string) *scopedLockSystem {
	candidate := &scopeLockStore{base: base, roots: make(map[string]scopedLockRecord)}
	actual, _ := scopeLockStores.LoadOrStore(base, candidate)
	store, ok := actual.(*scopeLockStore)
	s := &scopedLockSystem{store: store}
	if !ok || store == nil {
		s.initErr = webdav.ErrLocked
		s.store = candidate
		return s
	}
	root := scopePath
	if len(indexRoot) != 0 {
		root = indexRoot[0]
	}
	lexicalRoot, err := filepath.Abs(root)
	if err != nil {
		s.initErr = err
		return s
	}
	canonicalRoot, err := filepath.EvalSymlinks(lexicalRoot)
	if err != nil {
		s.initErr = err
		return s
	}
	absoluteScope, err := filepath.Abs(scopePath)
	if err != nil {
		s.initErr = err
		return s
	}
	relative, err := filepath.Rel(lexicalRoot, absoluteScope)
	if err != nil || outsideScope(relative) {
		relative, err = filepath.Rel(canonicalRoot, absoluteScope)
		if err != nil || outsideScope(relative) {
			s.initErr = webdav.ErrLocked
			return s
		}
	}
	physicalScope := filepath.Join(canonicalRoot, relative)
	if err := rejectScopeSymlinks(canonicalRoot, physicalScope); err != nil {
		s.initErr = err
		return s
	}
	s.scope = filepath.ToSlash(physicalScope)
	return s
}

func outsideScope(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// The configured root may itself be an OS alias, but aliases inside that root
// are rejected: separate path spellings must never bypass the same lock.
func rejectScopeSymlinks(root, target string) error {
	for component := target; component != root; component = filepath.Dir(component) {
		info, err := os.Lstat(component)
		if err != nil && !os.IsNotExist(err) {
			return webdav.ErrLocked
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return webdav.ErrLocked
		}
		if filepath.Dir(component) == component {
			return webdav.ErrLocked
		}
	}
	return nil
}

func (s *scopedLockSystem) physical(name string) (string, error) {
	if s.initErr != nil {
		return "", webdav.ErrLocked
	}
	// Clean the client path as an absolute path before joining: '..' cannot
	// escape the scope. Names received here have already been URL-decoded.
	physical := path.Join(s.scope, path.Clean("/"+name))
	if err := rejectScopeSymlinks(filepath.FromSlash(s.scope), filepath.FromSlash(physical)); err != nil {
		return "", err
	}
	return physical, nil
}

func (s *scopedLockSystem) relative(root string) (string, bool) {
	if root == s.scope {
		return "/", true
	}
	if s.scope == "/" {
		return root, true
	}
	if strings.HasPrefix(root, s.scope+"/") {
		return strings.TrimPrefix(root, s.scope), true
	}
	return "", false
}

func (s *scopedLockSystem) Create(now time.Time, details webdav.LockDetails) (string, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	for token, record := range s.store.roots {
		if record.held == 0 && !record.expires.IsZero() && !now.Before(record.expires) {
			delete(s.store.roots, token)
		}
	}
	physical, err := s.physical(details.Root)
	if err != nil {
		return "", err
	}
	details.Root = physical
	if s.recursiveMutation && details.Duration < 0 && details.OwnerXML == "" {
		details.ZeroDepth = false
	}
	token, err := s.store.base.Create(now, details)
	if err == nil {
		s.store.roots[token] = lockRecord(details.Root, now, details.Duration)
	}
	return token, err
}

func (s *scopedLockSystem) Confirm(now time.Time, name0, name1 string, conditions ...webdav.Condition) (func(), error) {
	if s.enforceRequest && !s.matchesRequest(name0, name1) {
		return nil, webdav.ErrConfirmationFailed
	}
	for _, condition := range conditions {
		if condition.Not || condition.ETag != "" {
			return nil, webdav.ErrConfirmationFailed
		}
	}
	var err error
	if name0 != "" {
		name0, err = s.physical(name0)
		if err != nil {
			return nil, webdav.ErrConfirmationFailed
		}
	}
	if name1 != "" {
		name1, err = s.physical(name1)
		if err != nil {
			return nil, webdav.ErrConfirmationFailed
		}
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.recursiveMutation {
		for _, record := range s.store.roots {
			if record.held == 0 && !record.expires.IsZero() && !now.Before(record.expires) {
				continue
			}
			for _, name := range []string{name0, name1} {
				if name != "" && record.root != name && (name == "/" || strings.HasPrefix(record.root, name+"/")) {
					return nil, webdav.ErrConfirmationFailed
				}
			}
		}
	}
	// Unsupported conditions fail closed above; positive tokens pass unchanged.
	// Inherited infinite-depth locks stay effective.
	release, err := s.store.base.Confirm(now, name0, name1, conditions...)
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool)
	for _, condition := range conditions {
		record, exists := s.store.roots[condition.Token]
		if exists && !held[condition.Token] {
			record.held++
			s.store.roots[condition.Token] = record
			held[condition.Token] = true
		}
	}
	return func() {
		s.store.mu.Lock()
		defer s.store.mu.Unlock()
		release()
		for token := range held {
			record, exists := s.store.roots[token]
			if exists {
				record.held--
				s.store.roots[token] = record
			}
		}
	}, nil
}

// x/net's tagged If lists can replace the source passed to Confirm with
// an unrelated resource tag. Bind confirmation to the actual request targets.
func (s *scopedLockSystem) matchesRequest(name0, name1 string) bool {
	same := func(a, b string) bool { return path.Clean("/"+a) == path.Clean("/"+b) }
	switch s.requestMethod {
	case "COPY":
		return s.requestDestination != "" && (name0 == "" || same(name0, s.requestDestination)) && name1 != "" && same(name1, s.requestDestination)
	case "MOVE":
		return name0 != "" && same(name0, s.requestSource) && s.requestDestination != "" && name1 != "" && same(name1, s.requestDestination)
	case "PUT", "DELETE", "MKCOL", "PROPPATCH":
		return name0 != "" && same(name0, s.requestSource) && name1 == ""
	default:
		return false
	}
}

func (s *scopedLockSystem) tokenMatchesRequest(root, method string) bool {
	if !s.enforceRequest || s.requestMethod != method {
		return true
	}
	expected, err := s.physical(s.requestSource)
	return err == nil && expected == root
}

func (s *scopedLockSystem) Refresh(now time.Time, token string, duration time.Duration) (webdav.LockDetails, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.initErr != nil {
		return webdav.LockDetails{}, webdav.ErrNoSuchLock
	}
	record, exists := s.store.roots[token]
	relative, visible := s.relative(record.root)
	if !exists || !visible || !s.tokenMatchesRequest(record.root, "LOCK") {
		return webdav.LockDetails{}, webdav.ErrNoSuchLock
	}
	details, err := s.store.base.Refresh(now, token, duration)
	if err != nil {
		if err == webdav.ErrNoSuchLock {
			delete(s.store.roots, token)
		}
		return webdav.LockDetails{}, err
	}
	s.store.roots[token] = lockRecord(record.root, now, duration)
	details.Root = relative
	return details, nil
}

func (s *scopedLockSystem) Unlock(now time.Time, token string) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.initErr != nil {
		return webdav.ErrNoSuchLock
	}
	record, exists := s.store.roots[token]
	if _, visible := s.relative(record.root); !exists || !visible || !s.tokenMatchesRequest(record.root, "UNLOCK") {
		return webdav.ErrNoSuchLock
	}
	err := s.store.base.Unlock(now, token)
	if err == nil || err == webdav.ErrNoSuchLock {
		delete(s.store.roots, token)
	}
	return err
}
