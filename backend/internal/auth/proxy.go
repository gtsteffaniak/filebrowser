package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header string `json:"header"`
}

// AuthenticateProxy authenticates the user via an HTTP header.
func AuthenticateProxy(r *http.Request, usr *users.Storage, headerName string) (*users.User, error) {
	username := r.Header.Get(headerName)
	id, err := users.ResolveUsernameToID(username)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}
	if err != nil {
		return nil, err
	}
	user, err := usr.Get(id)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}

	return user, err
}

// Auth authenticates the user via an HTTP header (legacy method for compatibility).
func (a ProxyAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	return AuthenticateProxy(r, usr, a.Header)
}

// ExtractGroupsFromHeader reads group/role values from a proxy header named by groupsClaim.
// Returns present=false when the header name is empty or the header is missing/blank.
func ExtractGroupsFromHeader(r *http.Request, headerName string) ([]string, bool) {
	if headerName == "" {
		return nil, false
	}
	headerVal := strings.TrimSpace(r.Header.Get(headerName))
	if headerVal == "" {
		return nil, false
	}
	if strings.HasPrefix(headerVal, "[") {
		var groups []string
		if err := json.Unmarshal([]byte(headerVal), &groups); err == nil {
			return groups, true
		}
	}
	groups := ExtractGroupsFromClaims(map[string]interface{}{headerName: headerVal}, headerName)
	if len(groups) == 1 && strings.Contains(groups[0], ",") {
		parts := strings.Split(groups[0], ",")
		groups = make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				groups = append(groups, part)
			}
		}
	}
	return groups, true
}
