package settings

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/go-logger/logger"
)

// userAgentTransport wraps an http.RoundTripper to add a User-Agent header
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

// RoundTrip implements the http.RoundTripper interface
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	reqCopy := req.Clone(req.Context())
	// Set User-Agent header if not already set
	if reqCopy.Header.Get("User-Agent") == "" {
		reqCopy.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(reqCopy)
}

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"` // time in hours each web UI session token is valid for. Default is 2 hours.
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key" yaml:"key"`           // secret: the key used to sign the JWT tokens. If not set, a random key will be generated.
	AdminUsername        string       `json:"adminUsername"` // secret: the username of the admin user. If not set, the default is "admin".
	AdminPassword        string       `json:"adminPassword"` // secret: the password of the admin user. If not set, the default is "admin".
	TotpSecret           string       `json:"totpSecret"`    // secret: secret used to encrypt TOTP secrets
	AuthMethods          []string     `json:"-"`
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy" validate:"omitempty"`
	NoAuth       bool               `json:"noauth"` // if set to true, overrides all other auth methods and disables authentication
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
	OidcAuth     OidcConfig         `json:"oidc" validate:"omitempty"`
	ChainFsAuth  ChainFsConfig      `json:"chainfs" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled     bool      `json:"enabled"`
	MinLength   int       `json:"minLength" validate:"omitempty"` // minimum pasword length required, default is 5.
	Signup      bool      `json:"signup" validate:"omitempty"`    // allow signups on login page if enabled -- not secure.
	Recaptcha   Recaptcha `json:"recaptcha" validate:"omitempty"` // recaptcha config, only used if signup is enabled
	EnforcedOtp bool      `json:"enforcedOtp"`                    // if set to true, TOTP is enforced for all password users users. Otherwise, users can choose to enable TOTP.
}

type ProxyAuthConfig struct {
	Enabled           bool   `json:"enabled"`
	CreateUser        bool   `json:"createUser"`        // create user if not exists
	Header            string `json:"header"`            // required header to use for authentication. Security Warning: FileBrowser blindly accepts the header value as username.
	LogoutRedirectUrl string `json:"logoutRedirectUrl"` // if provider logout url is provided, filebrowser will also redirect to logout url. Custom logout query params are respected.
}

type Recaptcha struct {
	Host   string `json:"host" validate:"required"`
	Key    string `json:"key" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

// OpenID OAuth2.0
type OidcConfig struct {
	Enabled           bool                  `json:"enabled"`           // whether to enable OIDC authentication
	ClientID          string                `json:"clientId"`          // secret: client id of the OIDC application
	ClientSecret      string                `json:"clientSecret"`      // secret: client secret of the OIDC application
	IssuerUrl         string                `json:"issuerUrl"`         // authorization URL of the OIDC provider
	Scopes            string                `json:"scopes"`            // scopes to request from the OIDC provider
	UserIdentifier    string                `json:"userIdentifier"`    // the field value to use as the username. Default is "preferred_username", can also be "email" or "username", or "phone"
	DisableVerifyTLS  bool                  `json:"disableVerifyTLS"`  // disable TLS verification for the OIDC provider. This is insecure and should only be used for testing.
	LogoutRedirectUrl string                `json:"logoutRedirectUrl"` // if provider logout url is provided, filebrowser will also redirect to logout url. Custom logout query params are respected.
	CreateUser        bool                  `json:"createUser"`        // create user if not exists
	AdminGroup        string                `json:"adminGroup"`        // if set, users in this group will be granted admin privileges.
	GroupsClaim       string                `json:"groupsClaim"`       // the JSON field name to read groups from. Default is "groups"
	Provider          *oidc.Provider        `json:"-"`                 // OIDC provider
	Verifier          *oidc.IDTokenVerifier `json:"-"`                 // OIDC verifier
}

// ChainFS Azure AD B2C Authentication
type ChainFsConfig struct {
	Enabled          bool   `json:"enabled"`          // whether to enable ChainFS authentication
	ApiBaseUrl       string `json:"apiBaseUrl"`       // ChainFS API base URL (DEV/UAT/PROD)
	LoginUrl         string `json:"loginUrl"`         // Azure B2C authorize endpoint (with client_id+scope params). When set, bypasses the ChainFS API call on every login.
	TokenUrl         string `json:"tokenUrl"`         // Azure B2C token endpoint. Used with LoginUrl to avoid the ChainFS API call on every callback.
	ClientSecret     string `json:"clientSecret"`     // Azure AD B2C client secret
	CreateUser       bool   `json:"createUser"`       // create user if not exists
	AdminClaim       string `json:"adminClaim"`       // claim to check for admin status (e.g., "roles" or "groups")
	AdminClaimValue  string `json:"adminClaimValue"`  // value that grants admin privileges (e.g., "admin")
	IssuerUrl        string `json:"issuerUrl"`        // Azure AD B2C issuer URL for ID token signature verification (recommended). Format: https://<tenant>.b2clogin.com/<tenant-id>/v2.0/
	AcornToolsUrl    string `json:"acornToolsUrl"`    // acorn.tools base URL for subscription checks (default: https://www.acorn.tools)
	AcornToolsSecret string `json:"acornToolsSecret"` // secret: API key for acorn.tools internal endpoints
}

// ValidateOidcAuth processes the OIDC callback and retrieves user identity
func validateOidcAuth() error {
	oidcCfg := &Config.Auth.Methods.OidcAuth // Use a pointer to modify the original config
	if oidcCfg.GroupsClaim == "" {
		oidcCfg.GroupsClaim = "groups"
	}
	if oidcCfg.UserIdentifier == "" {
		oidcCfg.UserIdentifier = "preferred_username"
	}
	if oidcCfg.Scopes == "" {
		oidcCfg.Scopes = "openid email profile"
	}

	if !oidcCfg.Enabled {
		return errors.New("OIDC is not enabled")
	}
	ctx := context.Background()

	// Create a custom HTTP client with proper User-Agent to avoid being blocked by bot protection
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // Respect HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
	}

	// If disableVerifyTLS is true, disable TLS verification
	if oidcCfg.DisableVerifyTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		logger.Warning("OIDC TLS verification is disabled.")
	}

	// Create custom client with User-Agent header to bypass bot protection (like Cloudflare Bot Fight Mode)
	customClient := &http.Client{
		Transport: &userAgentTransport{
			base:      transport,
			userAgent: fmt.Sprintf("FileBrowser Quantum - %s (OIDC Client)", version.Version),
		},
	}

	// Use oidc.ClientContext to pass the custom client to the OIDC library
	ctx = oidc.ClientContext(ctx, customClient)

	provider, err := oidc.NewProvider(ctx, oidcCfg.IssuerUrl)
	if err != nil {
		return fmt.Errorf("url '%v' failed to create OIDC provider: %w", oidcCfg.IssuerUrl, err)
	}
	oidcCfg.Provider = provider
	oidcCfg.Verifier = provider.Verifier(&oidc.Config{ClientID: oidcCfg.ClientID})

	return nil
}
