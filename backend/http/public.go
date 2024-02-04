package http

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/share"
)

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, path := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
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
		if path == "/" {
			path = link.Path
		} else if strings.HasPrefix("/"+path, link.Path) {
			path = "/" + path
		} else {
			path = link.Path + "/" + path
		}

		file, err := files.FileInfoFaster(files.FileOptions{
			Fs:         publicUser.Fs,
			Path:       "/home/graham" + path,
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

func ifPathWithName(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")
	id = pathElements[0]
	allButFirst := path.Join(pathElements[1:]...)
	if len(pathElements) == 1 {
		allButFirst = "/"
	}
	return id, allButFirst
}

var publicShareHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {

	file := d.raw.(*files.FileInfo)
	file.Path = strings.TrimPrefix(file.Path, "/home/graham")
	if file.IsDir {
		return renderJSON(w, r, file)
	}

	return renderJSON(w, r, file)
})

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file := d.raw.(*files.FileInfo)
	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
})

// http://vdebian.ghome.net:8080/api/public/dl/J06PsKgp/Pictures/2023/05/20230515_011801_1CACC659.mp4?inline=true
// http://vdebian.ghome.net:8080/api/public/dl/Vv6RHMYv/Pictures/2023/05/20230513_203846_8313230F.png?inline=true
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
