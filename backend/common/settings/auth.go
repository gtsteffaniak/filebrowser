package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"` // the number of hours until the token expires. Default is 2 hours.
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key"`               // the key used to sign the JWT tokens. If not set, a random key will be generated.
	AdminUsername        string       `json:"adminUsername"`     // the username of the admin user. If not set, the default is "admin".
	AdminPassword        string       `json:"adminPassword"`     // the password of the admin user. If not set, the default is "admin".
	ResetAdminOnStart    bool         `json:"resetAdminOnStart"` // if set to true, the admin user will be reset to the default username and password on startup.
	AuthMethods          []string     `json:"-"`
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy" validate:"omitempty"`
	NoAuth       bool               `json:"noauth"` // if set to true, overrides all other auth methods and disables authentication
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
	OidcAuth     OidcConfig         `json:"oidc" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled   bool      `json:"enabled"`
	MinLength int       `json:"minLength" validate:"omitempty,min=5"`
	Signup    bool      `json:"signup" validate:"omitempty"`    // currently not used by filebrowser
	Recaptcha Recaptcha `json:"recaptcha" validate:"omitempty"` // recaptcha config, only used if signup is enabled
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
	Enabled        bool                  `json:"enabled"`        // whether to enable OIDC authentication
	ClientID       string                `json:"clientId"`       // client id of the OIDC application
	ClientSecret   string                `json:"clientSecret"`   // client secret of the OIDC application
	IssuerUrl      string                `json:"issuerUrl"`      // authorization URL of the OIDC provider
	Scopes         string                `json:"scopes"`         // scopes to request from the OIDC provider
	UserIdentifier string                `json:"userIdentifier"` // the user identifier to use for authentication. Default is "username".
	Provider       *oidc.Provider        `json:"-"`              // OIDC provider
	Verifier       *oidc.IDTokenVerifier `json:"-"`              // OIDC verifier
}

// ValidateOidcAuth processes the OIDC callback and retrieves user identity
func validateOidcAuth() error {
	oidcCfg := Config.Auth.Methods.OidcAuth
	if !oidcCfg.Enabled {
		return errors.New("OIDC is not enabled")
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, oidcCfg.IssuerUrl)
	if err != nil {
		return fmt.Errorf("url '%v' failed to create OIDC provider: %w", oidcCfg.IssuerUrl, err)
	}
	Config.Auth.Methods.OidcAuth.Provider = provider
	Config.Auth.Methods.OidcAuth.Verifier = provider.Verifier(&oidc.Config{ClientID: oidcCfg.ClientID})
	if oidcCfg.Scopes == "" {
		Config.Auth.Methods.OidcAuth.Scopes = "openid email profile"
	}

	return nil
}
