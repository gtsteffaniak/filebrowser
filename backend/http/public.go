package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
	"github.com/gtsteffaniak/filebrowser/users"
)

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, path := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		if err != nil {
			return errToStatus(err), err
		}
		if link.Hash == "" {
			return errToStatus(err), err
		}
		var status int
		status, err = authenticateShareRequest(r, link)
		if err != nil || status != 0 {
			return status, err
		}
		fmt.Println("auth status")
		d.user = &users.PublicUser
		fmt.Println("ok passing", d.user.Scope, link.Path, path)
		realPath, isDir, err := files.GetRealPath(d.user.Scope, link.Path, path)
		if err != nil {
			return http.StatusNotFound, err
		}
		fmt.Println("realpath", realPath, isDir)
		file, err := files.FileInfoFaster(files.FileOptions{
			Path:       realPath,
			IsDir:      isDir,
			Modify:     d.user.Perm.Modify,
			Expand:     true,
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

func ifPathWithName(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")
	id = pathElements[0]
	allButFirst := path.Join(pathElements[1:]...)
	return id, allButFirst
}

var publicShareHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, ok := d.raw.(*files.FileInfo)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type *files.FileInfo")
	}

	file.Path = strings.TrimPrefix(file.Path, settings.Config.Server.Root)
	if file.IsDir {
		return renderJSON(w, r, file)
	}

	return renderJSON(w, r, file)
})

var publicUserGetHandler = func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	// Call the actual handler logic here (e.g., renderJSON, etc.)
	// You may need to replace `fn` with the actual handler logic.
	return renderJSON(w, r, users.PublicUser)
}

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, ok := d.raw.(*files.FileInfo)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type *files.FileInfo")
	}

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
