package http

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

var (
	revokedApiKeyList map[string]bool
	revokeMu          sync.Mutex
)

type extractor []string

func (e extractor) ExtractToken(r *http.Request) (string, error) {
	tokenObj, err := r.Cookie("auth")
	if err != nil {
		return "", err
	}
	token := tokenObj.Value

	// Checks if the token isn't empty and if it contains two dots.
	// The former prevents incompatibility with URLs that previously
	// used basic auth.
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}

	auth := r.URL.Query().Get("auth")
	if auth != "" && strings.Count(auth, ".") == 2 {
		return auth, nil
	}

	if r.Method == http.MethodGet {
		cookie, _ := r.Cookie("auth")
		if cookie != nil && strings.Count(cookie.Value, ".") == 2 {
			return cookie.Value, nil
		}
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

	userHome, err := config.MakeUserDir(user.Username, user.Scope, config.Server.Root)
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	user.Scope = userHome
	log.Printf("new user: %s, home dir: [%s].", user.Username, userHome)
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
	signed, err := makeSignedTokenAPI(*user, "WEB_TOKEN", time.Hour*2, user.Perm)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// Perform the user update
	err = store.Users.AddApiKey(user.ID, *signed)
	if err != nil {
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

func makeSignedTokenAPI(user users.User, name string, duration time.Duration, perms users.Permissions) (*users.AuthToken, error) {
	claims := &users.AuthToken{
		Permissions: perms,
		Created:     time.Now().Unix(),
		Expires:     time.Now().Add(duration).Unix(),
		Name:        name,
		BelongsTo:   user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			Issuer:    "FileBrowser Quantum",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(config.Auth.Key)
	claims.Key = tokenString
	return claims, err
}
