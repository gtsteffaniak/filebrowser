package http

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
)

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, subpath := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		linkPath := strings.TrimSuffix(settings.Config.Server.Root, "/") + link.Path + subpath
		if err != nil {
			return errToStatus(err), err
		}
		status, err := authenticateShareRequest(r, link)
		if status != 0 || err != nil {
			return status, err
		}
		publicUser, err := d.store.Users.Get("", "publicUser")
		if err != nil {
			return errToStatus(err), err
		}
		d.user = publicUser
		log.Println("linkPath", linkPath)
		file, err := files.FileInfoFaster(files.FileOptions{
			Fs:         publicUser.Fs,
			Path:       linkPath,
			Modify:     publicUser.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Token:      link.Token,
		})
		if err != nil {
			return errToStatus(err), err
		}
		d.raw = file
		return fn(w, r, d)
	}
}

var withHashFileShort = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, subpath := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		linkPath := link.Path + subpath
		if err != nil {
			return errToStatus(err), err
		}
		status, err := authenticateShareRequest(r, link)
		if status != 0 || err != nil {
			return status, err
		}
		publicUser, err := d.store.Users.Get("", "publicUser")
		if err != nil {
			return errToStatus(err), err
		}
		d.user = publicUser
		log.Println("linkPath", linkPath)
		file, err := files.FileInfoFaster(files.FileOptions{
			Fs:         publicUser.Fs,
			Path:       linkPath,
			Modify:     publicUser.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Token:      link.Token,
		})
		if err != nil {
			return errToStatus(err), err
		}
		d.raw = file
		return fn(w, r, d)
	}
}

// ref to https://github.com/filebrowser/filebrowser/pull/727
// `/api/public/dl/MEEuZK-v/file-name.txt` for old browsers to save file with correct name
func ifPathWithName(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")
	// prevent maliciously constructed parameters like `/api/public/dl/XZzCDnK2_not_exists_hash_name`
	// len(pathElements) will be 1, and golang will panic `runtime error: index out of range`

	switch len(pathElements) {
	case 1:
		return r.URL.Path, "/"
	default:
		return pathElements[0], path.Join("/", path.Join(pathElements[1:]...))
	}
}

var publicShareHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file := d.raw.(*files.FileInfo)
	if file.IsDir {
		return renderJSON(w, r, file)
	}

	return renderJSON(w, r, file)
})

var publicDlHandler = withHashFileShort(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file := d.raw.(*files.FileInfo)
	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
})

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 0, nil
	}

	if r.URL.Query().Get("token") == l.Token {
		return 0, nil
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return 0, err
	}
	if password == "" {
		return http.StatusUnauthorized, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 0, err
	}
	return 0, nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
