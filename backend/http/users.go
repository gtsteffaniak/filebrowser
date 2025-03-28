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
			prepForFrontend(u)
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
	prepForFrontend(u)
	return renderJSON(w, r, u)
}

func prepForFrontend(u *users.User) {
	u.Password = ""
	u.ApiKeys = nil
	u.Scopes = settings.ConvertToFrontendScopes(u.Scopes)
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

	// Perform the user update
	err = store.Users.Update(req.Data, d.user.Perm.Admin, req.Which...)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Return the updated user (with the password hidden) as JSON response
	req.Data.Password = ""
	return renderJSON(w, r, req.Data)
}
