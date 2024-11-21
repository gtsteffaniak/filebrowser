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
	durationStr := r.URL.Query().Get("days")
	permissionsStr := r.URL.Query().Get("permissions")

	if name == "" {
		return http.StatusInternalServerError, fmt.Errorf("api name must be valid")
	}
	if durationStr == "" {
		return http.StatusInternalServerError, fmt.Errorf("api duration must be valid")
	}
	if permissionsStr == "" {
		return http.StatusInternalServerError, fmt.Errorf("api permissions must be valid")
	}
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

func deleteApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")

	keyInfo, ok := d.user.ApiKeys[name]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("api key not found")
	}
	// Perform the user update
	err := store.Users.DeleteApiKey(d.user.ID, name)
	if err != nil {
		return http.StatusNotFound, err
	}

	revokeAPIKey(keyInfo.Key) // add to blacklist
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
	key := r.URL.Query().Get("key")

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
