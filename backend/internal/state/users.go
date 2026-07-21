package state

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// User operations

// copyBackendSourcePermissions returns a shallow copy of the per-source permissions map.
func copyBackendSourcePermissions(m map[string]users.SourceFilePermissions) map[string]users.SourceFilePermissions {
	if m == nil {
		return nil
	}
	out := make(map[string]users.SourceFilePermissions, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// copyUserSlices deep-copies user slice/map fields into userCopy.
func copyUserSlices(userCopy *users.User, user *users.User) {
	if user.BackendScopes != nil {
		userCopy.BackendScopes = make([]users.BackendScope, len(user.BackendScopes))
		copy(userCopy.BackendScopes, user.BackendScopes)
	}
	if user.BackendSourcePermissions != nil {
		userCopy.BackendSourcePermissions = copyBackendSourcePermissions(user.BackendSourcePermissions)
	}
	if user.Tokens != nil {
		userCopy.Tokens = make(map[string]users.AuthToken, len(user.Tokens))
		for k, v := range user.Tokens {
			userCopy.Tokens[k] = v
		}
	}
	if user.SidebarLinks != nil {
		userCopy.SidebarLinks = make([]users.SidebarLink, len(user.SidebarLinks))
		copy(userCopy.SidebarLinks, user.SidebarLinks)
	}
	if user.FrontendScopes != nil {
		userCopy.FrontendScopes = make([]users.FrontendScope, len(user.FrontendScopes))
		copy(userCopy.FrontendScopes, user.FrontendScopes)
	}
	if user.SourcePermissions != nil {
		userCopy.SourcePermissions = copyBackendSourcePermissions(user.SourcePermissions)
	}
	if user.ApiKeys != nil {
		userCopy.ApiKeys = make(map[string]users.AuthToken, len(user.ApiKeys))
		for k, v := range user.ApiKeys {
			userCopy.ApiKeys[k] = v
		}
	}
	if user.PasskeyCredentials != nil {
		userCopy.PasskeyCredentials = copyWebAuthnCredentials(user.PasskeyCredentials)
	}
	if user.PinnedItems != nil {
		userCopy.PinnedItems = copyPinnedItems(user.PinnedItems)
	}
}

func copyWebAuthnCredentials(in []users.WebAuthnCredential) []users.WebAuthnCredential {
	out := make([]users.WebAuthnCredential, len(in))
	for i, c := range in {
		out[i] = c
		if c.Transport != nil {
			out[i].Transport = append([]string(nil), c.Transport...)
		}
	}
	return out
}

func copyPinnedItems(in users.PinnedItems) users.PinnedItems {
	if in == nil {
		return nil
	}
	out := make(users.PinnedItems, len(in))
	for srcPath, dirs := range in {
		dirCopy := make(map[string][]string, len(dirs))
		for dirPath, names := range dirs {
			dirCopy[dirPath] = append([]string(nil), names...)
		}
		out[srcPath] = dirCopy
	}
	return out
}

// GetUserByID retrieves a user by stable numeric id (JWT belongsTo, admin APIs).
func GetUserByID(id uint64) (users.User, error) {
	if id == 0 {
		return users.User{}, errors.ErrNotExist
	}
	usersMux.RLock()
	defer usersMux.RUnlock()

	if cached, ok := getCachedUserByID(id); ok {
		return *cached, nil
	}

	user, err := loadUserByIDFromDB(id)
	if err != nil {
		return users.User{}, err
	}
	putUserInCache(user)
	userCopy := *user
	copyUserSlices(&userCopy, user)
	return userCopy, nil
}

// GetUserByUsername returns a user by username (30m cache, SQLite on miss).
func GetUserByUsername(username string) (users.User, error) {
	usersMux.RLock()
	defer usersMux.RUnlock()

	if cached, ok := getCachedUserByUsername(username); ok {
		return *cached, nil
	}

	user, err := loadUserByUsernameFromDB(username)
	if err != nil {
		return users.User{}, err
	}
	putUserInCache(user)
	userCopy := *user
	copyUserSlices(&userCopy, user)
	return userCopy, nil
}

// UserIDForUsername returns the stable id for a login name (for id-keyed user storage).
func UserIDForUsername(username string) (uint64, error) {
	u, err := GetUserByUsername(username)
	if err != nil {
		return 0, err
	}
	return u.ID, nil
}

// GetAllUsers returns all users from SQLite (not a long-lived memory map).
func GetAllUsers() ([]users.User, error) {
	usersMux.RLock()
	defer usersMux.RUnlock()

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return nil, err
	}
	out := make([]users.User, 0, len(usersList))
	for _, user := range usersList {
		if user == nil {
			continue
		}
		userCopy := *user
		copyUserSlices(&userCopy, user)
		out = append(out, userCopy)
	}
	return out, nil
}

// UserFromAPIToken resolves the user for a validated API JWT: numeric belongsTo id, or minimal tokens
// (hash → user id). Usernames are not used so a reused login name never inherits a previous account's API access.
func UserFromAPIToken(tk users.AuthToken, rawToken string) (users.User, error) {
	if tk.BelongsTo != 0 {
		return GetUserByID(tk.BelongsTo)
	}
	if uid, ok := accessDb.GetUserIDFromToken(rawToken); ok {
		return GetUserByID(uid)
	}
	return users.User{}, errors.ErrNotExist
}

// UserForShareOwner resolves the user who owns a share link.
func UserForShareOwner(link share.Share) (users.User, error) {
	if link.UserID == 0 {
		return users.User{}, errors.ErrNotExist
	}
	return GetUserByID(link.UserID)
}

// CreateUser creates a new user with plaintext password.
//
// Scope model: only BackendScopes are persisted (SQL user_data). JSON "scopes" on the API unmarshals into
// FrontendScopes; state converts those to BackendScopes via APIScopesToBackend, clears FrontendScopes, then
// may apply default-enabled sources. In-memory/cache users always keep FrontendScopes nil; GET handlers use
// PrepForFrontend to repopulate FrontendScopes from BackendScopes for responses.
func CreateUser(user *users.User, plaintextPassword string) error {
	if err := users.ValidateUsername(user.Username); err != nil {
		return err
	}
	if _, err := GetUserByUsername(user.Username); err == nil {
		return fmt.Errorf("user with username %s already exists", user.Username)
	} else if err != errors.ErrNotExist {
		return err
	}
	// Hash password if provided
	if plaintextPassword != "" {
		hashedPassword, err := utils.HashPwd(plaintextPassword)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
	}

	// Incoming API "scopes" → BackendScopes; FrontendScopes must not remain on the persisted user.
	if err := applyScopesFromAPI(user); err != nil {
		return err
	}

	// If still no BackendScopes (omitted or invalid API names), same defaults as ApplyUserDefaults.
	ApplyUserDefaults(user)
	defaults := EffectiveUserDefaults()
	enforced := EffectiveEnforced()
	settings.ApplyEnforcedDefaultsFrom(user, defaults, enforced)

	users.SyncBackendSourcePermissionsMap(user)

	usersMux.Lock()
	defer usersMux.Unlock()

	if user.ID == 0 {
		nid, genErr := utils.RandomUint64ID()
		if genErr != nil {
			return fmt.Errorf("allocate user id: %w", genErr)
		}
		user.ID = nid
	}

	if err := sqlDb.CreateUser(user); err != nil {
		return err
	}

	putUserInCache(user)

	return nil
}

// UpdateUser updates an existing user with write-through to SQL
// If plaintextPassword is provided (non-empty), it will be hashed before saving
// If fields are specified, only those fields are updated (patch operation)
// Note: fields should be JSON tag names (e.g., "showFirstLogin") which will be converted to struct field names
func UpdateUser(user *users.User, plaintextPassword string, fields ...string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	storedSnapshot, err := loadExistingUserLocked(user)
	if err != nil {
		return fmt.Errorf("user %s not found in cache", user.Username)
	}
	existingUser := cloneUserPtr(storedSnapshot)
	oldUsername := storedSnapshot.Username
	oldUserID := storedSnapshot.ID

	// If no fields specified, or the API sends which=["all"], replace the entire user (full update).
	// Client UX sends "all" as a broad save; it must not be interpreted as a single JSON field name.
	updateAll := len(fields) == 0
	if !updateAll && len(fields) == 1 && strings.EqualFold(strings.TrimSpace(fields[0]), "all") {
		updateAll = true
	}

	if !updateAll {
		// 2. Patch operation - selectively copy specified fields using reflection
		existingVal := reflect.ValueOf(existingUser).Elem()
		newVal := reflect.ValueOf(user).Elem()

		for _, jsonFieldName := range fields {
			// Handle password specially
			if jsonFieldName == "password" || jsonFieldName == "Password" {
				if plaintextPassword != "" {
					hashedPassword, err := utils.HashPwd(plaintextPassword)
					if err != nil {
						return fmt.Errorf("failed to hash password: %w", err)
					}
					existingUser.Password = hashedPassword
				}
				continue
			}

			// Find struct field by JSON tag name (handles embedded structs)
			structFieldName := findFieldByJSONTag(reflect.TypeOf(user).Elem(), jsonFieldName)

			// If not found by JSON tag, try direct field name match (for backwards compatibility)
			if structFieldName == "" {
				structFieldName = jsonFieldName
			}

			// Use reflection to copy the field (FieldByName works with embedded structs)
			existingField := existingVal.FieldByName(structFieldName)
			newField := newVal.FieldByName(structFieldName)

			if existingField.IsValid() && existingField.CanSet() && newField.IsValid() {
				existingField.Set(newField)
			}
		}
	} else {
		// Full update - replace all fields
		if plaintextPassword != "" {
			hashedPassword, err := utils.HashPwd(plaintextPassword)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			user.Password = hashedPassword
		} else {
			// Preserve existing password
			user.Password = existingUser.Password
		}

		// Profile PUT (which=all) omits server-managed fields; keep persisted data.
		preserveServerManagedFields(existingUser, user)

		// Replace entire user pointer
		existingUser = cloneUserPtr(user)
	}

	// Request JSON "scopes" → BackendScopes only; FrontendScopes are never persisted (see PrepForFrontend).
	if updateAll {
		if err := applyScopesFromAPI(existingUser); err != nil {
			return err
		}
	} else {
		if fieldListPatchesBackendScopes(fields) || fieldListPatchesAPISourcePermissions(fields) {
			if err := applyScopesFromAPI(existingUser); err != nil {
				return err
			}
		} else {
			for _, jsonFieldName := range fields {
				if strings.EqualFold(jsonFieldName, "scopes") || strings.EqualFold(jsonFieldName, "sourcePermissions") {
					if err := applyScopesFromAPI(existingUser); err != nil {
						return err
					}
					break
				}
			}
		}
	}
	users.SyncBackendSourcePermissionsMap(existingUser)
	existingUser.FrontendScopes = nil
	existingUser.SourcePermissions = nil

	defaults := EffectiveUserDefaults()
	enforced := EffectiveEnforced()
	if err := settings.ValidateUserAgainstEnforcedDefaults(existingUser, defaults, enforced); err != nil {
		return err
	}

	// 3. Write to database
	var updateErr error
	if oldUsername != existingUser.Username {
		if updateErr = sqlDb.UpdateUserUsername(oldUsername, existingUser); updateErr != nil {
			return updateErr
		}
		if oldUserID != 0 && oldUserID != existingUser.ID {
			userRecordCache.Delete(userCacheKeyID(oldUserID))
		}
		userRecordCache.Delete(userCacheKeyName(oldUsername))
	} else {
		if updateErr = sqlDb.UpdateUser(existingUser); updateErr != nil {
			return updateErr
		}
	}

	putUserInCache(existingUser)

	return nil
}

// preserveServerManagedFields copies persisted server-side data when a full user PUT
// omits fields the frontend never sends (API tokens, passkeys, pinned items, etc.).
func preserveServerManagedFields(old, new *users.User) {
	if new.Tokens == nil && old.Tokens != nil {
		new.Tokens = old.Tokens
	}
	if new.ApiKeys == nil && old.ApiKeys != nil {
		new.ApiKeys = old.ApiKeys
	}
	if len(new.PasskeyCredentials) == 0 && len(old.PasskeyCredentials) > 0 {
		new.PasskeyCredentials = old.PasskeyCredentials
	}
	if new.PinnedItems == nil && old.PinnedItems != nil {
		new.PinnedItems = old.PinnedItems
	}
	if new.BackendSourcePermissions == nil && old.BackendSourcePermissions != nil {
		new.BackendSourcePermissions = copyBackendSourcePermissions(old.BackendSourcePermissions)
	}
	if new.OtpEnabled && new.TOTPSecret == "" && new.TOTPNonce == "" && old.TOTPSecret != "" {
		new.TOTPSecret = old.TOTPSecret
		new.TOTPNonce = old.TOTPNonce
	}
	if new.Version == 0 && old.Version != 0 {
		new.Version = old.Version
	}
}

// fieldListPatchesBackendScopes reports whether fields include persisted scope paths (JSON tag
// "backendScopes", case-insensitive; matches struct name BackendScopes as well).
func fieldListPatchesBackendScopes(fields []string) bool {
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if strings.EqualFold(f, "backendScopes") {
			return true
		}
	}
	return false
}

func fieldListPatchesAPISourcePermissions(fields []string) bool {
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if strings.EqualFold(f, "sourcePermissions") || strings.EqualFold(f, "scopes") {
			return true
		}
	}
	return false
}

// applyScopesFromAPI converts API scopes (with nested permissions) into BackendScopes.
// Legacy sourcePermissions map is merged into scopes when permissions are omitted per scope.
func applyScopesFromAPI(user *users.User) error {
	if len(user.SourcePermissions) > 0 && len(user.FrontendScopes) > 0 {
		byName := make(map[string]users.SourceFilePermissions, len(user.SourcePermissions))
		for name, perms := range user.SourcePermissions {
			byName[name] = perms
		}
		for i, scope := range user.FrontendScopes {
			if scope.Permissions != nil {
				continue
			}
			if perms, ok := byName[scope.Name]; ok {
				p := perms
				user.FrontendScopes[i].Permissions = &p
			}
		}
	}
	defaults := settings.DefaultSourceFilePermissions()
	if user.Permissions.Admin {
		defaults = settings.AdminSourceFilePermissions()
	}
	for i, scope := range user.FrontendScopes {
		if scope.Permissions != nil {
			continue
		}
		p := defaults
		user.FrontendScopes[i].Permissions = &p
	}
	if len(user.FrontendScopes) > 0 {
		backend, convErr := users.APIScopesToBackend(user.FrontendScopes)
		if convErr != nil {
			return convErr
		}
		user.BackendScopes = backend
	} else if len(user.SourcePermissions) > 0 {
		backendPerms, convErr := users.APISourcePermsToBackend(user.SourcePermissions)
		if convErr != nil {
			return convErr
		}
		for i, scope := range user.BackendScopes {
			if perms, ok := backendPerms[scope.Path]; ok {
				user.BackendScopes[i].Permissions = perms
			}
		}
	}
	user.FrontendScopes = nil
	user.SourcePermissions = nil
	return nil
}

// findFieldByJSONTag recursively searches for a struct field by its JSON tag name
// Handles embedded structs by searching through all levels
func findFieldByJSONTag(t reflect.Type, jsonTag string) string {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check if this field's JSON tag matches
		jsonTagValue := field.Tag.Get("json")
		if jsonTagValue != "" {
			// Parse tag (might be "fieldName,omitempty")
			tagName := strings.Split(jsonTagValue, ",")[0]
			if tagName == jsonTag {
				return field.Name
			}
		}

		// If this is an embedded struct (Anonymous field), search recursively
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if nestedFieldName := findFieldByJSONTag(field.Type, jsonTag); nestedFieldName != "" {
				// For embedded structs, we can access the field directly by name
				// because Go promotes embedded struct fields
				return nestedFieldName
			}
		}
	}

	return ""
}

// DeleteUser deletes a user by stable numeric id (non-zero).
func DeleteUser(id uint64) error {
	if id == 0 {
		return fmt.Errorf("user not found in state")
	}
	usersMux.Lock()
	defer usersMux.Unlock()

	user, err := loadExistingUserLocked(&users.User{ID: id})
	if err != nil {
		return fmt.Errorf("user not found in state")
	}

	if err := sqlDb.DeleteUserByID(id); err != nil {
		return err
	}

	deleteUserFromCache(user)

	if accessDb != nil {
		_ = accessDb.RemoveHashedTokensForUser(id)
	}

	return nil
}

// DeleteUserByUsername deletes a user by username
func DeleteUserByUsername(username string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	user, err := loadExistingUserLocked(&users.User{FrontendUser: users.FrontendUser{Username: username}})
	if err != nil {
		return fmt.Errorf("user not found in state")
	}

	uid := user.ID
	if err := sqlDb.DeleteUserByUsername(username); err != nil {
		return err
	}

	deleteUserFromCache(user)

	if accessDb != nil && uid != 0 {
		_ = accessDb.RemoveHashedTokensForUser(uid)
	}

	return nil
}

// TokenNameForRawToken returns the persisted token name when rawToken matches a stored API key.
func TokenNameForRawToken(user *users.User, rawToken string) (string, bool) {
	if user == nil {
		return "", false
	}
	usersMux.RLock()
	defer usersMux.RUnlock()
	return users.TokenNameByRaw(user.Tokens, rawToken)
}

// AddUserToken adds an API token to a user
func AddUserToken(ownerUsername string, token users.AuthToken) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	user, err := loadExistingUserLocked(&users.User{FrontendUser: users.FrontendUser{Username: ownerUsername}})
	if err != nil {
		return fmt.Errorf("user not found in state")
	}

	// Check if token already exists
	if user.Tokens != nil {
		if _, tokenExists := user.Tokens[token.Name]; tokenExists {
			return fmt.Errorf("token with name %s already exists for user", token.Name)
		}
	}

	// Prepare the update
	if user.Tokens == nil {
		user.Tokens = make(map[string]users.AuthToken)
	}
	users.StoreToken(user.Tokens, token)

	if err := sqlDb.UpdateUser(user); err != nil {
		return err
	}

	putUserInCache(user)

	return nil
}

// DeleteUserToken removes an API token from a user
func DeleteUserToken(ownerUsername string, tokenName string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	user, err := loadExistingUserLocked(&users.User{FrontendUser: users.FrontendUser{Username: ownerUsername}})
	if err != nil {
		return fmt.Errorf("user not found in state")
	}

	// Check if token exists
	if user.Tokens == nil {
		return fmt.Errorf("user has no tokens")
	}
	if _, tokenExists := user.Tokens[tokenName]; !tokenExists {
		return fmt.Errorf("token with name %s not found for user", tokenName)
	}

	users.RemoveTokenByName(user.Tokens, tokenName)

	if err := sqlDb.UpdateUser(user); err != nil {
		return err
	}

	putUserInCache(user)

	return nil
}
