package http

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

type authToken struct {
	User users.User `json:"user"`
	jwt.RegisteredClaims
}

type extractor []string

func (e extractor) ExtractToken(r *http.Request) (string, error) {
	token, _ := request.HeaderExtractor{"X-Auth"}.ExtractToken(r)

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

func loginHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	// Get the authentication method from the settings
	auther, err := d.store.Auth.Get(d.settings.Auth.Method)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Authenticate the user based on the request
	user, err := auther.Auth(r, d.store.Users)
	if err == os.ErrPermission {
		return http.StatusForbidden, nil
	} else if err != nil {
		return http.StatusInternalServerError, err
	}

	// Print and return the authentication token
	return printToken(w, r, d, user) // Pass the data object
}

type signupBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func signupHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !settings.Config.Auth.Signup {
		return http.StatusMethodNotAllowed, nil
	}

	if r.Body == nil {
		return http.StatusBadRequest, nil
	}

	info := &signupBody{}
	err := json.NewDecoder(r.Body).Decode(info)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if info.Password == "" || info.Username == "" {
		return http.StatusBadRequest, nil
	}

	user := users.ApplyDefaults(users.User{})
	user.Username = info.Username
	user.Password = info.Password

	userHome, err := d.settings.MakeUserDir(user.Username, user.Scope, d.server.Root)
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		return http.StatusInternalServerError, err
	}
	user.Scope = userHome
	log.Printf("new user: %s, home dir: [%s].", user.Username, userHome)
	err = d.store.Users.Save(&user)
	if err == errors.ErrExist {
		return http.StatusConflict, err
	} else if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func renewHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	return printToken(w, r, d, d.user)
}

func printToken(w http.ResponseWriter, _ *http.Request, d *data, user *users.User) (int, error) {
	duration, err := time.ParseDuration(settings.Config.Auth.TokenExpirationTime)
	if err != nil {
		duration = time.Hour * 2 // Default duration if parsing fails
	}
	claims := &authToken{
		User: *user,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			Issuer:    "File Browser",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(d.settings.Auth.Key)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
