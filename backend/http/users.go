package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

type UserRequest struct {
	Which []string   `json:"which"`
	User  users.User `json:"data"`
}

// userGetHandler retrieves a user by ID.
// @Summary Retrieve a user by ID
// @Description Returns a user's details based on their ID, or all users if no id is provided.
// @Tags Users
// @Accept json
// @Produce json
// @Param id query string false "User ID or 'self'"
// @Success 200 {object} users.User "User details or list of users"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not Found"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [get]
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
			prepForFrontend(u)
			if u.ID == d.user.ID {
				selfUserList = append(selfUserList, u)
			}
		}

		sort.Slice(userList, func(i, j int) bool {
			return userList[i].ID < userList[j].ID
		})

		if !d.user.Permissions.Admin {
			userList = selfUserList
		}
		return renderJSON(w, r, userList)
	} else {
		num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
		givenUserId = uint(num)
	}

	if givenUserId != d.user.ID && !d.user.Permissions.Admin {
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
	prepForFrontend(u)
	return renderJSON(w, r, u)
}

func prepForFrontend(u *users.User) {
	u.Password = ""
	u.ApiKeys = nil
	u.OtpEnabled = u.TOTPSecret != ""
	u.TOTPSecret = ""
	u.TOTPNonce = ""
	u.Scopes = settings.ConvertToFrontendScopes(u.Scopes)
}

// userDeleteHandler deletes a user by ID.
// @Summary Delete a user by ID
// @Description Deletes a user identified by their ID.
// @Tags Users
// @Accept json
// @Produce json
// @Param id query string true "User ID"
// @Success 200 "User deleted successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [delete]
func userDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	givenUserIdString := r.URL.Query().Get("id")
	num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
	givenUserId := uint(num)

	if givenUserId == d.user.ID {
		return http.StatusForbidden, fmt.Errorf("cannot delete your own user")
	}

	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("cannot delete users without admin permissions")
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
	if !d.user.Permissions.Admin {
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

	if req.User.Username == "" {
		return http.StatusBadRequest, errors.ErrEmptyUsername
	}

	if len(req.Which) != 0 {
		return http.StatusBadRequest, nil
	}

	if req.User.Password == "" && req.User.LoginMethod == "password" {
		return http.StatusBadRequest, errors.ErrEmptyPassword
	}

	err = storage.CreateUser(req.User)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Location", "/settings/users/"+strconv.FormatUint(uint64(req.User.ID), 10))
	return http.StatusCreated, nil
}

// userPutHandler updates an existing user's details.
// @Summary Update a user's details
// @Description Updates the details of a user identified by ID.
// @Tags Users
// @Accept json
// @Param id query string false "user ID to update"
// @Param id query string false "usename to update"
// @Param data body users.User true "User data to update"
// @Success 204 "No Content - User updated successfully"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [put]
func userPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	givenUserIdString := r.URL.Query().Get("id")
	username := r.URL.Query().Get("username")
	num, _ := strconv.ParseUint(givenUserIdString, 10, 32)
	givenUserId := uint(num)

	if givenUserId != d.user.ID && !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}

	// Read the JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	// Parse the JSON into the UserRequest struct
	var req UserRequest
	if err = json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, err
	}
	if givenUserId != 0 {
		u, err2 := store.Users.Get(givenUserId)
		if err2 != nil {
			return http.StatusBadRequest, fmt.Errorf("no user not found, please provide a valid id or username")
		}
		req.User.ID = u.ID
		req.User.Username = u.Username
	} else {
		u, err2 := store.Users.Get(username)
		if err2 != nil {
			return http.StatusBadRequest, fmt.Errorf("no user not found, please provide a valid id or username")
		}
		req.User.ID = u.ID
		req.User.Username = u.Username
	}
	if !req.User.OtpEnabled {
		req.User.TOTPSecret = ""
		req.User.TOTPNonce = ""
	}

	err = store.Users.Update(&req.User, d.user.Permissions.Admin, req.Which...)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return http.StatusNoContent, nil
}
