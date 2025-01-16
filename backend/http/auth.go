package http

import (
	"encoding/json"
	libError "errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/share"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

var (
	revokedApiKeyList map[string]bool
	revokeMu          sync.Mutex
)

// first checks for cookie
// then checks for header Authorization as Bearer token
// then checks for query parameter
func extractToken(r *http.Request) (string, error) {
	hasToken := false
	tokenObj, err := r.Cookie("auth")
	if err == nil {
		hasToken = true
		token := tokenObj.Value
		// Checks if the token isn't empty and if it contains two dots.
		// The former prevents incompatibility with URLs that previously
		// used basic auth.
		if token != "" && strings.Count(token, ".") == 2 {
			return token, nil
		}
	}

	// Check for Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		hasToken = true
		// Split the header to get "Bearer {token}"
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			return token, nil
		}
	}

	auth := r.URL.Query().Get("auth")
	if auth != "" {
		hasToken = true
		if strings.Count(auth, ".") == 2 {
			return auth, nil
		}
	}

	if hasToken {
		return "", fmt.Errorf("invalid token provided")
	}

	return "", request.ErrNoTokenInRequest
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Get the authentication method from the settings
	auther, err := store.Auth.Get(config.Auth.Method)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// Authenticate the user based on the request
	user, err := auther.Auth(r, store.Users)
	if err == os.ErrPermission {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	} else if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	status, err := printToken(w, r, user) // Pass the data object
	if err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

type signupBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	if !settings.Config.Auth.Signup {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if r.Body == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	info := &signupBody{}
	err := json.NewDecoder(r.Body).Decode(info)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if info.Password == "" || info.Username == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	user := settings.ApplyUserDefaults(users.User{})
	user.Username = info.Username
	user.Password = info.Password

	userHome, err := config.MakeUserDir(user.Username, user.Scope, files.RootPaths["default"])
	if err != nil {
		logger.Error(fmt.Sprintf("create user: failed to mkdir user home dir: [%s]", userHome))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	user.Scope = userHome
	logger.Debug(fmt.Sprintf("new user: %s, home dir: [%s].", user.Username, userHome))
	err = store.Users.Save(&user)
	if err == errors.ErrExist {
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	} else if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func renewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// check if x-auth header is present and token is
	return printToken(w, r, d.user)
}

func printToken(w http.ResponseWriter, _ *http.Request, user *users.User) (int, error) {
	signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.GenerateRandomHash(4), time.Hour*2, user.Perm)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed.Key)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func isRevokedApiKey(key string) bool {
	_, exists := revokedApiKeyList[key]
	return exists
}

func revokeAPIKey(key string) {
	revokeMu.Lock()
	delete(revokedApiKeyList, key)
	revokeMu.Unlock()
}

func makeSignedTokenAPI(user *users.User, name string, duration time.Duration, perms users.Permissions) (users.AuthToken, error) {
	_, ok := user.ApiKeys[name]
	if ok {
		return users.AuthToken{}, fmt.Errorf("key already exists with same name %v ", name)
	}
	now := time.Now()
	expires := now.Add(duration)
	claim := users.AuthToken{
		Permissions: perms,
		Created:     now.Unix(),
		Expires:     expires.Unix(),
		Name:        name,
		BelongsTo:   user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			Issuer:    "FileBrowser Quantum",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenString, err := token.SignedString(config.Auth.Key)
	if err != nil {
		return claim, err
	}
	claim.Key = tokenString
	if strings.HasPrefix(name, "WEB_TOKEN") {
		// don't add to api tokens, its a short lived web token
		return claim, err
	}
	// Perform the user update
	err = store.Users.AddApiKey(user.ID, name, claim)
	if err != nil {
		return claim, err
	}
	return claim, err
}

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 200, nil
	}

	if r.URL.Query().Get("token") == l.Token {
		return 200, nil
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return http.StatusUnauthorized, err
	}
	if password == "" {
		return http.StatusUnauthorized, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if libError.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 401, err
	}
	return 200, nil
}
