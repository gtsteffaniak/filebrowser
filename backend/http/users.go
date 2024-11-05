package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"

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
	id := r.PathValue("id")
	i, err := strconv.ParseUint(id, 10, 0)
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

// usersGetHandler retrieves a list of all users.
// @Summary Retrieve a list of users
// @Description Returns a list of all registered users. Requires admin permissions.
// @Tags Users
// @Accept json
// @Produce json
// @Success 200 {array} users.User "List of users"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [get]
func usersGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	users, err := store.Users.Gets(config.Server.Root)
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
}

// userGetHandler retrieves a user by ID.
// @Summary Retrieve a user by ID
// @Description Returns a user's details based on their ID.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} users.User "User details"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not Found"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users/{id} [get]
func userGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Ensure the requesting user is either the admin or the user themselves
	if d.user.ID != d.raw.(uint) && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}

	// Fetch the user details
	u, err := store.Users.Get(config.Server.Root, d.raw.(uint))
	if err == errors.ErrNotExist {
		fmt.Println("user error!")
		return http.StatusNotFound, err
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Remove the password from the response if the user is not an admin
	u.Password = ""
	if !d.user.Perm.Admin {
		u.Scope = ""
	}

	return renderJSON(w, r, u)
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
	// Check if the requesting user is either the admin or the user themselves
	if d.user.ID != d.raw.(uint) && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}

	// Delete the user
	err := store.Users.Delete(d.raw.(uint))
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
	// Extract user data from the request
	req, err := getUser(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Ensure the requested user matches the current user (self)
	if req.Data.ID != d.raw.(uint) {
		return http.StatusForbidden, nil
	}

	// Validate the user's scope
	_, _, err = files.GetRealPath(config.Server.Root, req.Data.Scope)
	if err != nil {
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
