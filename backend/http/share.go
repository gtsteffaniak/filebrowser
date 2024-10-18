package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/share"
)

func shareListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var (
		s   []*share.Link
		err error
	)
	if d.user.Perm.Admin {
		s, err = store.Share.All()
	} else {
		s, err = store.Share.FindByUserID(d.user.ID)
	}
	if err == errors.ErrNotExist {
		return renderJSON(w, r, []*share.Link{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	sort.Slice(s, func(i, j int) bool {
		if s[i].UserID != s[j].UserID {
			return s[i].UserID < s[j].UserID
		}
		return s[i].Expire < s[j].Expire
	})
	return renderJSON(w, r, s)
}

func shareGetsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	s, err := store.Share.Gets(r.URL.Path, d.user.ID)
	if err == errors.ErrNotExist {
		return renderJSON(w, r, []*share.Link{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, s)
}

func shareDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := strings.TrimSuffix(r.URL.Path, "/")
	hash = strings.TrimPrefix(hash, "/")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	err := store.Share.Delete(hash)
	if err != nil {
		return errToStatus(err), err
	}

	return errToStatus(err), err
}

func sharePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {

	var s *share.Link
	var body share.CreateBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	bytes := make([]byte, 6) //nolint:gomnd
	_, err := rand.Read(bytes)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	str := base64.URLEncoding.EncodeToString(bytes)

	var expire int64 = 0

	if body.Expires != "" {
		//nolint:govet
		num, err := strconv.Atoi(body.Expires)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		var add time.Duration
		switch body.Unit {
		case "seconds":
			add = time.Second * time.Duration(num)
		case "minutes":
			add = time.Minute * time.Duration(num)
		case "days":
			add = time.Hour * 24 * time.Duration(num)
		default:
			add = time.Hour * time.Duration(num)
		}

		expire = time.Now().Add(add).Unix()
	}

	hash, status, err := getSharePasswordHash(body)
	if err != nil {
		return status, err
	}
	stringHash := ""
	var token string
	if len(hash) > 0 {
		tokenBuffer := make([]byte, 24) //nolint:gomnd
		if _, err := rand.Read(tokenBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		token = base64.URLEncoding.EncodeToString(tokenBuffer)
		stringHash = string(hash)
	}
	s = &share.Link{
		Path:         strings.TrimSuffix(r.URL.Path, "/"),
		Hash:         str,
		Expire:       expire,
		UserID:       d.user.ID,
		PasswordHash: stringHash,
		Token:        token,
	}

	if err := store.Share.Save(s); err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, s)
}

func getSharePasswordHash(body share.CreateBody) (data []byte, statuscode int, err error) {

	if body.Password == "" {
		return nil, 0, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to hash password: %w", err)
	}

	return hash, 0, nil
}
