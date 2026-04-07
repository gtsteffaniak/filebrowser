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
// @Success 200 {object} users.User "User details or list of users"
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
		prepForFrontend(&u)
		return renderJSON(w, r, u)
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
		prepForFrontend(&userValue)
		return renderJSON(w, r, userValue)
	}

	userList, err := state.GetAllUsers()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	selfUserList := []users.User{}
	for i := range userList {
		u := &userList[i]
		prepForFrontend(u)
		if u.Username == d.user.Username {
			selfUserList = append(selfUserList, userList[i])
		}
	}

	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Username < userList[j].Username
	})

	if !d.user.Permissions.Admin {
		userList = selfUserList
	}
	return renderJSON(w, r, userList)
}

func prepForFrontend(u *users.User) {
	u.ID = 0
	u.Password = ""
	u.ApiKeys = nil
	u.Tokens = nil
	u.OtpEnabled = u.TOTPSecret != ""
	u.TOTPSecret = ""
	u.TOTPNonce = ""
	u.Scopes = u.GetFrontendScopes()
	u.SidebarLinks = u.GetFrontendSidebarLinks()
	u.Locale = normalizeLocale(u.Locale)
}

// normalizeLocale converts various locale formats (xx_xx, xx-xx, xxxx) to camelCase format (xxXX)
// Frontend expects camelCase for compound locales (zhCN, ptBR, etc.) as shown in select option values
func normalizeLocale(locale string) string {
	if locale == "" {
		return locale
	}

	// Convert to lowercase for processing
	lower := strings.ToLower(locale)

	// Special case mappings (standard locale codes to frontend camelCase format)
	specialCases := map[string]string{
		"cs":    "cz", // Czech
		"uk":    "ua", // Ukrainian
		"zh-cn": "zhCN",
		"zh_cn": "zhCN",
		"zhcn":  "zhCN",
		"zh-tw": "zhTW",
		"zh_tw": "zhTW",
		"zhtw":  "zhTW",
		"pt-br": "ptBR",
		"pt_br": "ptBR",
		"ptbr":  "ptBR",
		"sv-se": "svSE",
		"sv_se": "svSE",
		"svse":  "svSE",
		"nl-be": "nlBE",
		"nl_be": "nlBE",
		"nlbe":  "nlBE",
	}

	// Check special cases first
	if normalized, ok := specialCases[lower]; ok {
		return normalized
	}

	// If already in camelCase format (4+ chars, has uppercase), return as-is
	if len(locale) >= 4 {
		// Check if it's a known camelCase locale
		knownCamelCase := map[string]bool{
			"zhCN": true, "zhTW": true, "ptBR": true, "svSE": true, "nlBE": true,
		}
		if knownCamelCase[locale] {
			return locale
		}
	}

	// Handle xx_xx or xx-xx format: convert to xxXX (camelCase)
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(parts) == 2 {
		// Convert to camelCase: first part lowercase, second part capitalized
		first := parts[0]
		second := parts[1]
		if len(second) > 0 {
			second = strings.ToUpper(second[:1]) + second[1:]
		}
		normalized := first + second

		// Check if this matches a known compound locale
		knownCompound := map[string]string{
			"zhcn": "zhCN", "zhtw": "zhTW", "ptbr": "ptBR",
			"svse": "svSE", "nlbe": "nlBE",
		}
		if normalizedVal, ok := knownCompound[normalized]; ok {
			return normalizedVal
		}
		return normalized
	}

	// Single part locale (en, fr, de, etc.) - return as-is (lowercase is fine)
	return lower
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

// userPutHandler updates an existing user's details, keyed by username (?username= or body data.username).
// Noauth: when the client omits the target login name, the configured admin user is updated.
// @Summary Update a user's details
// @Description Updates the details of a user identified by username.
// @Tags Users
// @Accept json
// @Param username query string false "Username of user to update"
// @Param data body users.User true "User data to update"
// @Success 204 "No Content - User updated successfully"
// @Failure 400 {object} map[string]string "Bad Request"
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

	// Extract plaintext password if provided, otherwise pass empty string
	plaintextPassword := ""
	if slices.Contains(req.Which, "Password") && req.User.Password != "" {
		plaintextPassword = req.User.Password
	}

	// Use patch update with specified fields
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
