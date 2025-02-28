package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

var (
	NonModifiableFieldsForNonAdmin = []string{"Username", "Scope", "LockPassword", "Perm"}
)

type UserRequest struct {
	What  string      `json:"what"`
	Which []string    `json:"which"`
	Data  *users.User `json:"data"`
}

// userGetHandler retrieves a user by ID.
// @Summary Retrieve a user by ID
// @Description Returns a user's details based on their ID.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path int true "User ID" or "self"
// @Success 200 {object} users.User "User details"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not Found"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users/{id} [get]
func userGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	givenUserIdString := r.URL.Query().Get("id")

	// since api self is used to validate a logged in user
	w.Header().Add("X-Renew-Token", "false")

	var givenUserId uint
	if givenUserIdString == "self" {
		givenUserId = d.user.ID
	} else if givenUserIdString == "" {

		userList, err := store.Users.Gets()
		if err != nil {
			return http.StatusInternalServerError, err
		}

		selfUserList := []*users.User{}
		for _, u := range userList {
			stripInfo(u)
			u.Password = ""
			u.ApiKeys = nil
			u.Scopes = nil
			if u.ID == d.user.ID {
				selfUserList = append(selfUserList, u)
			}
		}

		sort.Slice(userList, func(i, j int) bool {
			return userList[i].ID < userList[j].ID
		})

		if !d.user.Perm.Admin {
			userList = selfUserList
		}
		return renderJSON(w, r, userList)
	} else {
		num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
		givenUserId = uint(num)
	}

	if givenUserId != d.user.ID && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}

	// Fetch the user details
	u, err := store.Users.Get(givenUserId)
	if err == errors.ErrNotExist {
		return http.StatusNotFound, err
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	for key, source := range config.Server.SourceMap {
		if _, ok := d.user.Scopes[key]; ok {
			u.Sources = append(u.Sources, source.Name)
		}
	}

	stripInfo(u)
	return renderJSON(w, r, u)
}

func stripInfo(u *users.User) {
	u.Password = ""
	u.ApiKeys = nil
	u.Scopes = nil

}

// userDeleteHandler deletes a user by ID.
// @Summary Delete a user by ID
// @Description Deletes a user identified by their ID.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 "User deleted successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users/{id} [delete]
func userDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	givenUserIdString := r.URL.Query().Get("id")
	num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
	givenUserId := uint(num)

	if givenUserId == d.user.ID {
		return http.StatusForbidden, fmt.Errorf("cannot delete your own user")
	}

	if !d.user.Perm.Admin {
		return http.StatusForbidden, fmt.Errorf("cannot delete users without admin permissions")
	}

	if givenUserId == 1 {
		return http.StatusForbidden, fmt.Errorf("cannot delete the default admin user")
	}

	// Delete the user
	err := store.Users.Delete(givenUserId)
	if err != nil {
		return errToStatus(err), err
	}
	return http.StatusOK, nil
}

// usersPostHandler creates a new user.
// @Summary Create a new user
// @Description Adds a new user to the system.
// @Tags Users
// @Accept json
// @Produce json
// @Param data body users.User true "User data to create a new user"
// @Success 201 {object} users.User "Created user"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [post]
func usersPostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}
	// Read the JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// Parse the JSON into the UserRequest struct
	var req UserRequest
	if err = json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, err
	}
	r.Body.Close()
	for key, source := range config.Server.SourceMap {
		_, ok := req.Data.Scopes[key]
		if !ok {
			return http.StatusBadRequest, fmt.Errorf("invalid scope for source %s", source.Name)
		}
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
}

// userPutHandler updates an existing user's details.
// @Summary Update a user's details
// @Description Updates the details of a user identified by ID.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param data body users.User true "User data to update"
// @Success 200 {object} users.User "Updated user details"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users/{id} [put]
func userPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	givenUserIdString := r.URL.Query().Get("id")
	num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
	givenUserId := uint(num)

	if givenUserId != d.user.ID && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}

	// Read the JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer r.Body.Close()

	// Parse the JSON into the UserRequest struct
	var req UserRequest
	if err = json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, err
	}

	// If `Which` is not specified, default to updating all fields
	if len(req.Which) == 0 || req.Which[0] == "all" {
		req.Which = []string{}
		v := reflect.ValueOf(req.Data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		t := v.Type()

		// Dynamically populate fields to update
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Name == "Password" && req.Data.Password != "" {
				req.Which = append(req.Which, field.Name)
			} else if field.Name != "Password" && field.Name != "Fs" {
				req.Which = append(req.Which, field.Name)
			}
		}
	}

	// Process the fields to update
	for _, field := range req.Which {

		// Title case field names
		field = cases.Title(language.English, cases.NoLower).String(field)

		// Handle password update
		if field == "Password" {
			if !d.user.Perm.Admin && d.user.LockPassword {
				return http.StatusForbidden, nil
			}
			req.Data.Password, err = users.HashPwd(req.Data.Password)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		}

		// Prevent non-admins from modifying certain fields
		for _, restrictedField := range NonModifiableFieldsForNonAdmin {
			if !d.user.Perm.Admin && field == restrictedField {
				return http.StatusForbidden, nil
			}
		}
	}
	// Perform the user update
	err = store.Users.Update(req.Data, req.Which...)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Return the updated user (with the password hidden) as JSON response
	req.Data.Password = ""
	return renderJSON(w, r, req.Data)
}
