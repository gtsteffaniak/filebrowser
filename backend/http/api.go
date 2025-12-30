package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	ApiTokenUserCache = cache.NewCache[uint](24 * time.Hour) // API token to user ID mapping gets cached for 24 hours
)

// getUserFromApiToken finds the user ID associated with a given API token
func getUserFromApiToken(token string) (uint, error) {
	// Check cache first
	if cached, ok := ApiTokenUserCache.Get(token); ok {
		return cached, nil
	}

	// Get all users
	allUsers, err := store.Users.Gets()
	if err != nil {
		return 0, fmt.Errorf("failed to get all users: %w", err)
	}

	// Iterate through all users and their API keys
	for _, user := range allUsers {
		if user.ApiKeys == nil {
			continue
		}
		for _, apiKey := range user.ApiKeys {
			if apiKey.Key == token {
				// Cache the result
				ApiTokenUserCache.Set(token, user.ID)
				return user.ID, nil
			}
		}
	}

	// Token not found
	return 0, fmt.Errorf("token not found")
}

// createApiKeyHandler creates an API key for the user.
// @Summary Create API key
// @Description Create an API key with specified name, duration, and permissions.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string true "Name of the API key"
// @Param days query string true "Duration of the API key in days"
// @Param permissions query string true "Permissions for the API key (comma-separated)"
// @Success 200 {object} HttpResponse "Token created successfully, response contains json object with token key"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 409 {object} map[string]string "Conflict"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token [put]
func createApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	durationStr := r.URL.Query().Get("days")
	permissionsStr := r.URL.Query().Get("permissions")
	statefulStr := r.URL.Query().Get("stateful")

	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to create api keys")
	}

	if name == "" {
		return http.StatusBadRequest, fmt.Errorf("api name must be valid")
	}
	if durationStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api duration must be valid")
	}

	// Parse stateful parameter (defaults to false if not specified = full token)
	stateful := false
	if statefulStr != "" {
		stateful, _ = strconv.ParseBool(statefulStr)
	}

	// For full tokens (stateful=false), permissions are required in the claim
	// For minimal tokens (stateful=true), permissions are not in the token, state is on backend
	var permissions users.Permissions
	if !stateful {
		if permissionsStr == "" {
			return http.StatusBadRequest, fmt.Errorf("api permissions must be valid for full tokens")
		}
		// Parse permissions from the query parameter
		permissions = users.Permissions{
			Api:      strings.Contains(permissionsStr, "api") && d.user.Permissions.Api,
			Admin:    strings.Contains(permissionsStr, "admin") && d.user.Permissions.Admin,
			Modify:   strings.Contains(permissionsStr, "modify") && d.user.Permissions.Modify,
			Delete:   strings.Contains(permissionsStr, "delete") && d.user.Permissions.Delete,
			Create:   strings.Contains(permissionsStr, "create") && d.user.Permissions.Create,
			Share:    strings.Contains(permissionsStr, "share") && d.user.Permissions.Share,
			Realtime: strings.Contains(permissionsStr, "realtime") && d.user.Permissions.Realtime,
		}
	}

	// Convert the duration string to an int64
	durationInt, err := strconv.ParseInt(durationStr, 10, 64) // Base 10 and bit size of 64
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration value: %w", err)
	}

	// Here we assume the duration is in seconds; convert to time.Duration
	duration := time.Duration(durationInt) * time.Hour * 24
	// get request body like:
	token, err := makeSignedTokenAPI(d.user, name, duration, permissions, stateful)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return http.StatusInternalServerError, err
	}
	response := HttpResponse{
		Message: "here is your token!",
		Token:   token.Key,
	}
	return renderJSON(w, r, response)
}

// deleteApiKeyHandler deletes an API key for the user.
// @Summary Delete API key
// @Description Delete an API key with specified name.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string true "Name of the API key to delete"
// @Success 200 {object} HttpResponse "API key deleted successfully"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token [delete]
func deleteApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to delete api keys")
	}

	keyInfo, ok := d.user.ApiKeys[name]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("api key not found")
	}
	// Perform the user update
	err := store.Users.DeleteApiKey(d.user.ID, name)
	if err != nil {
		return http.StatusNotFound, err
	}

	auth.RevokeAPIKey(keyInfo.Key) // add to blacklist
	response := HttpResponse{
		Message: "successfully deleted api key from user",
	}
	return renderJSON(w, r, response)
}

type AuthTokenMin struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Created     int64             `json:"created"`
	Expires     int64             `json:"expires"`
	Permissions users.Permissions `json:"Permissions,omitempty"`
	Stateful    bool              `json:"stateful"`
}

// listApiKeysHandler lists all API keys or retrieves details for a specific key.
// @Summary List API keys
// @Description List all API keys or retrieve details for a specific key.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string false "Name of the API to retrieve details"
// @Success 200 {object} AuthTokenMin "List of API keys or specific key details"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/tokens [get]
func listApiKeysHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	key := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to list api keys")
	}

	if key != "" {
		keyInfo, ok := d.user.ApiKeys[key]
		if !ok {
			return http.StatusNotFound, fmt.Errorf("api key not found")
		}
		// Determine if token is minimal/stateful by parsing the JWT claim
		// For minimal tokens, the JWT claim doesn't have BelongsTo even though the database record does
		stateful := false
		if keyInfo.Key != "" {
			keyFunc := func(token *jwt.Token) (interface{}, error) {
				return []byte(config.Auth.Key), nil
			}
			var claim users.AuthToken
			token, err := jwt.ParseWithClaims(keyInfo.Key, &claim, keyFunc)
			if err == nil && token.Valid {
				// If the parsed claim doesn't have BelongsTo, it's a minimal token
				stateful = claim.BelongsTo == 0
			}
		}
		modifiedKey := AuthTokenMin{
			Key:      keyInfo.Key,
			Name:     key,
			Created:  0,
			Expires:  0,
			Stateful: stateful,
		}
		// Safely access IssuedAt and ExpiresAt (they're pointers)
		if keyInfo.IssuedAt != nil {
			modifiedKey.Created = keyInfo.IssuedAt.Unix()
		}
		if keyInfo.ExpiresAt != nil {
			modifiedKey.Expires = keyInfo.ExpiresAt.Unix()
		}
		// Only include Permissions for full tokens (not minimal/stateful)
		if !stateful {
			modifiedKey.Permissions = keyInfo.Permissions
		}
		return renderJSON(w, r, modifiedKey)
	}

	modifiedList := map[string]AuthTokenMin{}
	for key, value := range d.user.ApiKeys {
		// Determine if token is minimal/stateful by parsing the JWT claim
		// For minimal tokens, the JWT claim doesn't have BelongsTo even though the database record does
		stateful := false
		if value.Key != "" {
			keyFunc := func(token *jwt.Token) (interface{}, error) {
				return []byte(config.Auth.Key), nil
			}
			var claim users.AuthToken
			token, err := jwt.ParseWithClaims(value.Key, &claim, keyFunc)
			if err == nil && token.Valid {
				// If the parsed claim doesn't have BelongsTo, it's a minimal token
				stateful = claim.BelongsTo == 0
			}
		}
		modifiedKey := AuthTokenMin{
			Key:      value.Key,
			Created:  0,
			Expires:  0,
			Stateful: stateful,
		}
		// Safely access IssuedAt and ExpiresAt (they're pointers)
		if value.IssuedAt != nil {
			modifiedKey.Created = value.IssuedAt.Unix()
		}
		if value.ExpiresAt != nil {
			modifiedKey.Expires = value.ExpiresAt.Unix()
		}
		// Only include Permissions for full tokens (not minimal/stateful)
		if !stateful {
			modifiedKey.Permissions = value.Permissions
		}
		modifiedList[key] = modifiedKey
	}

	return renderJSON(w, r, modifiedList)
}
