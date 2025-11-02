package http

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/oauth2"
)

// userInfo holds all claims dynamically, plus pre-parsed Groups.
type userInfo struct {
	Claims map[string]interface{}
	Groups []string
}

// userInfoUnmarshaller handles unmarshalling all claims dynamically,
// while optionally parsing a configurable groups claim.
type userInfoUnmarshaller struct {
	userInfo    *userInfo
	groupsClaim string
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (u *userInfoUnmarshaller) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract groups if configured
	if u.groupsClaim != "" {
		if v, ok := raw[u.groupsClaim]; ok {
			switch val := v.(type) {
			case []interface{}:
				groups := make([]string, len(val))
				for i, g := range val {
					if s, ok := g.(string); ok {
						groups[i] = strings.TrimSpace(s)
					}
				}
				u.userInfo.Groups = groups
			case string:
				if val != "" {
					parts := strings.Split(val, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					u.userInfo.Groups = parts
				}
			}
		}
	}

	u.userInfo.Claims = raw
	return nil
}

// oidcLoginHandler initiates OIDC login.
// @Summary OIDC login
// @Description Initiates OIDC login flow.
// @Tags OIDC
// @Accept json
// @Produce json
// @Success 302 {string} string "Redirect to OIDC provider"
// @Router /api/auth/oidc/login [get]
func oidcLoginHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	if !oidcCfg.Enabled {
		return http.StatusForbidden, fmt.Errorf("oidc authentication is not enabled")
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = fmt.Sprintf("%s://%s", getScheme(r), r.Host)
	}
	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		Endpoint:     oidcCfg.Provider.Endpoint(),
		RedirectURL:  fmt.Sprintf("%s%sapi/auth/oidc/callback", origin, config.Server.BaseURL),
		Scopes:       strings.Fields(oidcCfg.Scopes),
	}

	nonce := utils.InsecureRandomIdentifier(16)
	fbRedirect := r.URL.Query().Get("redirect")
	state := fmt.Sprintf("%s:%s", nonce, fbRedirect)
	authURL := oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
	return 0, nil
}

// oidcCallbackHandler handles OIDC callback.
// @Summary OIDC callback
// @Description Handles OIDC login callback.
// @Tags OIDC
// @Accept json
// @Produce json
// @Param code query string false "OIDC code"
// @Param state query string false "OIDC state"
// @Success 200 {object} map[string]string "OIDC callback result"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /api/auth/oidc/callback [get]
func oidcCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	ctx := r.Context()
	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	if oidcCfg.Provider == nil || oidcCfg.Verifier == nil {
		// Ensure Provider and Verifier are initialized on application startup
		// This check is good, keep it.
		logger.Error("OIDC provider or verifier not initialized.")
		return http.StatusInternalServerError, fmt.Errorf("OIDC provider or verifier not initialized")
	}
	// If disableVerifyTLS is true, create a custom HTTP client
	// and set it in the context for the OIDC provider.
	if oidcCfg.DisableVerifyTLS {
		// Create a custom transport with InsecureSkipVerify set to true
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		customClient := &http.Client{
			Transport: transport,
		}
		ctx = oidc.ClientContext(ctx, customClient)
	}
	code := r.URL.Query().Get("code")
	// state := r.URL.Query().Get("state") // You might want to validate the state parameter for CSRF protection

	// The redirect URI MUST match the one registered with the OIDC provider
	// and used in the initial /api/auth/oidc/login handler.
	// Using r.Host here might be tricky if running behind a proxy.
	// Consider using a fixed redirect URL from settings if possible.
	redirectURL := fmt.Sprintf("%s://%s%sapi/auth/oidc/callback", getScheme(r), r.Host, config.Server.BaseURL)

	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		Endpoint:     oidcCfg.Provider.Endpoint(), // Use endpoint from discovered provider
		RedirectURL:  redirectURL,                 // Use the dynamically determined redirect URL
		Scopes:       strings.Fields(oidcCfg.Scopes),
	}

	// Exchange the authorization code for tokens
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		logger.Errorf("failed to exchange token: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to exchange token: %v", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	// accessToken := token.AccessToken // Access token is needed for UserInfo, already in 'token'

	var userdata userInfo      // Declare userdata here to be populated by either source
	claimsFromIDToken := false // Flag to track if we successfully got claims from ID token
	loginUsername := ""        // Variable to hold the login username

	// Create custom unmarshaller for userInfo
	userInfoUnmarshaller := &userInfoUnmarshaller{
		userInfo:    &userdata,
		groupsClaim: oidcCfg.GroupsClaim,
	}

	// --- Attempt to process ID Token ---
	if ok && rawIDToken != "" {
		logger.Debug("ID token found in token response, attempting verification.")

		// Verify the ID token
		// This uses the verifier initialized with the provider's JWKS endpoint and client ID
		idToken, verify_err := oidcCfg.Verifier.Verify(ctx, rawIDToken)
		if verify_err != nil {
			// this might not be necessary for certain providers like authentik
			logger.Debugf("failed to verify ID token: %v. This might be expected, falling back to UserInfo endpoint.", verify_err)
			// Verification failed, claimsFromIDToken remains false
		} else {
			// Decode the ID token claims using custom unmarshaller
			// This is where the JWE unmarshalling error occurs if the token is encrypted
			if err := idToken.Claims(userInfoUnmarshaller); err != nil {
				logger.Warningf("failed to decode ID token claims: %v. Falling back to UserInfo endpoint.", err)
				// Claims decoding failed, claimsFromIDToken remains false
			} else {
				// Successfully verified and decoded ID token claims
				logger.Debugf("ID Token verified and claims decoded: %+v", userdata)

				// Decide if we rely on ID token claims or still need UserInfo
				// Even if parsing succeeded, if essential claims are missing, use UserInfo
				if _, ok := userdata.Claims[oidcCfg.UserIdentifier]; ok {
					claimsFromIDToken = true
				}
			}
			logger.Debugf("Failed to verify ID token: %v", verify_err)
		}

	} else {
		logger.Debug("No ID token found in token response or it was empty. Falling back to UserInfo endpoint.")
		// claimsFromIDToken remains false
	}

	// --- Fallback to UserInfo endpoint if ID token processing did not provide essential claims ---
	if !claimsFromIDToken {
		// Use the access token obtained from the initial exchange
		// oauth2Config.TokenSource creates a token source that uses the provided token.
		userInfoResp, err := oidcCfg.Provider.UserInfo(ctx, oauth2Config.TokenSource(ctx, token))
		if err != nil {
			logger.Errorf("failed to fetch user info from endpoint: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to fetch user info from endpoint: %v", err)
		}
		// Decode the UserInfo response using custom unmarshaller
		// The UserInfo endpoint is expected to return standard JSON
		if err := userInfoResp.Claims(userInfoUnmarshaller); err != nil {
			logger.Errorf("failed to decode user info from endpoint: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to decode user info from endpoint: %v", err)
		}
	}

	// --- Determine login username dynamically ---
	if val, ok := userdata.Claims[oidcCfg.UserIdentifier]; ok {
		if v, ok := val.(string); ok {
			loginUsername = v
		}
	}
	if loginUsername == "" {
		logger.Errorf("No valid username found for identifier '%v' in claims.", oidcCfg.UserIdentifier)
		return http.StatusInternalServerError, fmt.Errorf("no valid username found for identifier '%v'", oidcCfg.UserIdentifier)
	}

	// Proceed to log the user in with the OIDC data
	// userdata struct now contains info from either verified ID token or UserInfo endpoint
	return loginWithOidcUser(w, r, loginUsername, userdata.Groups)
}

// loginWithOidcUser extracts the username from the user claims (userInfo)
// based on the configured UserIdentifier and logs the user into the application.
// It creates a new user if one doesn't exist.
func loginWithOidcUser(w http.ResponseWriter, r *http.Request, username string, groups []string) (int, error) {
	isAdmin := false // Default to non-admin user
	if config.Auth.Methods.OidcAuth.AdminGroup != "" {
		if slices.Contains(groups, config.Auth.Methods.OidcAuth.AdminGroup) {
			isAdmin = true // User is in the admin group, grant admin privileges
			logger.Debugf("User %s is in admin group %s, granting admin privileges.", username, config.Auth.Methods.OidcAuth.AdminGroup)
		}
	}
	logger.Debugf("Successfully authenticated OIDC username: %s isAdmin: %v", username, isAdmin)
	// Retrieve the user from the store and store it in the context
	user, err := store.Users.Get(username)
	if err != nil {
		if err.Error() != "the resource does not exist" {
			return http.StatusInternalServerError, err
		}
		if config.Auth.Methods.OidcAuth.CreateUser {
			if config.Auth.Methods.OidcAuth.AdminGroup == "" {
				isAdmin = config.UserDefaults.Permissions.Admin
			}
			user = &users.User{
				Username:    username,
				LoginMethod: users.LoginMethodOidc,
			}
			settings.ApplyUserDefaults(user)
			if isAdmin {
				user.Permissions.Admin = true
			}
			err = storage.CreateUser(*user)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			user, err = store.Users.Get(username)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		} else {
			return http.StatusForbidden, fmt.Errorf("user %s does not exist and createUser is disabled. Your admin needs to create your user before you can access this application", username)
		}
	} else {
		// update user admin perms
		if isAdmin != user.Permissions.Admin && config.Auth.Methods.OidcAuth.AdminGroup != "" {
			user.Permissions.Admin = isAdmin
			err = store.Users.Update(user, true, "Permissions")
			if err != nil {
				logger.Warningf("failed to update oidc user %s admin permissions: %v", username, err)
			}
		}
	}
	if user.LoginMethod != users.LoginMethodOidc {
		return http.StatusForbidden, errors.ErrWrongLoginMethod
	}

	// Sync OIDC groups with access control system
	if err = store.Access.SyncUserGroups(username, groups); err != nil {
		logger.Warningf("failed to sync oidc user %s groups: %v", username, err)
	}

	expires := time.Hour * time.Duration(config.Auth.TokenExpirationHours)
	// Generate a signed token for the user
	signed, err2 := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), expires, user.Permissions)
	if err2 != nil {
		// Handle potential errors during token generation
		if strings.Contains(err2.Error(), "key already exists with same name") {
			return http.StatusConflict, fmt.Errorf("token name conflict: %v", err2)
		}
		return http.StatusInternalServerError, fmt.Errorf("failed to generate authentication token for user %s: %v", username, err)
	}

	// Add 30 minutes buffer so expired token doesn't get automatically deleted by the browser
	// This allows backend to identify expired sessions and provide better user feedback
	expiresTime := time.Now().Add(expires).Add(time.Minute * 30)

	// Set the authentication token as an HTTP cookie
	// Get the correct domain for cookie - prefer X-Forwarded-Host from reverse proxy
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	cookie := &http.Cookie{
		Name:     "filebrowser_quantum_jwt",
		Value:    signed.Key,
		Domain:   strings.Split(host, ":")[0], // Set domain to the host without port
		Path:     "/",
		SameSite: http.SameSiteLaxMode, // Lax mode allows cookie on navigation from OIDC provider
		Expires:  expiresTime,
		// HttpOnly: true, // Cannot use HttpOnly since frontend needs to read cookie for renew operations
		// Secure: true, // Enable this in production with HTTPS
	}
	http.SetCookie(w, cookie)

	// Set the authenticated user in the request context or response writer
	// This allows subsequent handlers to access the current user.
	setUserInResponseWriter(w, user)

	// Redirect the user to the page they were trying to access before login,
	// or to the root ("/") if no specific redirect was requested.
	// The 'fb_redirect' parameter is extracted from the 'state' parameter for security.
	state := r.URL.Query().Get("state")

	fbRedirect := config.Server.BaseURL // Default redirect to the base URL
	if state != "" {
		parts := strings.SplitN(state, ":", 2)

		// 2. Validate the nonce
		// receivedNonce := parts[0]
		// if receivedNonce != nonceCookie.Value {
		//    // Handle error: nonce mismatch (possible CSRF attack)
		//    return http.StatusBadRequest, fmt.Errorf("invalid state nonce")
		// }

		if len(parts) == 2 && parts[1] != "" {
			// 3. Prevent Open Redirect vulnerability
			// Ensure the redirect is to a local path.
			potentialRedirect, err := url.QueryUnescape(parts[1])
			if err == nil && strings.HasPrefix(potentialRedirect, "/") {
				fbRedirect = potentialRedirect
			} else {
				logger.Warningf("Blocked potentially malicious redirect to: %s", parts[1])
			}
		}
	}

	// Clean up
	http.Redirect(w, r, fbRedirect, http.StatusFound)

	// Return 0 to indicate that the response has been handled by the redirect
	return 0, nil
}
