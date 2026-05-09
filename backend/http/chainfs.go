package http

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/chainfs"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/oauth2"
)

// chainfsLoginHandler initiates ChainFS Azure AD B2C login.
// @Summary ChainFS login
// @Description Initiates ChainFS Azure AD B2C login flow.
// @Tags ChainFS
// @Accept json
// @Produce json
// @Success 302 {string} string "Redirect to Azure AD B2C"
// @Router /api/auth/chainfs/login [get]
func chainfsLoginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth
	if !chainfsConfig.Enabled {
		return http.StatusForbidden, fmt.Errorf("ChainFS authentication is not enabled")
	}

	// Get the login URL from ChainFS API
	azureLoginUrl, err := chainfs.GetLoginUrl(chainfsConfig.ApiBaseUrl)
	if err != nil {
		logger.Errorf("Failed to fetch login URL from ChainFS: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch login URL: %w", err)
	}

	// Parse the Azure URL to modify redirect_uri
	parsedUrl, err := url.Parse(azureLoginUrl)
	if err != nil {
		logger.Errorf("Failed to parse Azure login URL: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to parse login URL: %w", err)
	}

	// Get FileBrowser's callback URL
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
	}
	callbackURL := fmt.Sprintf("%s%sapi/auth/chainfs/callback", origin, config.Server.BaseURL)
	// Replace 127.0.0.1 with localhost for Azure B2C compatibility
	callbackURL = strings.Replace(callbackURL, "127.0.0.1", "localhost", 1)

	// Modify the redirect_uri parameter
	query := parsedUrl.Query()
	query.Set("redirect_uri", callbackURL)

	// Change response_type from "token" to "code" for authorization code flow
	query.Set("response_type", "code")

	// Change response_mode from "fragment" to "query" so code is in query string
	query.Set("response_mode", "query")

	// Add offline_access scope for refresh token if not present
	scopeValue := query.Get("scope")
	if !strings.Contains(scopeValue, "offline_access") {
		scopeValue += " offline_access"
		query.Set("scope", scopeValue)
	}

	// Add state parameter for CSRF protection
	nonce := utils.InsecureRandomIdentifier(16)
	fbRedirect := r.URL.Query().Get("redirect")
	state := fmt.Sprintf("%s:%s", nonce, fbRedirect)
	query.Set("state", state)

	// Add PKCE code challenge (avoids need for client_secret)
	verifier := oauth2.GenerateVerifier()
	query.Set("code_challenge", oauth2.S256ChallengeFromVerifier(verifier))
	query.Set("code_challenge_method", "S256")

	// Store nonce and PKCE verifier in short-lived cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "chainfs_state_nonce",
		Value:    nonce,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   getScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "chainfs_pkce_verifier",
		Value:    verifier,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   getScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})

	parsedUrl.RawQuery = query.Encode()

	// Debug: Log the final URL we're redirecting to
	finalUrl := parsedUrl.String()
	logger.Infof("ChainFS Login - Redirecting to: %s", finalUrl)
	logger.Infof("ChainFS Login - response_type: %s, response_mode: %s", query.Get("response_type"), query.Get("response_mode"))

	// Redirect user to Azure AD B2C
	http.Redirect(w, r, finalUrl, http.StatusFound)
	return 0, nil
}

// chainfsCallbackHandler handles Azure AD B2C callback.
// @Summary ChainFS callback
// @Description Handles ChainFS Azure AD B2C login callback.
// @Tags ChainFS
// @Accept json
// @Produce json
// @Param code query string false "Authorization code"
// @Param state query string false "State parameter"
// @Success 200 {object} map[string]string "Callback result"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /api/auth/chainfs/callback [get]
func chainfsCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	ctx := r.Context()
	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	logger.Infof("ChainFS Callback - code: %s, state: %s, URL: %s",
		truncateString(code, 20), truncateString(state, 20), r.URL.String())

	if code == "" {
		// Azure AD B2C might be returning code in URL fragment instead of query string
		// Serve HTML that extracts fragment and reloads with query string
		logger.Info("ChainFS Callback - Serving HTML to extract fragment parameters")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		html := `<!DOCTYPE html>
<html>
<head><title>Processing login...</title></head>
<body>
<p>Processing login, please wait...</p>
<script>
// Extract parameters from URL fragment
const hash = window.location.hash.substring(1);
if (hash) {
	// Convert fragment to query string and reload
	const newUrl = window.location.pathname + '?' + hash;
	window.location.replace(newUrl);
} else {
	document.body.innerHTML = '<p>Error: Missing authorization code</p>';
}
</script>
</body>
</html>`
		w.Write([]byte(html))
		return 0, nil
	}

	// Validate state nonce to prevent CSRF attacks
	if state == "" {
		return http.StatusBadRequest, fmt.Errorf("missing state parameter")
	}
	nonceCookie, nonceCookieErr := r.Cookie("chainfs_state_nonce")
	if nonceCookieErr != nil {
		return http.StatusBadRequest, fmt.Errorf("missing state nonce cookie — possible CSRF attack")
	}
	stateParts := strings.SplitN(state, ":", 2)
	if stateParts[0] != nonceCookie.Value {
		return http.StatusBadRequest, fmt.Errorf("state nonce mismatch — possible CSRF attack")
	}
	// Clear the nonce cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "chainfs_state_nonce",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Parse state to extract redirect path
	var fbRedirect string
	if len(stateParts) == 2 {
		fbRedirect = stateParts[1]
	}

	// Get the Azure login URL to extract OAuth2 endpoints
	azureLoginUrl, err := chainfs.GetLoginUrl(chainfsConfig.ApiBaseUrl)
	if err != nil {
		logger.Errorf("Failed to fetch login URL: %v", err)
		return http.StatusInternalServerError, err
	}

	parsedUrl, err := url.Parse(azureLoginUrl)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Extract client_id and construct token endpoint
	query := parsedUrl.Query()
	clientID := query.Get("client_id")

	// Construct token endpoint from authorization endpoint
	// Azure AD B2C pattern: replace /authorize with /token
	tokenEndpoint := strings.Replace(azureLoginUrl, "/authorize", "/token", 1)
	// Remove query parameters
	if idx := strings.Index(tokenEndpoint, "?"); idx != -1 {
		tokenEndpoint = tokenEndpoint[:idx]
	}

	// Build callback URL
	redirectURL := fmt.Sprintf("%s://%s%sapi/auth/chainfs/callback", getScheme(r), r.Host, config.Server.BaseURL)
	// Replace 127.0.0.1 with localhost for Azure B2C compatibility
	redirectURL = strings.Replace(redirectURL, "127.0.0.1", "localhost", 1)

	// Read PKCE verifier cookie
	verifier := ""
	if verifierCookie, err := r.Cookie("chainfs_pkce_verifier"); err == nil {
		verifier = verifierCookie.Value
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "chainfs_pkce_verifier",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Raw HTTP POST token exchange — bypass golang.org/x/oauth2 entirely so no
	// hidden Authorization header or client_secret field can be injected.
	exchangeBody := url.Values{}
	exchangeBody.Set("grant_type", "authorization_code")
	exchangeBody.Set("code", code)
	exchangeBody.Set("client_id", clientID)
	exchangeBody.Set("redirect_uri", redirectURL)
	if verifier != "" {
		exchangeBody.Set("code_verifier", verifier)
	}
	// Include client_secret only if explicitly configured (confidential client fallback).
	if chainfsConfig.ClientSecret != "" {
		exchangeBody.Set("client_secret", chainfsConfig.ClientSecret)
	}

	logger.Infof("Token exchange: endpoint=%s client_id=%s has_verifier=%v has_secret=%v",
		tokenEndpoint, clientID, verifier != "", chainfsConfig.ClientSecret != "")

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint,
		strings.NewReader(exchangeBody.Encode()))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to build token request: %w", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := (&http.Client{Timeout: 30 * time.Second}).Do(tokenReq)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("token request failed: %w", err)
	}
	defer tokenResp.Body.Close()

	tokenRespBytes, _ := io.ReadAll(tokenResp.Body)
	if tokenResp.StatusCode != http.StatusOK {
		logger.Errorf("Token exchange failed: status=%d body=%s", tokenResp.StatusCode, string(tokenRespBytes))
		return http.StatusInternalServerError, fmt.Errorf("failed to exchange code: B2C returned %d: %s",
			tokenResp.StatusCode, string(tokenRespBytes))
	}

	var rawTokenResponse struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(tokenRespBytes, &rawTokenResponse); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to parse token response: %w", err)
	}

	rawIDToken := rawTokenResponse.IDToken
	if rawIDToken == "" {
		logger.Errorf("No ID token in response: %s", string(tokenRespBytes))
		return http.StatusInternalServerError, fmt.Errorf("no ID token received")
	}

	// Verify and parse the ID token.
	// When IssuerUrl is configured, the signature is verified against Azure's public JWKS.
	// Without it, claims are extracted unverified (less secure — set issuerUrl in config).
	claims, err := parseAndVerifyIDToken(ctx, rawIDToken, clientID, chainfsConfig.IssuerUrl)
	if err != nil {
		logger.Errorf("Failed to parse/verify ID token: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to parse ID token: %w", err)
	}

	// Extract username
	username := extractUsername(claims)
	if username == "" {
		logger.Error("No valid username found in ID token claims")
		return http.StatusInternalServerError, fmt.Errorf("no valid username found")
	}

	// Extract groups/roles for admin check
	groups := extractGroups(claims, chainfsConfig.AdminClaim)

	// Check if user should be admin
	isAdmin := false
	if chainfsConfig.AdminClaim != "" && chainfsConfig.AdminClaimValue != "" {
		if slices.Contains(groups, chainfsConfig.AdminClaimValue) {
			isAdmin = true
			logger.Debugf("User %s has admin claim, granting admin privileges", username)
		}
	}

	// Calculate token expiry
	var expiresAt int64
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = int64(exp)
	} else {
		expiresAt = time.Now().Add(time.Hour).Unix()
	}

	// Extract display name (real first name) from JWT claims
	displayName := extractDisplayName(claims)

	// Login or create user
	return loginWithChainFsUser(w, r, username, displayName, isAdmin, rawTokenResponse.AccessToken, rawTokenResponse.RefreshToken, expiresAt, fbRedirect)
}

// loginWithChainFsUser creates or updates a user and logs them in
func loginWithChainFsUser(w http.ResponseWriter, r *http.Request, username, displayName string, isAdmin bool, accessToken, refreshToken string, expiresAt int64, redirect string) (int, error) {
	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth

	// Check if user already exists in the DB and is already an admin there.
	// This allows existing admins to log in even when the Azure token lacks the admin role claim.
	existingUser, existingErr := store.Users.Get(username)
	if existingErr == nil && existingUser.Permissions.Admin {
		isAdmin = true
	}

	// Check subscription — must have Enhanced or Enterprise tier to access acorndrive
	userInfo, err := chainfs.GetUserInfo(chainfsConfig.ApiBaseUrl, accessToken)
	if err != nil {
		logger.Infof("Could not fetch ChainFS subscription status for %s: %v", username, err)
		return http.StatusServiceUnavailable, fmt.Errorf("could not verify subscription status, please try again")
	}
	subscribed := userInfo.IsActive() || settings.Env.ChainFsBypass
	logger.Infof("ChainFS subscription for %s: enhancedSubscription=%v admin=%v bypass=%v — access=%v",
		username, userInfo.EnhancedSubscription, isAdmin, settings.Env.ChainFsBypass, subscribed || isAdmin)
	if !subscribed && !isAdmin {
		loginURL := fmt.Sprintf("%slogin?error=subscription", config.Server.BaseURL)
		http.Redirect(w, r, loginURL, http.StatusFound)
		return 0, nil
	}

	// Get or create user
	user, err := store.Users.Get(username)
	if err != nil {
		// User doesn't exist
		if !chainfsConfig.CreateUser {
			logger.Errorf("User %s does not exist and auto-creation is disabled", username)
			return http.StatusForbidden, fmt.Errorf("user does not exist")
		}

		// Create new user
		logger.Infof("Creating new ChainFS user: %s", username)
		user = &users.User{
			Username:    username,
			DisplayName: displayName,
			LoginMethod: users.LoginMethodChainFs,
		}
		settings.ApplyUserDefaults(user)

		if isAdmin {
			user.Permissions.Admin = true
		}

		// Encrypt and store Azure tokens
		encryptedAccess, err := encryptToken(accessToken)
		if err != nil {
			logger.Errorf("Failed to encrypt access token: %v", err)
			return http.StatusInternalServerError, err
		}

		encryptedRefresh, err := encryptToken(refreshToken)
		if err != nil {
			logger.Errorf("Failed to encrypt refresh token: %v", err)
			return http.StatusInternalServerError, err
		}

		user.AzureAccessToken = encryptedAccess
		user.AzureRefreshToken = encryptedRefresh
		user.AzureTokenExpiry = expiresAt
		user.ChainFSSubscribed = subscribed

		err = storage.CreateUser(*user, user.Permissions)
		if err != nil {
			logger.Errorf("Failed to create user: %v", err)
			return http.StatusInternalServerError, err
		}

		// Reload user from database to get auto-generated ID
		user, err = store.Users.Get(username)
		if err != nil {
			logger.Errorf("Failed to reload created user: %v", err)
			return http.StatusInternalServerError, err
		}
	} else {
		// User exists, update tokens and admin status
		logger.Infof("Updating existing ChainFS user: %s", username)

		encryptedAccess, err := encryptToken(accessToken)
		if err != nil {
			logger.Errorf("Failed to encrypt access token: %v", err)
			return http.StatusInternalServerError, err
		}

		encryptedRefresh, err := encryptToken(refreshToken)
		if err != nil {
			logger.Errorf("Failed to encrypt refresh token: %v", err)
			return http.StatusInternalServerError, err
		}

		user.AzureAccessToken = encryptedAccess
		user.AzureRefreshToken = encryptedRefresh
		user.AzureTokenExpiry = expiresAt
		user.LoginMethod = users.LoginMethodChainFs
		user.ChainFSSubscribed = subscribed
		if displayName != "" {
			user.DisplayName = displayName
		}

		if isAdmin {
			user.Permissions.Admin = true
		}

		if err := store.Users.Update(user, true, "AzureAccessToken", "AzureRefreshToken", "AzureTokenExpiry", "LoginMethod", "Permissions", "ChainFSSubscribed", "DisplayName"); err != nil {
			logger.Errorf("Failed to update user: %v", err)
			return http.StatusInternalServerError, err
		}
	}

	// Generate FileBrowser JWT token
	tokenString, err := generateToken(user)
	if err != nil {
		logger.Errorf("Failed to generate JWT: %v", err)
		return http.StatusInternalServerError, err
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "filebrowser_quantum_jwt",
		Value:    tokenString,
		Path:     config.Server.BaseURL,
		HttpOnly: true,
		Secure:   getScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to original destination or default
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
	return 0, nil
}

// parseAndVerifyIDToken verifies the Azure AD B2C ID token signature when issuerUrl
// is configured, then returns the claims. Falls back to unverified parsing when
// issuerUrl is empty (logs a security warning).
func parseAndVerifyIDToken(ctx context.Context, rawIDToken, clientID, issuerUrl string) (map[string]interface{}, error) {
	if issuerUrl != "" {
		provider, err := gooidc.NewProvider(ctx, issuerUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to initialise OIDC provider for token verification: %w", err)
		}
		verifier := provider.Verifier(&gooidc.Config{ClientID: clientID})
		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			return nil, fmt.Errorf("ID token signature verification failed: %w", err)
		}
		var claims map[string]interface{}
		if err := idToken.Claims(&claims); err != nil {
			return nil, fmt.Errorf("failed to extract verified claims: %w", err)
		}
		return claims, nil
	}

	// IssuerUrl not configured — parse without signature verification.
	// Security warning: configure issuerUrl in chainfs settings to enable verification.
	logger.Warning("ChainFS: IssuerUrl is not set — ID token signature is NOT verified. Set issuerUrl in chainfs config to harden this deployment.")
	return parseJWTClaims(rawIDToken)
}

// parseJWTClaims parses JWT claims without verification (fallback only).
func parseJWTClaims(tokenString string) (map[string]interface{}, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid claims type")
}

// extractDisplayName extracts the user's real first name from JWT claims.
// It tries given_name, then name, then falls back to parsing the email local-part.
func extractDisplayName(claims map[string]interface{}) string {
	for _, key := range []string{"given_name", "name"} {
		if val, ok := claims[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				// Return only the first word in case it's "John Doe"
				parts := strings.Fields(s)
				if len(parts) > 0 {
					return parts[0]
				}
			}
		}
	}
	return ""
}

// extractUsername extracts username from JWT claims
func extractUsername(claims map[string]interface{}) string {
	// Try preferred_username first
	if val, ok := claims["preferred_username"]; ok {
		if username, ok := val.(string); ok && username != "" {
			return username
		}
	}

	// Try email
	if val, ok := claims["email"]; ok {
		if email, ok := val.(string); ok && email != "" {
			return email
		}
	}

	// Fall back to sub
	if val, ok := claims["sub"]; ok {
		if sub, ok := val.(string); ok && sub != "" {
			return sub
		}
	}

	return ""
}

// extractGroups extracts groups/roles from claims
func extractGroups(claims map[string]interface{}, claimName string) []string {
	if claimName == "" {
		claimName = "roles" // default
	}

	if val, ok := claims[claimName]; ok {
		switch v := val.(type) {
		case []interface{}:
			groups := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					groups = append(groups, str)
				}
			}
			return groups
		case string:
			if v != "" {
				return strings.Split(v, ",")
			}
		}
	}

	return []string{}
}

// deriveEncryptionKey derives a 32-byte AES key from the configured auth key using SHA-256.
func deriveEncryptionKey() []byte {
	h := sha256.Sum256([]byte(settings.Config.Auth.Key))
	return h[:]
}

// encryptToken encrypts a token using AES-GCM
func encryptToken(token string) (string, error) {
	key := deriveEncryptionKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptToken decrypts a token using AES-GCM
func decryptToken(encryptedToken string) (string, error) {
	key := deriveEncryptionKey()

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return string(plaintext), nil
}

// generateToken generates a FileBrowser JWT token
func generateToken(user *users.User) (string, error) {
	claims := &users.AuthToken{
		MinimalAuthToken: users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(settings.Config.Auth.TokenExpirationHours) * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		},
		BelongsTo:   user.ID,
		Permissions: user.Permissions,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(settings.Config.Auth.Key))
}

// truncateString truncates a string to maxLen characters for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if s == "" {
		return "(empty)"
	}
	return s[:maxLen] + "..."
}
