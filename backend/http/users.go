package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
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

	err = storage.CreateUser(req.User, req.User.Permissions)
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

	// Get the old user to check if permissions changed
	oldUser, err := store.Users.Get(req.User.ID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to get user: %w", err)
	}

	err = store.Users.Update(&req.User, d.user.Permissions.Admin, req.Which...)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Revoke all API keys if API permission was removed
	if slices.Contains(req.Which, "Permissions") && oldUser.Permissions.Api && !req.User.Permissions.Api {
		for _, tokenInfo := range oldUser.Tokens {
			if err := auth.RevokeApiToken(store.Access, tokenInfo.Token); err != nil {
				logger.Errorf("Failed to revoke API key: %v", err)
			}
			// Also remove from HashedTokens
			if err := store.Access.RemoveApiToken(tokenInfo.Token); err != nil {
				logger.Errorf("Failed to remove api token: %v", err)
			}
		}
	}

	return http.StatusNoContent, nil
}
