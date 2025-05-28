package http

import (
	"encoding/json"
	libError "errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
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

	auth := r.URL.Query().Get("auth")
	if auth != "" {
		hasToken = true
		if strings.Count(auth, ".") == 2 {
			return auth, nil
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

	if hasToken {
		return "", fmt.Errorf("invalid token provided")
	}

	return "", request.ErrNoTokenInRequest
}

func setupProxyUser(r *http.Request, data *requestContext, proxyUser string) (*users.User, error) {
	var err error
	// Retrieve the user from the store and store it in the context
	data.user, err = store.Users.Get(proxyUser)
	if err != nil {
		if err.Error() != "the resource does not exist" {
			return nil, err
		}
		if config.Auth.Methods.ProxyAuth.CreateUser {
			err = storage.CreateUser(users.User{
				LoginMethod: users.LoginMethodProxy,
				Username:    proxyUser,
			}, false)
			if err != nil {
				return nil, err
			}
			data.user, err = store.Users.Get(proxyUser)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("proxy authentication failed - no user found")
		}
	}
	if data.user.LoginMethod != users.LoginMethodProxy {
		logger.Warningf("user %s is not allowed to login with proxy authentication, bypassing and updating login method", data.user.Username)
		data.user.LoginMethod = users.LoginMethodProxy
		// Perform the user update
		err := store.Users.Update(data.user, true, "LoginMethod")
		if err != nil {
			logger.Debug(err.Error())
		}
		//return nil, fmt.Errorf("user %s is not allowed to login with proxy authentication", proxyUser)
	}
	return data.user, nil
}

// loginHandler handles user authentication via password.
// @Summary User login
// @Description Authenticate a user with a username and password.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {string} string "JWT token for authentication"
// @Failure 403 {object} map[string]string "Forbidden - authentication failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/login [post]
func loginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	passwordUser := d.user.LoginMethod == users.LoginMethodPassword
	enforcedOtp := config.Auth.Methods.PasswordAuth.EnforcedOtp
	missingOtp := d.user.TOTPSecret == ""
	if passwordUser && enforcedOtp && missingOtp {
		return http.StatusForbidden, errors.ErrNoTotpConfigured
	}
	return printToken(w, r, d.user) // Pass the data object
}

// logoutHandler handles user logout, specifically used for OIDC.
// @Summary User Logout
// @Description logs a user out of the application.
// @Tags Auth
// @Success 302 {string} string "Redirect to redirect URL if configured in oidc config."
// @Router /api/auth/logout [get]
func logoutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
	}
	logoutUrl := fmt.Sprintf("%s/login", origin)

	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	if oidcCfg.Enabled && oidcCfg.LogoutRedirectUrl != "" {
		logoutUrl = oidcCfg.LogoutRedirectUrl
		http.Redirect(w, r, logoutUrl, http.StatusFound)
		return 302, nil
	}

	http.Redirect(w, r, logoutUrl, http.StatusFound)
	return 302, nil
}

type signupBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// signupHandler registers a new user account.
// @Summary User signup
// @Description Register a new user account with a username and password.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body signupBody true "User signup details"
// @Success 201 {string} string "User created successfully"
// @Failure 400 {object} map[string]string "Bad request - invalid input"
// @Failure 405 {object} map[string]string "Method not allowed - signup is disabled"
// @Failure 409 {object} map[string]string "Conflict - user already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/signup [post]
func signupHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.Config.Auth.Methods.PasswordAuth.Signup {
		return http.StatusMethodNotAllowed, fmt.Errorf("signup is disabled")
	}
	if r.Body == nil {
		return http.StatusBadRequest, fmt.Errorf("no post body provided")
	}
	info := &signupBody{}
	err := json.NewDecoder(r.Body).Decode(info)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %v", err)
	}
	user := users.User{
		Username: info.Username,
		NonAdminEditable: users.NonAdminEditable{
			Password: info.Password,
		},
		LoginMethod: users.LoginMethodPassword,
	}
	err = storage.CreateUser(user, false)
	if err != nil {
		logger.Debug(err.Error())
		w.WriteHeader(http.StatusConflict)
		return http.StatusConflict, fmt.Errorf("user already exists")
	}
	return 201, nil
}

// renewHandler refreshes the authentication token for a logged-in user.
// @Summary Renew authentication token
// @Description Refresh the authentication token for a logged-in user.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {string} string "New JWT token generated"
// @Failure 401 {object} map[string]string "Unauthorized - invalid token"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/renew [post]
func renewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// check if x-auth header is present and token is
	return printToken(w, r, d.user)
}

func printToken(w http.ResponseWriter, _ *http.Request, user *users.User) (int, error) {
	signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), time.Hour*time.Duration(config.Auth.TokenExpirationHours), user.Permissions)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return 401, errors.ErrUnauthorized
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed.Key)); err != nil {
		return 401, errors.ErrUnauthorized
	}
	return 0, nil
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
	tokenString, err := token.SignedString([]byte(config.Auth.Key))
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
