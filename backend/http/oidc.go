package http

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/oauth2"
)

// userInfo struct to hold user claims from either UserInfo or ID token
type userInfo struct {
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Sub               string `json:"sub"`
	Phone             string `json:"phone_number"`
}

// oidcCallbackHandler handles the OIDC callback after the user authenticates with the provider.
// It exchanges the authorization code for tokens, attempts to verify the ID token.
// If ID token verification or essential claims extraction fails, it falls back to the UserInfo endpoint.
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
	redirectURL := fmt.Sprintf("%s://%s/api/auth/oidc/callback", getScheme(r), r.Host)

	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		Endpoint:     oidcCfg.Provider.Endpoint(), // Use endpoint from discovered provider
		RedirectURL:  redirectURL,                 // Use the dynamically determined redirect URL
		Scopes:       strings.Split(oidcCfg.Scopes, " "),
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

	// --- Attempt to process ID Token ---
	if ok && rawIDToken != "" {
		logger.Debug("ID token found in token response, attempting verification.")

		// Verify the ID token
		// This uses the verifier initialized with the provider's JWKS endpoint and client ID
		idToken, err := oidcCfg.Verifier.Verify(ctx, rawIDToken)
		if err != nil {
			logger.Warningf("failed to verify ID token: %v. Falling back to UserInfo endpoint.", err)
			// Verification failed, claimsFromIDToken remains false
		} else {
			var claims map[string]interface{}
			// Decode the ID token claims into a map to handle arbitrary structure
			// This is where the JWE unmarshalling error occurs if the token is encrypted
			if err := idToken.Claims(&claims); err != nil {
				logger.Warningf("failed to decode ID token claims: %v. Falling back to UserInfo endpoint.", err)
				// Claims decoding failed, claimsFromIDToken remains false
			} else {
				// Successfully verified and decoded ID token claims
				logger.Debugf("ID Token verified and claims decoded: %+v", claims)

				// Populate userdata from ID token claims
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
				// Decide if we rely on ID token claims or still need UserInfo
				// Even if parsing succeeded, if essential claims are missing, use UserInfo
				switch oidcCfg.UserIdentifier {
				case "email":
					if userdata.Email != "" {
						claimsFromIDToken = true
						loginUsername = userdata.Email
					}
				case "username":
					if userdata.PreferredUsername != "" {
						claimsFromIDToken = true
						loginUsername = userdata.PreferredUsername
					}
				case "phone":
					if userdata.Phone != "" {
						claimsFromIDToken = true
						loginUsername = userdata.Phone
					}
				}
			}
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
		// Decode the UserInfo response directly into the userdata struct
		// The UserInfo endpoint is expected to return standard JSON
		if err := userInfoResp.Claims(&userdata); err != nil {
			logger.Errorf("failed to decode user info from endpoint: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to decode user info from endpoint: %v", err)
		}
		// Decide if we rely on ID token claims or still need UserInfo
		// Even if parsing succeeded, if essential claims are missing, use UserInfo
		switch oidcCfg.UserIdentifier {
		case "email":
			loginUsername = userdata.Email
		case "username":
			loginUsername = userdata.PreferredUsername
		case "phone":
			loginUsername = userdata.Phone
		}
	}
	if loginUsername == "" {
		logger.Error("No valid username found in ID token or UserInfo response.")
		return http.StatusInternalServerError, fmt.Errorf("no valid username found in ID token or UserInfo response from claims")
	}
	// Proceed to log the user in with the OIDC data
	// userdata struct now contains info from either verified ID token or UserInfo endpoint
	return loginWithOidcUser(w, r, loginUsername)
}

// oidcLoginHandler redirects the user to the OIDC provider's authorization endpoint.
// This function remains largely the same, but includes the 'fb_redirect' parameter
// to redirect the user back to the original page after successful login.
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
		RedirectURL:  fmt.Sprintf("%s/api/auth/oidc/callback", origin),
		Scopes:       strings.Split(oidcCfg.Scopes, " "),
	}

	nonce := utils.InsecureRandomIdentifier(16)
	fbRedirect := r.URL.Query().Get("fb_redirect")
	state := fmt.Sprintf("%s:%s", nonce, fbRedirect)

	authURL := oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
	return 0, nil
}

// loginWithOidcUser extracts the username from the user claims (userInfo)
// based on the configured UserIdentifier and logs the user into the application.
// It creates a new user if one doesn't exist.
func loginWithOidcUser(w http.ResponseWriter, r *http.Request, username string) (int, error) {
	logger.Debugf("Successfully authenticated OIDC username: %s", username)
	// Retrieve the user from the store and store it in the context
	user, err := store.Users.Get(username)
	if err != nil {
		if err.Error() != "the resource does not exist" {
			return http.StatusInternalServerError, err
		}

		err = storage.CreateUser(users.User{
			LoginMethod: users.LoginMethodOidc,
			Username:    username,
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
		logger.Warningf("user %s is not allowed to login with oidc authentication, bypassing and updating login method", user.Username)
		user.LoginMethod = users.LoginMethodOidc
		err = store.Users.Update(user, true, "LoginMethod")
		if err != nil {
			logger.Debug(err.Error())
		}
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
