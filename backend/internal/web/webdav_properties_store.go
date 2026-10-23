package web

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"golang.org/x/net/webdav"
)

const davPropertyMaxBytes = 64 * 1024
const davPropertyMaxCount = 128

type davPropertyStore struct {
	mu sync.Mutex
	db *sql.DB
}

func openDAVPropertyStore(filename string) (*davPropertyStore, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(filename); err == nil && !st.Mode().IsRegular() {
		return nil, errors.New("property database must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute}).String() + "?mode=rw&_busy_timeout=5000"
	db, err := sql.Open(sqldb.SqliteDriver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS dav_properties (resource TEXT PRIMARY KEY NOT NULL, properties BLOB NOT NULL)`); err != nil {
		db.Close()
		return nil, err
	}
	return &davPropertyStore{db: db}, nil
}
func (s *davPropertyStore) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }
func validDAVPropertyKey(key string) bool {
	return strings.HasPrefix(key, "/") && path.Clean(key) == key
}
func decodeDAVProperties(data []byte) (map[xml.Name]webdav.Property, error) {
	props := make(map[xml.Name]webdav.Property)
	if len(data) == 0 {
		return props, nil
	}
	var list []webdav.Property
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, p := range list {
		p.InnerXML = append([]byte(nil), p.InnerXML...)
		props[p.XMLName] = p
	}
	return props, nil
}
func (s *davPropertyStore) Get(key string) (map[xml.Name]webdav.Property, error) {
	if !validDAVPropertyKey(key) {
		return nil, errors.New("invalid property resource key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var data []byte
	err := s.db.QueryRow(`SELECT properties FROM dav_properties WHERE resource=?`, key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return make(map[xml.Name]webdav.Property), nil
	}
	if err != nil {
		return nil, err
	}
	return decodeDAVProperties(data)
}
func davPropertyPatchStatus(patches []webdav.Proppatch, status int) []webdav.Propstat {
	stat := webdav.Propstat{Status: status}
	for _, patch := range patches {
		for _, p := range patch.Props {
			stat.Props = append(stat.Props, webdav.Property{XMLName: p.XMLName})
		}
	}
	return []webdav.Propstat{stat}
}
func (s *davPropertyStore) Patch(key string, patches []webdav.Proppatch) ([]webdav.Propstat, error) {
	if !validDAVPropertyKey(key) {
		return nil, errors.New("invalid property resource key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var data []byte
	err = tx.QueryRow(`SELECT properties FROM dav_properties WHERE resource=?`, key).Scan(&data)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	props, err := decodeDAVProperties(data)
	if err != nil {
		return nil, err
	}
	for _, patch := range patches {
		for _, p := range patch.Props {
			if patch.Remove {
				delete(props, p.XMLName)
			} else {
				p.InnerXML = append([]byte(nil), p.InnerXML...)
				props[p.XMLName] = p
			}
		}
	}
	if len(props) > davPropertyMaxCount {
		return davPropertyPatchStatus(patches, http.StatusForbidden), nil
	}
	list := make([]webdav.Property, 0, len(props))
	for _, p := range props {
		list = append(list, p)
	}
	data, err = json.Marshal(list)
	if err != nil {
		return nil, err
	}
	if len(data) > davPropertyMaxBytes {
		return davPropertyPatchStatus(patches, http.StatusInsufficientStorage), nil
	}
	if len(props) == 0 {
		_, err = tx.Exec(`DELETE FROM dav_properties WHERE resource=?`, key)
	} else {
		_, err = tx.Exec(`INSERT INTO dav_properties(resource,properties) VALUES(?,?) ON CONFLICT(resource) DO UPDATE SET properties=excluded.properties`, key, data)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return davPropertyPatchStatus(patches, http.StatusOK), nil
}
func davPropertyWithin(key, root string) bool {
	return key == root || strings.HasPrefix(key, strings.TrimSuffix(root, "/")+"/")
}
func (s *davPropertyStore) Move(oldKey, newKey string) error {
	if !validDAVPropertyKey(oldKey) || !validDAVPropertyKey(newKey) {
		return errors.New("invalid property resource key")
	}
	if oldKey == newKey {
		return nil
	}
	if davPropertyWithin(newKey, oldKey) || davPropertyWithin(oldKey, newKey) {
		return errors.New("overlapping property move")
	}
	return s.rewrite(oldKey, newKey)
}
func (s *davPropertyStore) Delete(key string) error {
	if !validDAVPropertyKey(key) {
		return errors.New("invalid property resource key")
	}
	return s.rewrite(key, "")
}
func (s *davPropertyStore) Copy(oldKey, newKey string) error {
	if !validDAVPropertyKey(oldKey) || !validDAVPropertyKey(newKey) || davPropertyWithin(newKey, oldKey) || davPropertyWithin(oldKey, newKey) {
		return errors.New("invalid property copy")
	}
	return s.rewrite(oldKey, newKey, true)
}
func (s *davPropertyStore) rewrite(oldKey, newKey string, copying ...bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query(`SELECT resource,properties FROM dav_properties WHERE resource=? OR instr(resource,?)=1 OR (?<>'' AND (resource=? OR instr(resource,?)=1))`, oldKey, strings.TrimSuffix(oldKey, "/")+"/", newKey, newKey, strings.TrimSuffix(newKey, "/")+"/")
	if err != nil {
		return err
	}
	type record struct {
		key  string
		data []byte
	}
	var remove []string
	var move []record
	for rows.Next() {
		var key string
		var data []byte
		if err = rows.Scan(&key, &data); err != nil {
			rows.Close()
			return err
		}
		if davPropertyWithin(key, oldKey) {
			if len(copying) == 0 || !copying[0] {
				remove = append(remove, key)
			}
			if newKey != "" {
				move = append(move, record{newKey + strings.TrimPrefix(key, oldKey), append([]byte(nil), data...)})
			}
		} else if newKey != "" && davPropertyWithin(key, newKey) {
			remove = append(remove, key)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range remove {
		if _, err = tx.Exec(`DELETE FROM dav_properties WHERE resource=?`, key); err != nil {
			return err
		}
	}
	for _, r := range move {
		if _, err = tx.Exec(`INSERT INTO dav_properties(resource,properties) VALUES(?,?)`, r.key, r.data); err != nil {
			return err
		}
	}
	return tx.Commit()
}
