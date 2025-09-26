package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

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

	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to create api keys")
	}

	if name == "" {
		return http.StatusBadRequest, fmt.Errorf("api name must be valid")
	}
	if durationStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api duration must be valid")
	}
	if permissionsStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api permissions must be valid")
	}
	// Parse permissions from the query parameter
	permissions := users.Permissions{
		Api:      strings.Contains(permissionsStr, "api") && d.user.Permissions.Api,
		Admin:    strings.Contains(permissionsStr, "admin") && d.user.Permissions.Admin,
		Modify:   strings.Contains(permissionsStr, "modify") && d.user.Permissions.Modify,
		Share:    strings.Contains(permissionsStr, "share") && d.user.Permissions.Share,
		Realtime: strings.Contains(permissionsStr, "realtime") && d.user.Permissions.Realtime,
	}

	// Convert the duration string to an int64
	durationInt, err := strconv.ParseInt(durationStr, 10, 64) // Base 10 and bit size of 64
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration value: %w", err)
	}

	// Here we assume the duration is in seconds; convert to time.Duration
	duration := time.Duration(durationInt) * time.Hour * 24
	// get request body like:
	token, err := makeSignedTokenAPI(d.user, name, duration, permissions)
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
	Permissions users.Permissions `json:"Permissions"`
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
		modifiedKey := AuthTokenMin{
			Key:         keyInfo.Key,
			Name:        key,
			Created:     keyInfo.Created,
			Expires:     keyInfo.Expires,
			Permissions: keyInfo.Permissions,
		}
		return renderJSON(w, r, modifiedKey)
	}

	modifiedList := map[string]AuthTokenMin{}
	for key, value := range d.user.ApiKeys {
		modifiedList[key] = AuthTokenMin{
			Key:         value.Key,
			Created:     value.Created,
			Expires:     value.Expires,
			Permissions: value.Permissions,
		}
	}

	return renderJSON(w, r, modifiedList)
}
