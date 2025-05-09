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
	"github.com/lestrrat-go/jwx/v3/jwk"
	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/common/cache"
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
	if data.user.LoginMethod != users.LoginMethodProxy {
		logger.Warning(fmt.Sprintf("user %s is not allowed to login with proxy authentication, bypassing and updating login method", data.user.Username))
		data.user.LoginMethod = users.LoginMethodProxy
		// Perform the user update
		go store.Users.Update(data.user, true, "LoginMethod")
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
	if user.LoginMethod != users.LoginMethodPassword {
		logger.Warning(fmt.Sprintf("user %s is not allowed to login with password authentication, bypassing and updating login method", user.Username))
		user.LoginMethod = users.LoginMethodPassword
		// Perform the user update
		go store.Users.Update(user, true, "LoginMethod")
		//http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		//return
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

// userInfo struct to hold user claims from either UserInfo or ID token
type userInfo struct {
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Sub               string `json:"sub"`
	Phone             string `json:"phone_number"`
}

// getKeySet fetches the JWKS from the given URL, using the provided cache.
func getKeySet(jwksUrl string) (jwk.Set, error) {
	// Check cache first
	value := cache.JwtCache.Get(jwksUrl)
	jwks, ok := value.(jwk.Set)
	if ok {
		return jwks, nil
	}
	logger.Debug("JWKS not found in cache for, fetching...")
	// Fetch the JWKS from the URL
	resp, err := http.Get(jwksUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %v", jwksUrl, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch JWKS from %s: received status code %d with body: %s", jwksUrl, resp.StatusCode, string(bodyBytes))
	}

	// Decode the JWKS using jwk.ParseReader
	jwks, err = jwk.ParseReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %v", err)
	}

	// Store in cache. Assuming your cache handles expiration (e.g., 1 hour).
	cache.JwtCache.Set(jwksUrl, jwks) // Store the jwk.Set
	return jwks, nil                  // Return the jwk.Set after type assertion
}

// oidcCallbackHandler handles the OIDC callback after the user authenticates with the provider.
// It exchanges the authorization code for tokens, then attempts to verify the ID token using JWKS
// if configured, falling back to the UserInfo endpoint if necessary.
func oidcCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	code := r.URL.Query().Get("code")
	// state := r.URL.Query().Get("state") // You might want to validate the state parameter for CSRF protection

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
	client := &http.Client{} // Consider using a client with a timeout
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug(fmt.Sprintf("failed to send token request: %v", err))
		return 500, fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Debug(fmt.Sprintf("failed to fetch token: %v %v", resp.StatusCode, string(body)))
		return 500, fmt.Errorf("failed to fetch token: %v %v", resp.StatusCode, string(body))
	}

	// Decode the token response, expecting access_token and potentially id_token
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"` // OIDC providers include id_token here
	}

	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		logger.Debug(fmt.Sprintf("failed to parse token response: %v", err))
		return 500, fmt.Errorf("failed to parse token response: %v", err)
	}

	var userdata userInfo       // This will hold the user claims, using the original struct
	useUserInfoEndpoint := true // Flag to determine if we need to call the UserInfo endpoint

	// Step 2: Process ID Token if available and JWKS URL is configured
	if tokenResp.IDToken != "" && config.Auth.Methods.OidcAuth.JwksUrl != "" {
		logger.Debug(fmt.Sprintf("ID token received `%v` attempting to verify using JWKS URL", tokenResp.IDToken))
		// getKeySet now returns jwk.Set (interface)
		var jwks jwk.Set
		jwks, err = getKeySet(config.Auth.Methods.OidcAuth.JwksUrl)
		if err != nil {
			// Log the error but don't fail yet, try UserInfo endpoint as a fallback
			logger.Warning("Failed to fetch or decode JWKS: %v. Falling back to UserInfo endpoint. Ensure your OidcAuth.JwksUrl is correct.")
		} else {
			var token *jwt.Token
			// Parse the ID token. We need to provide a key function to `jwt.Parse`
			// that looks up the key in the JWKS based on the token's `kid`.
			token, err = jwt.Parse(tokenResp.IDToken, func(token *jwt.Token) (interface{}, error) {
				switch alg := token.Method.Alg(); alg {
				case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512", "EdDSA":
					// accepted
				default:
					logger.Debug(fmt.Sprintf("unsupported signing method recieved `%v`", alg))
					return nil, fmt.Errorf("unsupported signing method recieved: %v", alg)
				}
				// Find the key in the JWKS that matches the token's kid (Key ID)
				keyID, ok := token.Header["kid"].(string)
				if !ok {
					logger.Debug("kid header not found in ID token")
					return nil, fmt.Errorf("kid header not found in ID token")
				}
				// Use the jwk.Set interface to find the key
				// LookupKeyID returns a slice of jwk.Key and a boolean indicating if any keys were found.
				key, found := jwks.LookupKeyID(keyID) // Called on the jwk.Set interface
				if !found {
					logger.Debug(fmt.Sprintf("key with kid %s not found in JWKS", keyID))
					return nil, fmt.Errorf("key with kid %s not found in JWKS", keyID)
				}
				// Return the public key from the first matching JWK
				// In a real scenario, you might need more sophisticated key selection logic.
				// The jwk.Key interface has a PublicKey() method to get the public key.
				publicKey, publicKeyErr := key.PublicKey()
				if publicKeyErr != nil {
					logger.Debug(fmt.Sprintf("failed to get public key from JWK: %v", publicKeyErr))
					return nil, fmt.Errorf("failed to get public key from JWK: %v", err)
				}
				return publicKey, nil
			})

			if err != nil {
				// Log the verification error but fall back to UserInfo endpoint
				logger.Error(fmt.Sprintf("failed to parse or verify ID token: %v. Falling back to UserInfo endpoint.", err))
			} else if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				logger.Debug("ID token verified successfully, claims extracted.")

				// --- IMPORTANT OIDC VALIDATION ---
				// In a production application, you MUST perform additional validations on the ID token claims:
				// - Issuer (iss): Must match the expected issuer URL.
				// - Audience (aud): Must contain your client ID.
				// - Expiration Time (exp): Token must not be expired (jwt.Parse handles this if clock skew is accounted for).
				// - Issued At Time (iat): Token must not be issued in the future.
				// - Nonce (nonce): If a nonce was sent in the authorization request, it must match the 'nonce' claim in the ID token.
				// - Authorized Party (azp): If 'aud' has multiple values, 'azp' must be present and equal to your client ID.
				// - at_hash and c_hash: If the access token or authorization code were issued, validate their hashes if present in the ID token.
				// A dedicated OIDC library handles these validations correctly and securely.
				// This simplified example only verifies the signature and basic validity.
				// --- END IMPORTANT OIDC VALIDATION ---

				// Populate the userInfo struct from the ID token claims (map)
				if name, ok := claims["name"].(string); ok {
					userdata.Name = name
				}
				if preferredUsername, ok := claims["preferred_username"].(string); ok {
					userdata.PreferredUsername = preferredUsername
				}
				if email, ok := claims["email"].(string); ok {
					userdata.Email = email
				}
				if sub, ok := claims["sub"].(string); ok {
					userdata.Sub = sub
				}
				if phone, ok := claims["phone_number"].(string); ok {
					userdata.Phone = phone
				}

				useUserInfoEndpoint = false // Successfully processed ID token, no need for UserInfo endpoint

			} else {
				// ID token signature verification failed or token is invalid for other reasons
				logger.Error("id token signature verification failed or token is invalid. Falling back to UserInfo endpoint.")
			}
		}
	}

	// Step 3: Use the access token to fetch user info from the UserInfo URL if needed
	// This step is skipped if the ID token was successfully processed and verified.
	if useUserInfoEndpoint {
		userInfoURL := config.Auth.Methods.OidcAuth.UserInfoUrl
		req, _ = http.NewRequest("GET", userInfoURL, nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

		resp, err = client.Do(req)
		if err != nil {
			return 500, fmt.Errorf("failed to fetch user info: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return 500, fmt.Errorf("failed to fetch user info: received status code %d with body: %s", resp.StatusCode, string(bodyBytes))
		}

		// Decode the UserInfo response directly into the original userdata struct
		if err = json.NewDecoder(resp.Body).Decode(&userdata); err != nil {
			return 500, fmt.Errorf("failed to parse user info: %v", err)
		}
	}

	// Now userdata contains claims from either the verified ID token or the UserInfo endpoint.
	// Proceed with logging in the user based on the extracted claims.
	return loginWithOidcUser(w, r, userdata)
}

// oidcLoginHandler redirects the user to the OIDC provider's authorization endpoint.
// This function remains largely the same, but includes the 'fb_redirect' parameter
// to redirect the user back to the original page after successful login.
func oidcLoginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Auth.Methods.OidcAuth.Enabled {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
		}

		// Generate a nonce for CSRF protection (optional but recommended)
		// You should store this nonce and validate it in the callback handler.
		nonce := utils.InsecureRandomIdentifier(16)
		// For state parameter, you might include the nonce and the original redirect URL
		state := fmt.Sprintf("%s:%s", nonce, r.URL.Query().Get("fb_redirect")) // Example state parameter structure

		redirectURI := fmt.Sprintf("%s/api/auth/oidc/callback", origin)

		// Construct the authorization URL
		authURL := fmt.Sprintf("%s?client_id=%s&response_type=code&scope=%s&redirect_uri=%s&state=%s",
			config.Auth.Methods.OidcAuth.AuthorizationUrl,
			url.QueryEscape(config.Auth.Methods.OidcAuth.ClientID),
			url.QueryEscape(config.Auth.Methods.OidcAuth.Scopes),
			url.QueryEscape(redirectURI),
			url.QueryEscape(state), // Use the state parameter
		)

		// Redirect the user to the OIDC provider
		http.Redirect(w, r, authURL, http.StatusFound)
		return 0, nil // Indicate that the request has been handled by the redirect
	}

	// OIDC authentication is not enabled
	return http.StatusForbidden, fmt.Errorf("oidc authentication is not enabled")
}

// loginWithOidcUser extracts the username from the user claims (userInfo)
// based on the configured UserIdentifier and logs the user into the application.
// It creates a new user if one doesn't exist.
func loginWithOidcUser(w http.ResponseWriter, r *http.Request, userInfo userInfo) (int, error) {
	username := userInfo.PreferredUsername
	switch config.Auth.Methods.OidcAuth.UserIdentifier {
	case "email":
		username = userInfo.Email
	case "username":
		username = userInfo.PreferredUsername
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
	if user.LoginMethod != users.LoginMethodOidc {
		logger.Warning(fmt.Sprintf("user %s is not allowed to login with oidc authentication, bypassing and updating login method", user.Username))
		user.LoginMethod = users.LoginMethodOidc
		go store.Users.Update(user, true, "LoginMethod")
		//return http.StatusForbidden, fmt.Errorf("user %s is not allowed to login with OIDC", username)
	}
	signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), time.Hour*time.Duration(config.Auth.TokenExpirationHours), user.Permissions)
	if err != nil {
		// Handle potential errors during token generation
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, fmt.Errorf("token name conflict: %v", err)
		}
		return http.StatusInternalServerError, fmt.Errorf("failed to generate authentication token for user %s: %v", username, err)
	}

	// Set the authentication token as an HTTP cookie
	cookie := &http.Cookie{
		Name:   "auth",                        // The name of your auth cookie
		Value:  signed.Key,                    // The generated token value
		Domain: strings.Split(r.Host, ":")[0], // Set domain to the host without port
		Path:   "/",                           // Make the cookie available to the whole site
		// Secure: true, // Recommended: Set to true in production with HTTPS
		// HttpOnly: true, // Recommended: Set to true to prevent client-side script access
		Expires: time.Now().Add(time.Hour * time.Duration(config.Auth.TokenExpirationHours)), // Set cookie expiration
	}
	http.SetCookie(w, cookie)

	// Set the authenticated user in the request context or response writer
	// This allows subsequent handlers to access the current user.
	setUserInResponseWriter(w, user)

	// Redirect the user to the page they were trying to access before login,
	// or to the root ("/") if no specific redirect was requested.
	// The 'fb_redirect' parameter is extracted from the 'state' parameter for security.
	state := r.URL.Query().Get("state")
	fbRedirect := "/" // Default redirect path
	if state != "" {
		// Assuming state is in the format "nonce:fb_redirect"
		parts := strings.SplitN(state, ":", 2)
		if len(parts) == 2 {
			// TODO: Validate the nonce part against the stored nonce for CSRF protection
			// For this example, we'll just extract the redirect part
			extractedRedirect := parts[1]
			if extractedRedirect != "" {
				fbRedirect = extractedRedirect
			}
		}
	}

	http.Redirect(w, r, fbRedirect, http.StatusFound)

	// Return 0 to indicate that the response has been handled by the redirect
	return 0, nil
}
