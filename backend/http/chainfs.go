package http

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/chainfs"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
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

	// Resolve the Azure B2C authorize URL.
	// When loginUrl is configured we use it directly (no network call).
	// Otherwise we fall back to fetching it from the ChainFS API.
	var parsedUrl *url.URL
	if chainfsConfig.LoginUrl != "" {
		var parseErr error
		parsedUrl, parseErr = url.Parse(chainfsConfig.LoginUrl)
		if parseErr != nil {
			logger.Errorf("Failed to parse configured loginUrl: %v", parseErr)
			return http.StatusInternalServerError, fmt.Errorf("failed to parse login URL: %w", parseErr)
		}
	} else {
		azureLoginUrl, fetchErr := chainfs.GetLoginUrl(chainfsConfig.ApiBaseUrl)
		if fetchErr != nil {
			logger.Errorf("Failed to fetch login URL from ChainFS: %v", fetchErr)
			return http.StatusInternalServerError, fmt.Errorf("failed to fetch login URL: %w", fetchErr)
		}
		var parseErr error
		parsedUrl, parseErr = url.Parse(azureLoginUrl)
		if parseErr != nil {
			logger.Errorf("Failed to parse Azure login URL: %v", parseErr)
			return http.StatusInternalServerError, fmt.Errorf("failed to parse login URL: %w", parseErr)
		}
	}

	// Get FileBrowser's callback URL — prefer ExternalUrl so it stays correct behind reverse proxies
	var origin string
	if config.Server.ExternalUrl != "" {
		origin = strings.TrimRight(config.Server.ExternalUrl, "/")
	} else {
		origin = r.Header.Get("Origin")
		if origin == "" {
			origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
		}
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

	// Generate PKCE code verifier: 32 random bytes → base64url (no padding) = 43 chars
	verifierBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, verifierBytes); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	// S256 code challenge: BASE64URL(SHA256(verifier))
	challengeHash := sha256.Sum256([]byte(verifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	logger.Infof("ChainFS Login - PKCE verifier_len=%d challenge=%s", len(verifier), codeChallenge)

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

	// Resolve client_id and token endpoint.
	// When loginUrl is configured we derive them from config (no network call).
	// Otherwise we fall back to fetching from the ChainFS API.
	var clientID, tokenEndpoint string
	if chainfsConfig.LoginUrl != "" {
		parsedLoginUrl, parseErr := url.Parse(chainfsConfig.LoginUrl)
		if parseErr != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to parse configured loginUrl: %w", parseErr)
		}
		clientID = parsedLoginUrl.Query().Get("client_id")
		if chainfsConfig.TokenUrl != "" {
			tokenEndpoint = chainfsConfig.TokenUrl
		} else {
			// Derive token URL: strip query params, replace /authorize with /token
			base := parsedLoginUrl.Scheme + "://" + parsedLoginUrl.Host + parsedLoginUrl.Path
			tokenEndpoint = strings.Replace(base, "/authorize", "/token", 1)
		}
	} else {
		azureLoginUrl, fetchErr := chainfs.GetLoginUrl(chainfsConfig.ApiBaseUrl)
		if fetchErr != nil {
			logger.Errorf("Failed to fetch login URL: %v", fetchErr)
			return http.StatusInternalServerError, fetchErr
		}
		parsedUrl, parseErr := url.Parse(azureLoginUrl)
		if parseErr != nil {
			return http.StatusInternalServerError, parseErr
		}
		clientID = parsedUrl.Query().Get("client_id")
		tokenEndpoint = strings.Replace(azureLoginUrl, "/authorize", "/token", 1)
		if idx := strings.Index(tokenEndpoint, "?"); idx != -1 {
			tokenEndpoint = tokenEndpoint[:idx]
		}
	}

	// Build callback URL — prefer ExternalUrl so it stays correct behind reverse proxies
	var callbackOrigin string
	if config.Server.ExternalUrl != "" {
		callbackOrigin = strings.TrimRight(config.Server.ExternalUrl, "/")
	} else {
		callbackOrigin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
	}
	redirectURL := fmt.Sprintf("%s%sapi/auth/chainfs/callback", callbackOrigin, config.Server.BaseURL)
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

	// Extract Azure B2C subject identifier for acorn.tools subscription check
	azureSub, _ := claims["sub"].(string)

	// Login or create user
	return loginWithChainFsUser(w, r, username, displayName, azureSub, isAdmin, rawTokenResponse.AccessToken, rawTokenResponse.RefreshToken, expiresAt, fbRedirect)
}

// loginWithChainFsUser creates or updates a user and logs them in
func loginWithChainFsUser(w http.ResponseWriter, r *http.Request, username, displayName, azureSub string, isAdmin bool, accessToken, refreshToken string, expiresAt int64, redirect string) (int, error) {
	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth

	// Check if user already exists in the DB and is already an admin there.
	// This allows existing admins to log in even when the Azure token lacks the admin role claim.
	existingUser, existingErr := store.Users.Get(username)
	if existingErr == nil && existingUser.Permissions.Admin {
		isAdmin = true
	}

	// Check subscription via acorn.tools billing system.
	var subscribed bool
	if settings.Env.ChainFsBypass {
		subscribed = true
		logger.Infof("Subscription check bypassed for %s via FILEBROWSER_CHAINFS_BYPASS", username)
	} else if settings.Env.AcornToolsSecret != "" {
		access, accessErr := chainfs.CheckAcornToolsAccess(settings.Env.AcornToolsURL, settings.Env.AcornToolsSecret, azureSub)
		if accessErr != nil {
			logger.Errorf("acorn.tools subscription check failed for %s: %v", username, accessErr)
			return http.StatusServiceUnavailable, fmt.Errorf("could not verify subscription status, please try again")
		}
		subscribed = access.HasAccess
		logger.Infof("acorn.tools subscription for %s: plan=%s hasAccess=%v admin=%v", username, access.PlanTier, subscribed, isAdmin)
	} else {
		// Fallback: ChainFS UserInfo (legacy)
		userInfo, userInfoErr := chainfs.GetUserInfo(chainfsConfig.ApiBaseUrl, accessToken)
		if userInfoErr != nil {
			logger.Infof("Could not fetch ChainFS subscription status for %s: %v", username, userInfoErr)
			return http.StatusServiceUnavailable, fmt.Errorf("could not verify subscription status, please try again")
		}
		subscribed = userInfo.IsActive()
		logger.Infof("ChainFS subscription for %s: enhancedSubscription=%v admin=%v", username, userInfo.EnhancedSubscription, isAdmin)
	}
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
		// Restore SAFEMode state from central state file (survives DB wipes)
		if sm := AcornStateGetSafeMode(username); sm != nil && len(sm.Items) > 0 {
			user.SafeModeItems = sm.Items
			user.SafeModePINHash = sm.PINHash
			if err := store.Users.Update(user, true, "SafeModeItems", "SafeModePINHash"); err != nil {
				logger.Errorf("acornstate: failed to restore SafeMode for %s: %v", username, err)
			} else {
				logger.Infof("acornstate: restored %d SAFEMode item(s) for %s", len(sm.Items), username)
			}
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

		updateFields := []string{"AzureAccessToken", "AzureRefreshToken", "AzureTokenExpiry", "LoginMethod", "Permissions", "ChainFSSubscribed", "DisplayName"}
		if correctUserScope(user) {
			updateFields = append(updateFields, "Scopes")
		}
		if err := store.Users.Update(user, true, updateFields...); err != nil {
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

// correctUserScope ensures the user's scope points to their own isolated directory
// (/users/<username>) rather than the root. If a correction is needed it creates the
// directory tree under /srv/users/<username>/ and updates user.Scopes in place.
// Returns true if the scope was changed — the caller must include "Scopes" in the
// subsequent store.Users.Update call so the corrected value is persisted.
func correctUserScope(user *users.User) bool {
	cleanedUsername := users.CleanUsername(user.Username)
	changed := false
	for i, scope := range user.Scopes {
		if filepath.Base(scope.Scope) != cleanedUsername {
			user.Scopes[i].Scope = "/users"
			changed = true
		}
	}
	if changed {
		logger.Infof("correctUserScope: migrating %s scope to /users/%s", user.Username, cleanedUsername)
		if err := files.MakeUserDirs(user, false); err != nil {
			logger.Errorf("correctUserScope: MakeUserDirs failed for %s: %v", user.Username, err)
		}
	}
	return changed
}

// ssoPayload is the structure of the signed token issued by acorn.tools.
type ssoPayload struct {
	Email     string `json:"email"`
	AzureSub  string `json:"azureSub"`
	GivenName string `json:"givenName"`
	Exp       int64  `json:"exp"`
	Jti       string `json:"jti"`
}

// verifySsoToken validates the HMAC-SHA256 signed token from acorn.tools and
// returns the decoded payload, or an error if the signature or expiry is invalid.
func verifySsoToken(token string) (*ssoPayload, error) {
	secret := settings.Env.AcornDriveSsoSecret
	if secret == "" {
		return nil, fmt.Errorf("SSO secret not configured")
	}

	sep := strings.LastIndex(token, ".")
	if sep == -1 {
		return nil, fmt.Errorf("malformed SSO token")
	}
	payloadSegment := token[:sep]
	sigSegment := token[sep+1:]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigSegment)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sigBytes, expected) {
		return nil, fmt.Errorf("SSO token signature mismatch")
	}

	var p ssoPayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return nil, fmt.Errorf("invalid SSO payload: %w", err)
	}
	if p.Email == "" || p.AzureSub == "" || p.Jti == "" {
		return nil, fmt.Errorf("SSO payload missing required fields")
	}
	if p.Exp <= time.Now().UnixMilli() {
		return nil, fmt.Errorf("SSO token expired")
	}
	return &p, nil
}

// ssoJtiStore tracks redeemed SSO token IDs to enforce one-time use.
// Drive runs as a single replica (min/max = 1), so an in-process store is
// authoritative. Entries are swept on each redemption; tokens live 60s so
// anything older than 5 minutes is garbage. A container restart clears the
// store — acceptable because tokens expire 60s after minting regardless.
var ssoJtiStore = struct {
	sync.Mutex
	used map[string]int64 // jti -> token expiry (unix ms)
}{used: make(map[string]int64)}

// markSsoJtiUsed atomically records a jti as redeemed. Returns an error if
// the jti has already been used (replay). Callers must treat any error as a
// rejected login (fail closed).
func markSsoJtiUsed(jti string, exp int64) error {
	ssoJtiStore.Lock()
	defer ssoJtiStore.Unlock()
	if _, exists := ssoJtiStore.used[jti]; exists {
		return fmt.Errorf("SSO token replay detected")
	}
	// Sweep expired entries so the map stays small.
	cutoff := time.Now().Add(-5 * time.Minute).UnixMilli()
	for k, v := range ssoJtiStore.used {
		if v < cutoff {
			delete(ssoJtiStore.used, k)
		}
	}
	ssoJtiStore.used[jti] = exp
	return nil
}

// chainfsSSOHandler handles SSO login initiated by acorn.tools.
// It verifies the signed token, checks the subscription using the correct
// azure_sub identifier, then creates a Drive session without a B2C round-trip.
// @Summary Acorn SSO login
// @Description Accepts a signed SSO token from acorn.tools and logs the user in.
// @Tags Auth
// @Param token query string true "Signed SSO token"
// @Param next  query string false "Redirect path after login"
// @Success 302 {string} string "Redirect to next path"
// @Router /api/auth/sso [get]
func chainfsSSOHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	loginURL := fmt.Sprintf("%slogin?error=sso", config.Server.BaseURL)

	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		http.Redirect(w, r, loginURL, http.StatusFound)
		return 0, nil
	}

	payload, err := verifySsoToken(rawToken)
	if err != nil {
		logger.Warningf("SSO token verification failed: %v", err)
		http.Redirect(w, r, loginURL, http.StatusFound)
		return 0, nil
	}

	// One-time use: record the jti; any error (replay or store failure) rejects.
	if err := markSsoJtiUsed(payload.Jti, payload.Exp); err != nil {
		logger.Warningf("SSO one-time-use check failed for %s: %v", payload.Email, err)
		http.Redirect(w, r, loginURL, http.StatusFound)
		return 0, nil
	}

	// Subscription check — use azureSub (GUID) not email so the landing page
	// endpoint can find the user in its database.
	var subscribed bool
	if settings.Env.ChainFsBypass {
		subscribed = true
		logger.Infof("SSO: subscription check bypassed for %s", payload.Email)
	} else if settings.Env.AcornToolsSecret != "" {
		access, accessErr := chainfs.CheckAcornToolsAccess(settings.Env.AcornToolsURL, settings.Env.AcornToolsSecret, payload.AzureSub)
		if accessErr != nil {
			logger.Errorf("SSO: acorn.tools subscription check failed for %s: %v", payload.Email, accessErr)
			return http.StatusServiceUnavailable, fmt.Errorf("could not verify subscription status, please try again")
		}
		subscribed = access.HasAccess
		logger.Infof("SSO: acorn.tools subscription for %s: plan=%s hasAccess=%v", payload.Email, access.PlanTier, subscribed)
	} else {
		// No acorn.tools secret configured and bypass not set — deny SSO login.
		logger.Errorf("SSO: FILEBROWSER_ACORN_TOOLS_SECRET not set, cannot verify subscription for %s", payload.Email)
		return http.StatusServiceUnavailable, fmt.Errorf("subscription verification not configured")
	}

	if !subscribed {
		http.Redirect(w, r, loginURL, http.StatusFound)
		return 0, nil
	}

	// Determine next path (relative paths only).
	next := r.URL.Query().Get("next")
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	// Drive usernames are the Azure sub GUID (matching what extractUsername returns
	// from the B2C token, which lacks preferred_username/email for this tenant).
	username := payload.AzureSub

	// Check admin status from existing DB record — preserve manually-granted admin rights.
	isAdmin := false
	if existingUser, err := store.Users.Get(username); err == nil && existingUser.Permissions.Admin {
		isAdmin = true
	}

	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth
	user, err := store.Users.Get(username)
	if err != nil {
		// New user — auto-create if allowed.
		if !chainfsConfig.CreateUser {
			logger.Errorf("SSO: user %s does not exist and auto-creation is disabled", username)
			return http.StatusForbidden, fmt.Errorf("user does not exist")
		}
		logger.Infof("SSO: creating new user %s", username)
		newUser := &users.User{
			Username:          username,
			DisplayName:       payload.GivenName,
			LoginMethod:       users.LoginMethodChainFs,
			ChainFSSubscribed: true,
		}
		settings.ApplyUserDefaults(newUser)
		if isAdmin {
			newUser.Permissions.Admin = true
		}
		if createErr := storage.CreateUser(*newUser, newUser.Permissions); createErr != nil {
			logger.Errorf("SSO: failed to create user %s: %v", username, createErr)
			return http.StatusInternalServerError, createErr
		}
		user, err = store.Users.Get(username)
		if err != nil {
			logger.Errorf("SSO: failed to reload created user %s: %v", username, err)
			return http.StatusInternalServerError, err
		}
		// Restore SAFEMode state from central state file (survives DB wipes)
		if sm := AcornStateGetSafeMode(username); sm != nil && len(sm.Items) > 0 {
			user.SafeModeItems = sm.Items
			user.SafeModePINHash = sm.PINHash
			if err := store.Users.Update(user, true, "SafeModeItems", "SafeModePINHash"); err != nil {
				logger.Errorf("acornstate: SSO failed to restore SafeMode for %s: %v", username, err)
			} else {
				logger.Infof("acornstate: SSO restored %d SAFEMode item(s) for %s", len(sm.Items), username)
			}
		}
	} else {
		// Existing user — update subscription flag and display name.
		user.ChainFSSubscribed = true
		user.LoginMethod = users.LoginMethodChainFs
		if payload.GivenName != "" {
			user.DisplayName = payload.GivenName
		}
		if isAdmin {
			user.Permissions.Admin = true
		}
		updateFields := []string{"ChainFSSubscribed", "LoginMethod", "Permissions", "DisplayName"}
		if correctUserScope(user) {
			updateFields = append(updateFields, "Scopes")
		}
		if updateErr := store.Users.Update(user, true, updateFields...); updateErr != nil {
			logger.Errorf("SSO: failed to update user %s: %v", username, updateErr)
			return http.StatusInternalServerError, updateErr
		}
	}

	tokenString, err := generateToken(user)
	if err != nil {
		logger.Errorf("SSO: failed to generate JWT for %s: %v", username, err)
		return http.StatusInternalServerError, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "filebrowser_quantum_jwt",
		Value:    tokenString,
		Path:     config.Server.BaseURL,
		HttpOnly: true,
		Secure:   getScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, next, http.StatusFound)
	return 0, nil
}