package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/state"
	"github.com/gtsteffaniak/go-logger/logger"
)

type UserRequest struct {
	Which []string   `json:"which"`
	User  users.User `json:"data"`
}

// userGetHandler lists users or returns one user by username. Numeric user IDs are not accepted.
// @Summary List users or get one by username
// @Description Returns all users (admins) or only the current user; with ?username=self, the logged-in user; with ?username=login, that user if permitted. Query id= is not supported.
// @Tags Users
// @Accept json
// @Produce json
// @Param username query string false "Login name, or 'self' for the current session user"
// @Success 200 {object} users.FrontendUser "User details or list of users"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not Found"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [get]
func userGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// since api self is used to validate a logged in user
	w.Header().Add("X-Renew-Token", "false")

	if strings.TrimSpace(r.URL.Query().Get("id")) != "" {
		return http.StatusBadRequest, fmt.Errorf("query parameter id is not supported; use username=self for the current user or username=<login>")
	}

	usernameParam := strings.TrimSpace(r.URL.Query().Get("username"))
	if usernameParam == "self" {
		u, err := state.GetUserByUsername(d.user.Username)
		if err == errors.ErrNotExist {
			return http.StatusNotFound, err
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		u.PrepForFrontend()
		return renderJSON(w, r, u.FrontendUser)
	}

	if usernameParam != "" {
		userValue, err := state.GetUserByUsername(usernameParam)
		if err == errors.ErrNotExist {
			return http.StatusNotFound, err
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !d.user.Permissions.Admin && userValue.Username != d.user.Username {
			return http.StatusForbidden, nil
		}
		userValue.PrepForFrontend()
		return renderJSON(w, r, userValue.FrontendUser)
	}

	userList, err := state.GetAllUsers()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	userListFE := make([]users.FrontendUser, 0, len(userList))
	for i := range userList {
		u := userList[i]
		u.PrepForFrontend()
		userListFE = append(userListFE, u.FrontendUser)
	}

	sort.Slice(userListFE, func(i, j int) bool {
		return userListFE[i].Username < userListFE[j].Username
	})

	if !d.user.Permissions.Admin {
		var selfOnly []users.FrontendUser
		for _, fe := range userListFE {
			if fe.Username == d.user.Username {
				selfOnly = append(selfOnly, fe)
			}
		}
		userListFE = selfOnly
	}
	return renderJSON(w, r, userListFE)
}

// userDeleteHandler deletes a user by username (query ?username=).
// @Summary Delete a user by username
// @Description Deletes a user identified by login name.
// @Tags Users
// @Accept json
// @Produce json
// @Param username query string true "Username"
// @Success 200 "User deleted successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [delete]
func userDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		return http.StatusBadRequest, fmt.Errorf("username query parameter is required")
	}

	uVal, err := state.GetUserByUsername(username)
	if err == errors.ErrNotExist {
		return http.StatusNotFound, err
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	givenUserId := uVal.ID

	if givenUserId == d.user.ID {
		return http.StatusForbidden, fmt.Errorf("cannot delete your own user")
	}

	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("cannot delete users without admin permissions")
	}

	err = state.DeleteUser(givenUserId)
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

	// Extract plaintext password before creating user
	plaintextPassword := req.User.Password
	err = state.CreateUser(&req.User, plaintextPassword)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Location", "/settings/users/"+url.PathEscape(req.User.Username))
	return http.StatusCreated, nil
}

// passwordUpdateRequested reports whether the client intends to change the target user's password
// (non-empty new password and "password" in the which list). This matches storage parseFields behavior.
func passwordUpdateRequested(req *UserRequest) bool {
	if req.User.Password == "" {
		return false
	}
	for _, w := range req.Which {
		if strings.EqualFold(w, "password") {
			return true
		}
	}
	return false
}

// verifyActorPasswordForUserPasswordChange ensures the authenticated actor re-proves their password
// when changing another user's or their own password via PUT. Requires URL-encoded X-Password (same as login).
func verifyActorPasswordForUserPasswordChange(r *http.Request, d *requestContext, target *users.User) (int, error) {
	if d.user.LoginMethod != users.LoginMethodPassword || target.LoginMethod != users.LoginMethodPassword {
		return http.StatusForbidden, fmt.Errorf("password can only be changed when both accounts use password login")
	}
	encoded := r.Header.Get("X-Password")
	if encoded == "" {
		return http.StatusUnauthorized, fmt.Errorf("X-Password header is required to confirm your password")
	}
	plain, err := url.QueryUnescape(encoded)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid password encoding")
	}
	if plain == "" {
		return http.StatusUnauthorized, fmt.Errorf("X-Password header is required to confirm your password")
	}
	actor, err := state.GetUser(d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := utils.CheckPwd(plain, actor.Password); err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid password")
	}
	return 0, nil
}

// userPutHandler updates an existing user's details.
// @Summary Update a user's details
// @Description Updates the details of a user identified by ID. When updating the target user's password the actor must send their current password in the X-Password header
// @Tags Users
// @Accept json
// @Param id query string false "user ID to update"
// @Param id query string false "usename to update"
// @Param X-Password header string false "Actor's current password (URL-encoded); required when changing a password user's password"
// @Param data body users.User true "User data to update"
// @Success 204 "No Content - User updated successfully"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized - invalid or missing actor password for password change"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [put]
func userPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusBadRequest, err
	}
	defer r.Body.Close()

	var req UserRequest
	if err = json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, err
	}

	targetUsername := strings.TrimSpace(r.URL.Query().Get("username"))
	if targetUsername == "" {
		targetUsername = strings.TrimSpace(req.User.Username)
	}
	if targetUsername == "" && config.Auth.Methods.NoAuth {
		admin := config.Auth.AdminUsername
		if admin == "" {
			admin = "admin"
		}
		targetUsername = admin
	}
	if targetUsername == "" && d.user != nil && d.user.Username != "" && d.user.Username != "anonymous" {
		targetUsername = d.user.Username
	}
	if targetUsername == "" {
		return http.StatusBadRequest, fmt.Errorf("username is required (?username= or in request data)")
	}

	if !d.user.Permissions.Admin && targetUsername != d.user.Username {
		return http.StatusForbidden, nil
	}

	uValue, err2 := state.GetUserByUsername(targetUsername)
	if err2 == errors.ErrNotExist {
		return http.StatusBadRequest, fmt.Errorf("user not found: %s", targetUsername)
	}
	if err2 != nil {
		return http.StatusInternalServerError, err2
	}
	req.User.ID = uValue.ID
	req.User.Username = uValue.Username
	if !req.User.OtpEnabled {
		req.User.TOTPSecret = ""
		req.User.TOTPNonce = ""
	}

	// Get the old user to check if permissions changed
	var oldUser *users.User
	userValue, err := state.GetUser(req.User.ID)
	if err == nil {
		oldUser = &userValue
	}
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to get user: %w", err)
	}

	plaintextPassword := ""
	if passwordUpdateRequested(&req) {
		plaintextPassword = req.User.Password
		var status int
		status, err = verifyActorPasswordForUserPasswordChange(r, d, oldUser)
		if err != nil {
			return status, err
		}
	}

	err = state.UpdateUser(&req.User, plaintextPassword, req.Which...)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Revoke all API keys if API permission was removed
	if slices.Contains(req.Which, "Permissions") && oldUser.Permissions.Api && !req.User.Permissions.Api {
		for _, tokenInfo := range oldUser.Tokens {
			if err := auth.RevokeApiToken(accessStore, tokenInfo.Token); err != nil {
				logger.Errorf("Failed to revoke API key: %v", err)
			}
			// Also remove from HashedTokens
			if err := accessStore.RemoveApiToken(tokenInfo.Token); err != nil {
				logger.Errorf("Failed to remove api token: %v", err)
			}
		}
	}

	return http.StatusNoContent, nil
}
