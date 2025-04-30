package http

import (
	"encoding/json"
	libError "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
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
			hashpass, err := users.HashPwd(proxyUser)
			if err != nil {
				return nil, err
			}
			err = storage.CreateUser(users.User{
				LoginMethod: users.LoginMethodProxy,
				Username:    proxyUser,
				NonAdminEditable: users.NonAdminEditable{
					Password: hashpass, // hashed password that can't actually be used
				},
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
func loginHandler(w http.ResponseWriter, r *http.Request) {
	proxyUser := r.Header.Get(config.Auth.Methods.ProxyAuth.Header)
	if config.Auth.Methods.ProxyAuth.Enabled && proxyUser != "" {
		user, err := setupProxyUser(r, &requestContext{}, proxyUser)
		if err != nil {
			http.Error(w, err.Error(), 403)
			return
		}
		status, err := printToken(w, r, user) // Pass the data object
		if err != nil {
			http.Error(w, http.StatusText(status), status)
		}
		return
	}
	if !config.Auth.Methods.PasswordAuth.Enabled {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	// currently only supports user/pass
	// Get the authentication method from the settings
	auther, err := store.Auth.Get("password")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Authenticate the user based on the request
	user, err := auther.Auth(r, store.Users)
	if err != nil {
		logger.Debug(err.Error())
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
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

type userInfo struct {
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Sub               string `json:"sub"`
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
func signupHandler(w http.ResponseWriter, r *http.Request) {
	if !settings.Config.Auth.Methods.PasswordAuth.Signup {
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
	user := &users.User{}
	settings.ApplyUserDefaults(user)
	user.Username = info.Username
	user.Password = info.Password

	err = store.Users.Save(user, true)
	if err == errors.ErrExist {
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	} else if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
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
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed.Key)); err != nil {
		return http.StatusInternalServerError, err
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

func oidcCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	code := r.URL.Query().Get("code")
	//state := r.URL.Query().Get("state")
	//
	//fmt.Println("State:", state)
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
	}

	redirectURI := fmt.Sprintf("%s/api/auth/oidc/callback", origin)
	// Step 1: Exchange the code for tokens
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", config.Auth.Methods.OidcAuth.ClientID)
	data.Set("client_secret", config.Auth.Methods.OidcAuth.ClientSecret)
	data.Set("redirect_uri", redirectURI)

	req, _ := http.NewRequest("POST", config.Auth.Methods.OidcAuth.TokenUrl, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 500, fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// print body in response
		body, _ := io.ReadAll(resp.Body)
		return 500, fmt.Errorf("failed to fetch token: %v %v", resp.StatusCode, string(body))
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return 500, fmt.Errorf("failed to parse token response: %v", err)
	}
	// Step 2: Use the access token to fetch user info from the UserInfo URL
	userInfoURL := config.Auth.Methods.OidcAuth.UserInfoUrl
	req, _ = http.NewRequest("GET", userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return 500, fmt.Errorf("failed to fetch user info: %v %v", resp.StatusCode, err)
	}
	defer resp.Body.Close()

	var userdata userInfo
	if err = json.NewDecoder(resp.Body).Decode(&userdata); err != nil {
		return 500, fmt.Errorf("failed to parse user info: %v", err)
	}
	return loginWithOidcUser(w, r, userdata)
}

func oidcLoginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Auth.Methods.OidcAuth.Enabled {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
		}

		nonce := utils.InsecureRandomIdentifier(16)
		redirectURI := fmt.Sprintf("%s/api/auth/oidc/callback", origin)

		authURL := fmt.Sprintf("%s?client_id=%s&response_type=code&scope=%s&redirect_uri=%s&state=%s&fb_redirect=%s",
			config.Auth.Methods.OidcAuth.AuthorizationUrl,
			url.QueryEscape(config.Auth.Methods.OidcAuth.ClientID),
			url.QueryEscape(config.Auth.Methods.OidcAuth.Scopes),
			url.QueryEscape(redirectURI),
			nonce,
			r.URL.Query().Get("fb_redirect"),
		)

		http.Redirect(w, r, authURL, http.StatusFound)
		return 0, nil
	}

	return http.StatusForbidden, fmt.Errorf("oidc authentication is not enabled")
}

func loginWithOidcUser(w http.ResponseWriter, r *http.Request, userInfo userInfo) (int, error) {
	username := userInfo.PreferredUsername
	if userInfo.PreferredUsername == "email" {
		username = userInfo.Email
	}
	// Retrieve the user from the store and store it in the context
	user, err := store.Users.Get(username)
	if err != nil {
		if err.Error() != "the resource does not exist" {
			return http.StatusInternalServerError, err
		}
		hashpass := ""
		hashpass, err = users.HashPwd(username) // hashed password that can't actually be used, gets double hashed
		if err != nil {
			return http.StatusInternalServerError, err
		}
		err = storage.CreateUser(users.User{
			LoginMethod: users.LoginMethodOidc,
			Username:    username,
			NonAdminEditable: users.NonAdminEditable{
				Password: hashpass, // hashed password that can't actually be used
			},
		}, false)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		user, err = store.Users.Get(username)
		if err != nil {
			return http.StatusInternalServerError, err
		}
	}
	signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), time.Hour*time.Duration(config.Auth.TokenExpirationHours), user.Permissions)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return http.StatusInternalServerError, err
	}

	// set signed.Key in the cookie as auth
	cookie := &http.Cookie{
		Name:   "auth",
		Value:  signed.Key,
		Domain: strings.Split(r.Host, ":")[0],
		Path:   "/",
	}
	http.SetCookie(w, cookie)
	setUserInResponseWriter(w, user)

	http.Redirect(w, r, "/", http.StatusFound)
	return 200, nil
}
