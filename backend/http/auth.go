package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	libError "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
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
	tokenObj, err := r.Cookie("filebrowser_quantum_jwt")
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
		if len(parts) > 1 {
			// some clients don't respect RFC regarding cases
			switch strings.ToLower(parts[0]) {
			case "bearer":
				return parts[1], nil
			case "basic":
				// compatibility for basic auth
				// user ignored, password is token
				_, token, ok := r.BasicAuth()
				if ok {
					return token, nil
				}
			}
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
			user := users.User{
				LoginMethod: users.LoginMethodProxy,
				Username:    proxyUser,
			}
			settings.ApplyUserDefaults(&user)
			if user.Username == config.Auth.AdminUsername {
				user.Permissions.Admin = true
			}
			err = storage.CreateUser(user, user.Permissions)
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
		return nil, errors.ErrWrongLoginMethod
	}
	if data.user.Username == config.Auth.AdminUsername && !data.user.Permissions.Admin {
		data.user.Permissions.Admin = true
		err = store.Users.Update(data.user, true, "Permissions")
		if err != nil {
			return nil, err
		}
	}
	return data.user, nil
}

// loginHandler handles user authentication via password.
// @Summary User login
// @Description Authenticate a user with a username and password. The password must be URL-encoded and sent in the X-Password header to support special characters (e.g., ^, %, £, €, etc.).
// @Tags Auth
// @Accept json
// @Produce json
// @Param username query string true "Username"
// @Param recaptcha query string false "ReCaptcha response token (if enabled)"
// @Param X-Password header string true "URL-encoded password"
// @Param X-Secret header string false "TOTP code (if 2FA is enabled)"
// @Success 200 {string} string "JWT token for authentication"
// @Failure 403 {object} map[string]string "Forbidden - authentication failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/login [post]
func loginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.user.LoginMethod == users.LoginMethodProxy {
		return printToken(w, r, d.user)
	}
	if d.user.LoginMethod == users.LoginMethodLdap {
		return printToken(w, r, d.user)
	}
	passwordUser := d.user.LoginMethod == users.LoginMethodPassword
	enforcedOtp := config.Auth.Methods.PasswordAuth.EnforcedOtp
	missingOtp := d.user.TOTPSecret == ""
	if passwordUser && enforcedOtp && missingOtp {
		return http.StatusForbidden, errors.ErrNoTotpConfigured
	}
	return printToken(w, r, d.user)
}

// logoutHandler handles user logout
// @Summary User Logout
// @Description Returns a logout URL for the frontend to redirect to.
// @Tags Auth
// @Produce json
// @Param auth query string false "JWT token"
// @Success 200 {object} map[string]string "{"logoutUrl": "http://..."}"
// @Router /api/auth/logout [post]
func logoutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	defer auth.RevokeAPIKey(d.token)

	// Clear the authentication cookie by setting it to expire in the past
	// Get the correct domain for cookie - prefer X-Forwarded-Host from reverse proxy
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	cookie := &http.Cookie{
		Name:     "filebrowser_quantum_jwt",
		Value:    "",
		Domain:   strings.Split(host, ":")[0],
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0), // Expire immediately
		MaxAge:   -1,              // Delete cookie
	}
	http.SetCookie(w, cookie)

	logoutUrl := fmt.Sprintf("%vlogin", config.Server.BaseURL) // Default fallback
	if d.user != nil && d.user.LoginMethod == users.LoginMethodProxy {
		proxyRedirectUrl := config.Auth.Methods.ProxyAuth.LogoutRedirectUrl
		if proxyRedirectUrl != "" {
			logoutUrl = proxyRedirectUrl
		}
	} else if d.user != nil && d.user.LoginMethod == users.LoginMethodOidc {
		oidcRedirectUrl := config.Auth.Methods.OidcAuth.LogoutRedirectUrl
		if oidcRedirectUrl != "" {
			logoutUrl = oidcRedirectUrl
		}
	} else if d.user != nil && d.user.LoginMethod == users.LoginMethodLdap {
		ldapRedirectUrl := config.Auth.Methods.LdapAuth.LogoutRedirectUrl
		if ldapRedirectUrl != "" {
			logoutUrl = ldapRedirectUrl
		}
	}
	if logoutUrl == "" {
		logger.Debug("no logout url found, using default")
		logoutUrl = fmt.Sprintf("%vlogin", config.Server.BaseURL)
	}
	response := map[string]string{
		"logoutUrl": logoutUrl,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

// signupHandler registers a new user account.
// @Summary User signup
// @Description Register a new user account with a username and password.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 201 {string} string "User created successfully"
// @Failure 400 {object} map[string]string "Bad request - invalid input"
// @Failure 405 {object} map[string]string "Method not allowed - signup is disabled"
// @Failure 409 {object} map[string]string "Conflict - user already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/signup [post]
func signupHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.Config.Auth.Methods.PasswordAuth.Signup {
		return http.StatusMethodNotAllowed, fmt.Errorf("signup is disabled")
	}

	// Get credentials from query parameters
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")

	// Validate that we have both username and password
	if username == "" || password == "" {
		return http.StatusBadRequest, fmt.Errorf("username and password are required")
	}

	user := users.User{
		Username: username,
		NonAdminEditable: users.NonAdminEditable{
			Password: password,
		},
		LoginMethod: users.LoginMethodPassword,
	}
	err := storage.CreateUser(user, settings.ConvertPermissionsToUsers(settings.Config.UserDefaults.Permissions))
	if err != nil {
		logger.Debug(err.Error())
		// Return the actual error message instead of a generic one
		return http.StatusBadRequest, err
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

func printToken(w http.ResponseWriter, r *http.Request, user *users.User) (int, error) {
	expires := time.Hour * time.Duration(config.Auth.TokenExpirationHours)
	signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), expires, user.Permissions, false)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return 401, errors.ErrUnauthorized
	}

	// Add 30 minutes buffer so expired token doesn't get automatically deleted by the browser
	// This allows backend to identify expired sessions and provide better user feedback
	expiresTime := time.Now().Add(expires).Add(time.Minute * 30)

	setSessionCookie(w, r, signed.Key, expiresTime)

	// Still return token in body for backward compatibility and state management
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed.Key)); err != nil {
		return 401, errors.ErrUnauthorized
	}
	return 0, nil
}

func makeSignedTokenAPI(user *users.User, name string, duration time.Duration, perms users.Permissions, minimal bool) (users.AuthToken, error) {
	_, ok := user.ApiKeys[name]
	if ok {
		return users.AuthToken{}, fmt.Errorf("key already exists with same name %v ", name)
	}
	now := time.Now()
	expires := now.Add(duration)

	var tokenString string
	var err error

	if minimal {
		// Create minimal token with only JWT standard claims
		minimalClaim := users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(expires),
				Issuer:    "FileBrowser Quantum",
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, minimalClaim)
		tokenString, err = token.SignedString([]byte(config.Auth.Key))
		if err != nil {
			return users.AuthToken{}, err
		}
	} else {
		// Create full token with permissions and user ID
		fullClaim := users.AuthToken{
			MinimalAuthToken: users.MinimalAuthToken{
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(expires),
					Issuer:    "FileBrowser Quantum",
				},
			},
			Name:        name,
			Permissions: perms,
			BelongsTo:   user.ID,
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, fullClaim)
		tokenString, err = token.SignedString([]byte(config.Auth.Key))
		if err != nil {
			return users.AuthToken{}, err
		}
	}

	// Create the AuthToken to store in database (always includes permissions and user ID)
	storedClaim := users.AuthToken{
		MinimalAuthToken: users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(expires),
				Issuer:    "FileBrowser Quantum",
			},
		},
		Key:         tokenString,
		Name:        name,
		Permissions: perms,
		BelongsTo:   user.ID,
	}

	if strings.HasPrefix(name, "WEB_TOKEN") {
		// don't add to api tokens, its a short lived web token
		return storedClaim, nil
	}

	// Perform the user update
	err = store.Users.AddApiKey(user.ID, name, storedClaim)
	if err != nil {
		return storedClaim, err
	}
	return storedClaim, nil
}

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 200, nil
	}

	tokenParam := r.URL.Query().Get("token")
	if tokenParam != "" {
		// Verify the token signature if it's in the new signed format
		if strings.Contains(tokenParam, ".") {
			parts := strings.Split(tokenParam, ".")
			if len(parts) == 2 {
				payload := parts[0]
				signature := parts[1]

				// Verify HMAC signature
				mac := hmac.New(sha256.New, []byte(config.Auth.Key))
				mac.Write([]byte(payload))
				expectedSignature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

				// Use constant-time comparison to prevent timing attacks
				if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
					// Token signature is valid, now check if it matches stored token
					if tokenParam == l.Token {
						return 200, nil
					}
				}
			}
		} else {
			// Legacy token format (plain base64) - direct comparison
			if tokenParam == l.Token {
				return 200, nil
			}
		}
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if libError.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 401, err
	}
	return 200, nil
}

// setSessionCookie - sets the authentication token as an HTTP cookie
// Get the correct domain for cookie - prefer X-Forwarded-Host from reverse proxy
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresTime time.Time) {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	cookie := &http.Cookie{
		Name:     "filebrowser_quantum_jwt",
		Value:    token,
		Domain:   strings.Split(host, ":")[0], // Set domain to the host without port
		Path:     "/",
		SameSite: http.SameSiteLaxMode, // Lax mode allows cookie on navigation from OIDC provider
		Expires:  expiresTime,
		// HttpOnly: true, // Cannot use HttpOnly since frontend needs to read cookie for renew operations
		// Secure: true, // Enable this in production with HTTPS
	}
	http.SetCookie(w, cookie)
}
