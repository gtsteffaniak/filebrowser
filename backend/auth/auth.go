package auth

import (
	"net/http"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

var (
	revokedApiKeyList map[string]bool
	revokeMu          sync.Mutex
)

func init() {
	revokedApiKeyList = make(map[string]bool)
}

// Auther is the authentication interface.
type Auther interface {
	// Auth is called to authenticate a request.
	Auth(r *http.Request, userStore *users.Storage) (*users.User, error)
	// LoginPage indicates if this auther needs a login page.
	LoginPage() bool
}

func IsRevokedApiKey(key string) bool {
	_, exists := revokedApiKeyList[key]
	return exists
}

func RevokeAPIKey(key string) {
	revokeMu.Lock()
	defer revokeMu.Unlock()
	revokedApiKeyList[key] = true
}
