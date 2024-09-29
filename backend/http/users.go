package http

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"

	"github.com/gorilla/mux"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/users"
)

var (
	NonModifiableFieldsForNonAdmin = []string{"Username", "Scope", "LockPassword", "Perm", "Commands", "Rules"}
)

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}
type modifyUserRequest struct {
	modifyRequest
	Data *users.User `json:"data"`
}

func getUserID(r *http.Request) (uint, error) {
	vars := mux.Vars(r)
	i, err := strconv.ParseUint(vars["id"], 10, 0)
	if err != nil {
		return 0, err
	}
	return uint(i), err
}

func getUser(_ http.ResponseWriter, r *http.Request) (*modifyUserRequest, error) {
	if r.Body == nil {
		return nil, errors.ErrEmptyRequest
	}

	req := &modifyUserRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		return nil, err
	}

	if req.What != "user" {
		return nil, errors.ErrInvalidDataType
	}

	return req, nil
}

func withSelfOrAdmin(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, err := getUserID(r)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if d.user.ID != id && !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		d.raw = id
		return fn(w, r, d)
	})
}

var usersGetHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	users, err := d.store.Users.Gets(d.server.Root)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	for _, u := range users {
		u.Password = ""
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return renderJSON(w, r, users)
})

var userGetHandler = withSelfOrAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	u, err := d.store.Users.Get(d.server.Root, d.raw.(uint))
	if err == errors.ErrNotExist {
		return http.StatusNotFound, err
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	u.Password = ""
	if !d.user.Perm.Admin {
		u.Scope = ""
	}
	return renderJSON(w, r, u)
})

var userDeleteHandler = withSelfOrAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	err := d.store.Users.Delete(d.raw.(uint))
	if err != nil {
		return errToStatus(err), err
	}

	return http.StatusOK, nil
})

var userPostHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	req, err := getUser(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if len(req.Which) != 0 {
		return http.StatusBadRequest, nil
	}

	if req.Data.Password == "" {
		return http.StatusBadRequest, errors.ErrEmptyPassword
	}

	err = storage.CreateUser(*req.Data, req.Data.Perm.Admin)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Location", "/settings/users/"+strconv.FormatUint(uint64(req.Data.ID), 10))
	return http.StatusCreated, nil
})

var userPutHandler = withSelfOrAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	req, err := getUser(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if req.Data.ID != d.raw.(uint) {
		return http.StatusBadRequest, nil
	}
	_, _, err = files.GetRealPath(d.server.Root, req.Data.Scope)
	if err != nil {
		return http.StatusBadRequest, nil
	}

	if len(req.Which) == 0 || req.Which[0] == "all" {
		req.Which = []string{}
		v := reflect.ValueOf(req.Data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Name == "Password" && req.Data.Password != "" {
				req.Which = append(req.Which, field.Name)
			} else if field.Name != "Password" && field.Name != "Fs" {
				req.Which = append(req.Which, field.Name)
			}
		}
	}

	for k, v := range req.Which {
		v = cases.Title(language.English, cases.NoLower).String(v)
		req.Which[k] = v
		if v == "Password" {
			if !d.user.Perm.Admin && d.user.LockPassword {
				return http.StatusForbidden, nil
			}
			req.Data.Password, err = users.HashPwd(req.Data.Password)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		}

		for _, f := range NonModifiableFieldsForNonAdmin {
			if !d.user.Perm.Admin && v == f {
				return http.StatusForbidden, nil
			}
		}
	}
	err = d.store.Users.Update(req.Data, req.Which...)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
})
