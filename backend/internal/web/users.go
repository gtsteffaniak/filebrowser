package web

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strings"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"

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
func userGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	// since api self is used to validate a logged in user
	w.Header().Add("X-Renew-Token", "false")

	if strings.TrimSpace(r.URL.Query().Get("id")) != "" {
		return http.StatusBadRequest, fmt.Errorf("query parameter id is not supported; use username=self for the current user or username=<login>")
	}

	usernameParam := strings.TrimSpace(r.URL.Query().Get("username"))
	if usernameParam == "self" {
		u, err := state.GetUserByUsername(d.User.Username)
		if err == errors.ErrNotExist {
			return http.StatusNotFound, err
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		u = PrepForFrontend(u)
		return RenderJSON(w, r, u.FrontendUser)
	}

	if usernameParam != "" {
		userValue, err := state.GetUserByUsername(usernameParam)
		if err == errors.ErrNotExist {
			return http.StatusNotFound, err
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !d.User.Permissions.Admin && userValue.Username != d.User.Username {
			return http.StatusForbidden, nil
		}
		userValue = PrepForFrontend(userValue)
		return RenderJSON(w, r, userValue.FrontendUser)
	}

	userList, err := state.GetAllUsers()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	userListFE := make([]users.FrontendUser, 0, len(userList))
	for i := range userList {
		u := userList[i]
		u = PrepForFrontend(u)
		userListFE = append(userListFE, u.FrontendUser)
	}

	sort.Slice(userListFE, func(i, j int) bool {
		return userListFE[i].Username < userListFE[j].Username
	})

	if !d.User.Permissions.Admin {
		var selfOnly []users.FrontendUser
		for _, fe := range userListFE {
			if fe.Username == d.User.Username {
				selfOnly = append(selfOnly, fe)
			}
		}
		userListFE = selfOnly
	}
	return RenderJSON(w, r, userListFE)
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
func userDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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

	if givenUserId == d.User.ID {
		return http.StatusForbidden, fmt.Errorf("cannot delete your own user")
	}

	if !d.User.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("cannot delete users without admin permissions")
	}

	status, err := verifyActorPasswordForUserActions(r, d)
	if err != nil {
		return status, err
	}

	// Delete the user
	err = state.DeleteUser(givenUserId)
	if err != nil {
		return ErrToStatus(err), err
	}
	activity.RecordUserMutation(r, toActor(d), activitydb.EventUserDelete, &uVal, nil)
	return http.StatusOK, nil
}

// usersPostHandler creates a new user.
// @Summary Create a new user
// @Description Adds a new user to the system. When the authenticated actor uses password login, they must send their current password in the X-Password header.
// @Tags Users
// @Accept json
// @Produce json
// @Param X-Password header string false "Actor's current password (URL-encoded); required for password-login actors"
// @Param data body users.User true "User data to create a new user"
// @Success 201 {object} users.User "Created user"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized - invalid or missing actor password when required"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [post]
func usersPostHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !d.User.Permissions.Admin {
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
	status, err := verifyActorPasswordForUserActions(r, d)
	if err != nil {
		return status, err
	}
	plaintextPassword := req.User.Password
	err = state.CreateUser(&req.User, plaintextPassword)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if dirErr := files.MakeUserDirs(&req.User, true); dirErr != nil {
		logger.Error(dirErr.Error())
	}

	activity.RecordUserMutation(r, toActor(d), activitydb.EventUserCreate, &req.User, nil)
	w.Header().Set("Location", "/settings/users/"+url.PathEscape(req.User.Username))
	return http.StatusCreated, nil
}

// nonAdminEditableFieldNameSet returns User.NonAdminEditable struct field names (e.g. "Locale", "Preview").
func nonAdminEditableFieldNameSet() map[string]struct{} {
	m := make(map[string]struct{})
	t := reflect.TypeOf(users.NonAdminEditable{})
	for i := 0; i < t.NumField(); i++ {
		m[t.Field(i).Name] = struct{}{}
	}
	return m
}

// userPutOnlyNonAdminEditableFields reports whether req.Which lists exclusively NonAdminEditable fields,
// excluding Password. Empty which or which[0] == "all" means a broad update and returns false.
func userPutOnlyNonAdminEditableFields(which []string) bool {
	if len(which) == 0 || strings.EqualFold(strings.TrimSpace(which[0]), "all") {
		return false
	}
	allowed := nonAdminEditableFieldNameSet()
	for _, w := range which {
		f := utils.CapitalizeFirst(strings.TrimSpace(w))
		if f == "" {
			return false
		}
		if strings.EqualFold(f, "Password") {
			return false
		}
		if _, ok := allowed[f]; !ok {
			return false
		}
	}
	return true
}

// verifyActorPasswordForUserPut requires URL-encoded X-Password when the authenticated actor uses
// password login. Callers should invoke this only when the update requires re-authentication.
func verifyActorPasswordForUserActions(r *http.Request, d *Context) (int, error) {
	if d.User.LoginMethod != users.LoginMethodPassword {
		return 0, nil
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
	actor, err := state.GetUserByID(d.User.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := utils.CheckPwd(plain, actor.Password); err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid password")
	}
	return 0, nil
}

// userPatchHandler updates an existing user's details.
// @Summary Update a user's details
// @Description Updates the details of a user identified by ID. When the authenticated actor uses password login, they must send their current password in the X-Password header unless the update only touches NonAdminEditable profile fields (not password). Full updates (which empty or "all") or any admin-only field require confirmation.
// @Tags Users
// @Accept json
// @Param id query string false "user ID to update"
// @Param id query string false "usename to update"
// @Param X-Password header string false "Actor's current password (URL-encoded); required for password-login actors when updating password, using which=all, or any field outside NonAdminEditable"
// @Param data body users.User true "User data to update"
// @Success 204 "No Content - User updated successfully"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized - invalid or missing actor password when required"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users [patch]
func userPatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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
	if targetUsername == "" && settings.Config.Auth.Methods.NoAuth {
		admin := settings.Config.Auth.AdminUsername
		if admin == "" {
			admin = "admin"
		}
		targetUsername = admin
	}
	if targetUsername == "" && d.User != nil && d.User.Username != "" && d.User.Username != "anonymous" {
		targetUsername = d.User.Username
	}
	if targetUsername == "" {
		return http.StatusBadRequest, fmt.Errorf("username is required (?username= or in request data)")
	}

	if !d.User.Permissions.Admin && targetUsername != d.User.Username {
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
	userValue, err := state.GetUserByID(req.User.ID)
	if err == nil {
		oldUser = &userValue
	}
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to get user: %w", err)
	}

	if d.User.LoginMethod == users.LoginMethodPassword && !userPutOnlyNonAdminEditableFields(req.Which) {
		var status int
		status, err = verifyActorPasswordForUserActions(r, d)
		if err != nil {
			return status, err
		}
	}

	if targetUsername == d.User.Username {
		if enfErr := settings.ValidateSelfUserUpdateNotEnforced(req.Which, state.GetEnforcedUserDefaults(), d.User); enfErr != nil {
			var locked settings.ErrEnforcedUserField
			if stderrors.As(enfErr, &locked) {
				return http.StatusForbidden, enfErr
			}
			return http.StatusBadRequest, enfErr
		}
	}

	err = state.UpdateUser(&req.User, req.User.Password, req.Which...)
	if err != nil {
		var locked settings.ErrEnforcedUserField
		var mismatch settings.ErrEnforcedUserValueMismatch
		if stderrors.As(err, &locked) || stderrors.As(err, &mismatch) {
			return http.StatusForbidden, err
		}
		return http.StatusBadRequest, err
	}

	// Revoke all API keys if API permission was removed
	if slices.Contains(req.Which, "Permissions") && oldUser.Permissions.Api && !req.User.Permissions.Api {
		users.EachNamedToken(oldUser.Tokens, func(_ string, tokenInfo users.AuthToken) {
			if err := state.RevokeToken(tokenInfo.Token); err != nil {
				logger.Errorf("Failed to revoke API key: %v", err)
			}
			if err := state.RemoveApiToken(tokenInfo.Token); err != nil {
				logger.Errorf("Failed to remove api token: %v", err)
			}
		})
	}

	updatedUser, getErr := state.GetUserByID(req.User.ID)
	if getErr != nil {
		return http.StatusInternalServerError, getErr
	}
	changes := activity.UserUpdateChanges(oldUser, &updatedUser, req.Which, req.User.Password != "")
	if len(changes) > 0 {
		activity.RecordUserMutation(r, toActor(d), activitydb.EventUserUpdate, &updatedUser, changes)
	}
	return http.StatusNoContent, nil
}

// PrepForFrontend fills response-only fields for GET handlers. FrontendScopes are derived from
// persisted BackendScopes (GetFrontendScopes); they are not read from SQL and must not be written back as-is.
func PrepForFrontend(u users.User) users.User {
	u.FrontendScopes = u.GetFrontendScopes()
	u.Permissions = users.GlobalPermissionsOnly(u.Permissions)
	u.SourcePermissions = nil
	u.SidebarLinks = usersidebar.FrontendLinks(u.SidebarLinks, u.ShowToolsInSidebar)
	u.Password = ""
	u.ApiKeys = nil
	u.Tokens = nil
	u.OtpEnabled = u.TOTPSecret != ""
	u.TOTPSecret = ""
	u.TOTPNonce = ""
	u.PinnedItems = nil
	u.Locale = NormalizeLocaleForFrontend(u.Locale)
	for i := range u.PasskeyCredentials {
		u.PasskeyCredentials[i].PublicKey = ""
		u.PasskeyCredentials[i].AttestationType = ""
		u.PasskeyCredentials[i].AttestationFormat = ""
		u.PasskeyCredentials[i].Flags = users.WebAuthnCredentialFlags{}
		u.PasskeyCredentials[i].SignCount = 0
		u.PasskeyCredentials[i].ClientDataJSON = ""
		u.PasskeyCredentials[i].ClientDataHash = ""
		u.PasskeyCredentials[i].AuthenticatorData = ""
		u.PasskeyCredentials[i].PublicKeyAlg = 0
		u.PasskeyCredentials[i].AttestationObj = ""
	}
	return u
}

// normalizeLocaleForFrontend converts various locale formats to camelCase (e.g. zhCN, ptBR).
func NormalizeLocaleForFrontend(locale string) string {
	if locale == "" {
		return locale
	}

	lower := strings.ToLower(locale)

	specialCases := map[string]string{
		"cs":    "cz",
		"uk":    "ua",
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

	if normalized, ok := specialCases[lower]; ok {
		return normalized
	}

	if len(locale) >= 4 {
		knownCamelCase := map[string]bool{
			"zhCN": true, "zhTW": true, "ptBR": true, "svSE": true, "nlBE": true,
		}
		if knownCamelCase[locale] {
			return locale
		}
	}

	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(parts) == 2 {
		first := parts[0]
		second := parts[1]
		if len(second) > 0 {
			second = strings.ToUpper(second[:1]) + second[1:]
		}
		normalized := first + second

		knownCompound := map[string]string{
			"zhcn": "zhCN", "zhtw": "zhTW", "ptbr": "ptBR",
			"svse": "svSE", "nlbe": "nlBE",
		}
		if normalizedVal, ok := knownCompound[normalized]; ok {
			return normalizedVal
		}
		return normalized
	}

	return lower
}
