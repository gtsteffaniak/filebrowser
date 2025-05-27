package settings

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/go-logger/logger"
)

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"` // the number of hours until the token expires. Default is 2 hours.
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key"`               // the key used to sign the JWT tokens. If not set, a random key will be generated.
	AdminUsername        string       `json:"adminUsername"`     // the username of the admin user. If not set, the default is "admin".
	AdminPassword        string       `json:"adminPassword"`     // the password of the admin user. If not set, the default is "admin".
	ResetAdminOnStart    bool         `json:"resetAdminOnStart"` // if set to true, the admin user will be reset to the default username and password on startup.
	TotpSecret           string       `json:"totpSecret"`        // secret used to encrypt TOTP secrets
	AuthMethods          []string     `json:"-"`
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy" validate:"omitempty"`
	NoAuth       bool               `json:"noauth"` // if set to true, overrides all other auth methods and disables authentication
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
	OidcAuth     OidcConfig         `json:"oidc" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled      bool      `json:"enabled"`
	MinLength    int       `json:"minLength" validate:"omitempty,min=5"` // minimum pasword length required.
	Signup       bool      `json:"signup" validate:"omitempty"`          // allow signups on login page if enabled -- not secure.
	Recaptcha    Recaptcha `json:"recaptcha" validate:"omitempty"`       // recaptcha config, only used if signup is enabled
	TotpEnforced bool      `json:"totpEnforced"`                         // if set to true, TOTP is enforced for all password users users. Otherwise, users can choose to enable TOTP.
}

type ProxyAuthConfig struct {
	Enabled    bool   `json:"enabled"`
	CreateUser bool   `json:"createUser"` // create user if not exists
	Header     string `json:"header"`     // required header to use for authentication. Security Warning: FileBrowser blindly accepts the header value as username.
}

type Recaptcha struct {
	Host   string `json:"host" validate:"required"`
	Key    string `json:"key" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

// OpenID OAuth2.0
type OidcConfig struct {
	Enabled           bool                  `json:"enabled"`           // whether to enable OIDC authentication
	ClientID          string                `json:"clientId"`          // client id of the OIDC application
	ClientSecret      string                `json:"clientSecret"`      // client secret of the OIDC application
	IssuerUrl         string                `json:"issuerUrl"`         // authorization URL of the OIDC provider
	Scopes            string                `json:"scopes"`            // scopes to request from the OIDC provider
	UserIdentifier    string                `json:"userIdentifier"`    // the user identifier to use for authentication. Default is "username", can be "email" or "username", or "phone"
	DisableVerifyTLS  bool                  `json:"disableVerifyTLS"`  // disable TLS verification for the OIDC provider. This is insecure and should only be used for testing.
	LogoutRedirectUrl string                `json:"logoutRedirectUrl"` // if provider logout url is provided, filebrowser will also redirect to logout url. Custom logout query params are respected.
	Provider          *oidc.Provider        `json:"-"`                 // OIDC provider
	Verifier          *oidc.IDTokenVerifier `json:"-"`                 // OIDC verifier
}

// ValidateOidcAuth processes the OIDC callback and retrieves user identity
func validateOidcAuth() error {
	oidcCfg := &Config.Auth.Methods.OidcAuth // Use a pointer to modify the original config
	if !oidcCfg.Enabled {
		return errors.New("OIDC is not enabled")
	}
	if oidcCfg.UserIdentifier == "" {
		oidcCfg.UserIdentifier = "username"
	}

	ctx := context.Background()

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
		// Use oidc.ClientContext to pass the custom client to the OIDC library
		ctx = oidc.ClientContext(ctx, customClient)
		// Log that TLS verification is disabled (important for security awareness)
		logger.Warning("OIDC TLS verification is disabled.")
	}

	provider, err := oidc.NewProvider(ctx, oidcCfg.IssuerUrl)
	if err != nil {
		return fmt.Errorf("url '%v' failed to create OIDC provider: %w", oidcCfg.IssuerUrl, err)
	}
	oidcCfg.Provider = provider
	oidcCfg.Verifier = provider.Verifier(&oidc.Config{ClientID: oidcCfg.ClientID})
	if oidcCfg.Scopes == "" {
		oidcCfg.Scopes = "openid email profile"
	}

	return nil
}
