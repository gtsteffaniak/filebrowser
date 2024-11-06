package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/users"
)

func createApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	durationStr := r.URL.Query().Get("duration")
	permissionsStr := r.URL.Query().Get("permissions")
	// Parse permissions from the query parameter
	permissions := users.Permissions{
		Api:      strings.Contains(permissionsStr, "api") && d.user.Perm.Api,
		Admin:    strings.Contains(permissionsStr, "admin") && d.user.Perm.Admin,
		Execute:  strings.Contains(permissionsStr, "execute") && d.user.Perm.Execute,
		Create:   strings.Contains(permissionsStr, "create") && d.user.Perm.Create,
		Rename:   strings.Contains(permissionsStr, "rename") && d.user.Perm.Rename,
		Modify:   strings.Contains(permissionsStr, "modify") && d.user.Perm.Modify,
		Delete:   strings.Contains(permissionsStr, "delete") && d.user.Perm.Delete,
		Share:    strings.Contains(permissionsStr, "share") && d.user.Perm.Share,
		Download: strings.Contains(permissionsStr, "download") && d.user.Perm.Download,
	}

	// Convert the duration string to an int64
	durationInt, err := strconv.ParseInt(durationStr, 10, 64) // Base 10 and bit size of 64
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration value: %w", err)
	}

	// Here we assume the duration is in seconds; convert to time.Duration
	duration := time.Duration(durationInt) * time.Second
	if name == "" {
		return http.StatusInternalServerError, fmt.Errorf("api name must be valid string")
	}

	// get request body like:
	token, err := makeSignedTokenAPI(d.user, name, duration, permissions)
	if err != nil {
		return 500, err
	}
	response := HttpResponse{
		Message: "here is your token!",
		Token:   token.Key,
	}
	return renderJSON(w, r, response)
}

func deleteApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	apiKey := r.URL.Query().Get("apiKey")
	// Perform the user update
	err := store.Users.DeleteApiKey(d.user.ID, apiKey)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	revokeAPIKey(apiKey) // add to blacklist
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

func listApiKeysHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	modifiedList := []AuthTokenMin{}
	for key, value := range d.user.ApiKeys {
		modifiedList = append(modifiedList, AuthTokenMin{
			Name:        key,
			Key:         value.Key,
			Created:     value.Created,
			Expires:     value.Expires,
			Permissions: value.Permissions,
		})
	}

	return renderJSON(w, r, modifiedList)
}
