package web

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"golang.org/x/net/webdav"
)

// Properties live beside the application DB, never on a user's FUSE mount.
// Unlike DAV live properties, Microsoft Win32 timestamps are client metadata;
// storing them does not change the source file's POSIX modification time.
func davPropertyDBPath() (string, error) {
	mainDB := settings.Config.Server.DatabaseV2.Path
	if mainDB == "" {
		return "", fmt.Errorf("WebDAV properties require a configured database path")
	}
	return filepath.Join(filepath.Dir(mainDB), "webdav-properties.sqlite"), nil
}

func (ffs *filteredFileSystem) davPropertyKey(name string) (string, error) {
	dir, ok := ffs.fs.(webdav.Dir)
	if !ok {
		return "", fmt.Errorf("WebDAV properties require a scoped filesystem")
	}
	root, err := filepath.EvalSymlinks(string(dir))
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path.Clean("/"+name), "/")))), nil
}

func withDAVPropertyStore(create bool, fn func(*davPropertyStore) error) error {
	name, err := davPropertyDBPath()
	if err != nil {
		if !create {
			return nil
		}
		return err
	}
	if !create {
		if _, statErr := os.Stat(name); os.IsNotExist(statErr) {
			return nil
		} else if statErr != nil {
			return statErr
		}
	}
	store, err := openDAVPropertyStore(name)
	if err != nil {
		return err
	}
	defer store.Close()
	return fn(store)
}

func (ff *filteredFile) DeadProps() (map[xml.Name]webdav.Property, error) {
	key, err := ff.fs.davPropertyKey(ff.requestPath)
	if err != nil {
		return nil, err
	}
	var props map[xml.Name]webdav.Property
	err = withDAVPropertyStore(false, func(store *davPropertyStore) error {
		var getErr error
		props, getErr = store.Get(key)
		return getErr
	})
	return props, err
}

func (ff *filteredFile) Patch(patches []webdav.Proppatch) ([]webdav.Propstat, error) {
	count := 0
	for _, p := range patches {
		count += len(p.Props)
	}
	if count == 0 {
		return []webdav.Propstat{{Status: http.StatusOK}}, nil
	}
	perms := ff.fs.sourceFilePerms()
	copyCreation := ff.fs.httpReq != nil && ff.fs.httpReq.Method == "COPY" && perms.Create && perms.Download
	if !perms.Modify && !copyCreation {
		return nil, os.ErrPermission
	}
	key, err := ff.fs.davPropertyKey(ff.requestPath)
	if err != nil {
		return nil, err
	}
	var result []webdav.Propstat
	err = withDAVPropertyStore(true, func(store *davPropertyStore) error {
		var patchErr error
		result, patchErr = store.Patch(key, patches)
		return patchErr
	})
	return result, err
}

func (ffs *filteredFileSystem) deleteDAVProperties(name string) error {
	key, err := ffs.davPropertyKey(name)
	if err != nil {
		return err
	}
	return withDAVPropertyStore(false, func(store *davPropertyStore) error { return store.Delete(key) })
}

func (ffs *filteredFileSystem) moveDAVProperties(oldName, newName string) error {
	oldKey, err := ffs.davPropertyKey(oldName)
	if err != nil {
		return err
	}
	newKey, err := ffs.davPropertyKey(newName)
	if err != nil {
		return err
	}
	return withDAVPropertyStore(false, func(store *davPropertyStore) error { return store.Move(oldKey, newKey) })
}

// A known unavailable/corrupt/read-only metadata database must fail before
// changing user paths. This is not a distributed filesystem/SQLite transaction.
func preflightDAVProperties() error {
	return withDAVPropertyStore(false, func(store *davPropertyStore) error {
		tx, err := store.db.Begin()
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		_, err = tx.Exec("DELETE FROM dav_properties WHERE 0")
		return err
	})
}
func davDeletePhysicalProperties(key string) error {
	return withDAVPropertyStore(false, func(store *davPropertyStore) error { return store.Delete(filepath.ToSlash(filepath.Clean(key))) })
}
func davMovePhysicalProperties(oldKey, newKey string) error {
	return withDAVPropertyStore(false, func(store *davPropertyStore) error {
		return store.Move(filepath.ToSlash(filepath.Clean(oldKey)), filepath.ToSlash(filepath.Clean(newKey)))
	})
}

func davCopyPhysicalProperties(oldKey, newKey string) error {
	return withDAVPropertyStore(false, func(store *davPropertyStore) error {
		return store.Copy(filepath.ToSlash(filepath.Clean(oldKey)), filepath.ToSlash(filepath.Clean(newKey)))
	})
}

// COPY produces only a status/XML response. Delay that response until metadata
// reconciliation succeeds, so a storage failure cannot be reported as success.
type davCopyResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *davCopyResponse) Header() http.Header { return w.header }
func (w *davCopyResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *davCopyResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}
func (w *davCopyResponse) flushTo(dst http.ResponseWriter) {
	for k, values := range w.header {
		dst.Header()[k] = values
	}
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	dst.WriteHeader(status)
	_, _ = dst.Write(w.body.Bytes())
}
