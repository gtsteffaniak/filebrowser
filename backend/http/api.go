package http

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// createApiTokenHandler creates an API token for the user.
// @Summary Create API Token
// @Description Create an API token with specified name, duration, and permissions.
// @Tags Auth
// @Accept json
// @Produce json
// @Param name query string true "Name of the API token"
// @Param days query string true "Duration of the API token in days"
// @Param permissions query string true "Permissions for the API token (comma-separated)"
// @Success 200 {object} HttpResponse "Token created successfully, response contains json object with token"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 409 {object} map[string]string "Conflict"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token [post]
func createApiTokenHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	durationStr := r.URL.Query().Get("days")
	permissionsStr := r.URL.Query().Get("permissions")
	minimal := permissionsStr == ""

	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to create api tokens")
	}

	if name == "" || strings.HasPrefix(name, "WEB_TOKEN") {
		return http.StatusBadRequest, fmt.Errorf("api token name must be valid")
	}
	if durationStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api token duration must be valid")
	}

	// For full tokens (minimal=false), permissions are required in the claim
	// For minimal tokens (minimal=true), permissions are not in the token
	var permissions users.Permissions
	if !minimal {
		// Parse permissions from the query parameter
		permissions = users.Permissions{
			Api:      strings.Contains(permissionsStr, "api") && d.user.Permissions.Api,
			Admin:    strings.Contains(permissionsStr, "admin") && d.user.Permissions.Admin,
			Modify:   strings.Contains(permissionsStr, "modify") && d.user.Permissions.Modify,
			Delete:   strings.Contains(permissionsStr, "delete") && d.user.Permissions.Delete,
			Create:   strings.Contains(permissionsStr, "create") && d.user.Permissions.Create,
			Share:    strings.Contains(permissionsStr, "share") && d.user.Permissions.Share,
			Realtime: strings.Contains(permissionsStr, "realtime") && d.user.Permissions.Realtime,
			Download: strings.Contains(permissionsStr, "download") && d.user.Permissions.Download,
		}
	}

	// Convert the duration string to an int64
	durationInt, err := strconv.ParseInt(durationStr, 10, 64) // Base 10 and bit size of 64
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration value: %w", err)
	}

	// Here we assume the duration is in seconds; convert to time.Duration
	duration := time.Duration(durationInt) * time.Hour * 24
	tokenString, authToken, err := auth.MakeSignedTokenAPI(d.user, name, duration, permissions, minimal)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return http.StatusInternalServerError, err
	}

	// Store API token metadata in user's Tokens map
	err = store.Users.AddApiToken(d.user.ID, name, tokenString, authToken)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Store token hash → user ID mapping in access storage for fast lookups
	err = store.Access.AddApiToken(tokenString, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	response := HttpResponse{
		Message: "here is your token!",
		Token:   tokenString,
	}
	return renderJSON(w, r, response)
}

// deleteApiTokenHandler deletes an API token for the user.
// @Summary Delete API token
// @Description Delete an API token with specified name.
// @Tags Auth
// @Accept json
// @Produce json
// @Param name query string true "Name of the API token to delete"
// @Success 200 {object} HttpResponse "API token deleted successfully"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token [delete]
func deleteApiTokenHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to delete api tokens")
	}

	tokenInfo, ok := d.user.Tokens[name]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("api token not found")
	}

	// Perform the user update
	err := store.Users.DeleteApiToken(d.user.ID, name)
	if err != nil {
		return http.StatusNotFound, err
	}

	// Revoke the token (adds to RevokedTokens set)
	if err := auth.RevokeApiToken(store.Access, tokenInfo.Token); err != nil {
		logger.Errorf("Failed to revoke token: %v", err)
	}
	if err := store.Access.RemoveApiToken(tokenInfo.Token); err != nil {
		logger.Errorf("Failed to remove api token: %v", err)
	}

	response := HttpResponse{
		Message: "successfully deleted api token from user",
	}
	return renderJSON(w, r, response)
}

type AuthTokenFrontend struct {
	Token       string            `json:"token"`
	Name        string            `json:"name"`
	IssuedAt    int64             `json:"issuedAt"`
	ExpiresAt   int64             `json:"expiresAt"`
	Permissions users.Permissions `json:"Permissions,omitempty"`
}

// listApiTokensHandler lists all API tokens or retrieves details for a specific token.
// @Summary List API tokens
// @Description List all API tokens or retrieve details for a specific token.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {array} AuthTokenFrontend "List of API tokens"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token/list [get]
func listApiTokensHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to list api tokens")
	}
	if len(d.user.Tokens) == 0 {
		return http.StatusNotFound, fmt.Errorf("no api tokens found")
	}
	AuthTokensFrontend := make([]AuthTokenFrontend, 0, len(d.user.Tokens))
	for name, token := range d.user.Tokens {
		AuthTokensFrontend = append(AuthTokensFrontend, AuthTokenFrontend{
			Token:       token.Token,
			Name:        name,
			IssuedAt:    token.RegisteredClaims.IssuedAt.Unix(),
			ExpiresAt:   token.RegisteredClaims.ExpiresAt.Unix(),
			Permissions: token.Permissions,
		})
	}
	
	sort.Slice(AuthTokensFrontend, func(i, j int) bool {
		return AuthTokensFrontend[i].Name < AuthTokensFrontend[j].Name
	})

	return renderJSON(w, r, AuthTokensFrontend)
}

// getApiTokenHandler gets a specific API token.
// @Summary Get API token
// @Description Get a specific API token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param name query string true "Name of the API token to retrieve"
// @Success 200 {object} AuthTokenFrontend "API token details"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/token [get]
func getApiTokenHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to list api tokens")
	}
	tokenInfo, ok := d.user.Tokens[name]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("api token not found")
	}
	AuthTokenFrontendResponse := AuthTokenFrontend{
		Token:       tokenInfo.Token,
		Name:        name,
		IssuedAt:    tokenInfo.IssuedAt,
		ExpiresAt:   tokenInfo.ExpiresAt,
		Permissions: tokenInfo.Permissions,
	}
	return renderJSON(w, r, AuthTokenFrontendResponse)
}
