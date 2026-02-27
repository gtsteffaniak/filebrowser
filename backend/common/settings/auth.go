package settings

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	ldap "github.com/go-ldap/ldap/v3"
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

// AuthCommon contains fields shared across multiple authentication methods
type AuthCommon struct {
	Enabled           bool     `json:"enabled"`           // whether to enable this authentication method.
	AdminGroup        string   `json:"adminGroup"`        // if set, users in this group will be granted admin privileges
	UserGroups        []string `json:"userGroups"`        // if set, only users in these groups are allowed to log in. Blocks all other users even with valid credentials.
	GroupsClaim       string   `json:"groupsClaim"`       // the JSON field name to read groups from. Default is "groups"
	UserIdentifier    string   `json:"userIdentifier"`    // the field value to use as the username. Default is "preferred_username", can also be "email" or "username", or "phone"
	DisableVerifyTLS  bool     `json:"disableVerifyTLS"`  // disable TLS verification (insecure, for testing only)
	LogoutRedirectUrl string   `json:"logoutRedirectUrl"` // if provider logout url is provided, filebrowser will also redirect to logout url. Custom logout query params are respected.
	CreateUser        bool     `json:"createUser"`        // deprecated: always true for supported authentication methods
}

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"` // time in hours each web UI session token is valid for. Default is 2 hours.
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key"`           // secret: the key used to sign the JWT tokens. If not set, a random key will be generated.
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
	LdapAuth     LdapConfig         `json:"ldap" validate:"omitempty"`
	JwtAuth      JwtAuthConfig      `json:"jwt" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled     bool      `json:"enabled"`
	MinLength   int       `json:"minLength" validate:"omitempty"` // minimum pasword length required, default is 5.
	Signup      bool      `json:"signup" validate:"omitempty"`    // allow signups on login page if enabled -- not secure.
	Recaptcha   Recaptcha `json:"recaptcha" validate:"omitempty"` // recaptcha config, only used if signup is enabled
	EnforcedOtp bool      `json:"enforcedOtp"`                    // if set to true, TOTP is enforced for all password users users. Otherwise, users can choose to enable TOTP.
}

type ProxyAuthConfig struct {
	AuthCommon `json:",inline"`
	Header     string `json:"header"` // required header to use for authentication. Security Warning: FileBrowser blindly accepts the header value as username.
}

type Recaptcha struct {
	Host   string `json:"host" validate:"required"`
	Key    string `json:"key" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

// OpenID OAuth2.0
type OidcConfig struct {
	AuthCommon   `json:",inline"`
	ClientID     string                `json:"clientId"`     // secret: client id of the OIDC application
	ClientSecret string                `json:"clientSecret"` // secret: client secret of the OIDC application
	IssuerUrl    string                `json:"issuerUrl"`    // authorization URL of the OIDC provider
	Scopes       string                `json:"scopes"`       // scopes to request from the OIDC provider
	Provider     *oidc.Provider        `json:"-"`            // OIDC provider
	Verifier     *oidc.IDTokenVerifier `json:"-"`            // OIDC verifier
}

type LdapConfig struct {
	AuthCommon   `json:",inline"`
	Server       string `json:"server"`       // scheme://host:port of the LDAP server (e.g. ldap://localhost:389)
	BaseDN       string `json:"baseDN"`       // LDAP search base DN (e.g. dc=ldap,dc=goauthentik,dc=io)
	UserDN       string `json:"userDN"`       // Bind DN for service account (e.g. cn=admin,ou=users,dc=ldap,dc=goauthentik,dc=io)
	UserPassword string `json:"userPassword"` // Bind password for service account
	UserFilter   string `json:"userFilter"`   // Search filter for finding user by username. Default (&(cn=%s)(objectClass=user)); override e.g. (email=%s) or (sAMAccountName=%s) for other directories.
	Port         int    `json:"-"`            // derived from server
	Scheme       string `json:"-"`            // derived from server
	Host         string `json:"-"`            // derived from server
}

// JwtAuthConfig configures external JWT token authentication
// Similar to Grafana's JWT auth: accepts external JWT tokens signed with a shared secret
// The query parameter is hardcoded to "jwt" (e.g. ?jwt=<token>)
type JwtAuthConfig struct {
	AuthCommon `json:",inline"`
	Header     string `json:"header"`    // HTTP header to look for JWT token (e.g. X-JWT-Assertion). Default is "X-JWT-Assertion"
	Secret     string `json:"secret"`    // secret: shared secret key for verifying JWT token signatures (required)
	Algorithm  string `json:"algorithm"` // JWT signing algorithm (HS256, HS384, HS512, RS256, ES256). Default is "HS256"
}

// ValidateLdapAuth checks LDAP config and sets defaults. Call when LDAP is enabled.
func ValidateLdapAuth() error {
	ldapCfg := &Config.Auth.Methods.LdapAuth
	if ldapCfg.Server == "" {
		return fmt.Errorf("LDAP server is required when LDAP is enabled")
	}
	scheme, host, port, err := parseLdapServer(ldapCfg.Server)
	if err != nil {
		return fmt.Errorf("LDAP server is invalid: %w", err)
	}
	ldapCfg.Server = fmt.Sprintf("%s://%s:%d", scheme, host, port)
	ldapCfg.Scheme = scheme
	ldapCfg.Host = host
	ldapCfg.Port = port

	if ldapCfg.BaseDN == "" {
		return fmt.Errorf("LDAP baseDN is required when LDAP is enabled")
	}
	if ldapCfg.UserDN == "" {
		return fmt.Errorf("LDAP userDN (bind DN) is required when LDAP is enabled")
	}
	if ldapCfg.UserFilter == "" {
		ldapCfg.UserFilter = "(&(cn=%s)(objectClass=user))"
	}
	if err := verifyLdapConnection(); err != nil {
		logger.Fatalf("LDAP connection check failed: %v", err)
	}
	return nil
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

func parseLdapServer(server string) (scheme, host string, port int, err error) {
	parts := strings.Split(server, "://")
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid LDAP server: %s", server)
	}
	scheme = strings.ToLower(parts[0])
	if scheme != "ldap" && scheme != "ldaps" {
		return "", "", 0, fmt.Errorf("invalid LDAP scheme: %s", scheme)
	}
	hostParts := strings.Split(parts[1], ":")
	host = strings.TrimSuffix(hostParts[0], "/")
	if len(hostParts) > 1 && hostParts[1] != "" {
		port, err = strconv.Atoi(strings.TrimSuffix(hostParts[1], "/"))
		if err != nil || port == 0 {
			return "", "", 0, fmt.Errorf("invalid LDAP port: %s", hostParts[1])
		}
	} else {
		if scheme == "ldaps" {
			port = 636
		} else {
			port = 389
		}
	}
	return scheme, host, port, nil
}

// VerifyLdapConnection tests that the LDAP server is reachable, can bind (if credentials set), and responds to a search.
func verifyLdapConnection() error {
	if Config.Auth.Methods.LdapAuth.Server == "" {
		return fmt.Errorf("LDAP server is required when LDAP is enabled")
	}
	if Config.Auth.Methods.LdapAuth.BaseDN == "" {
		return fmt.Errorf("LDAP baseDN is required for connection verification")
	}
	var opts []ldap.DialOpt
	scheme := Config.Auth.Methods.LdapAuth.Scheme
	host := Config.Auth.Methods.LdapAuth.Host
	port := Config.Auth.Methods.LdapAuth.Port
	if Config.Auth.Methods.LdapAuth.DisableVerifyTLS && scheme == "ldaps" {
		opts = append(opts, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	}
	fullURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)
	conn, err := ldap.DialURL(fullURL, opts...)
	if err != nil {
		return fmt.Errorf("LDAP connection failed (is the LDAP outpost running at %s? use port 389 for LDAP, 636 for LDAPS; this is not the HTTP port): %w", fullURL, err)
	}
	defer conn.Close()
	if Config.Auth.Methods.LdapAuth.UserDN != "" && Config.Auth.Methods.LdapAuth.UserPassword != "" {
		if err = conn.Bind(Config.Auth.Methods.LdapAuth.UserDN, Config.Auth.Methods.LdapAuth.UserPassword); err != nil {
			return fmt.Errorf("LDAP bind failed (check userDN and userPassword): %w", err)
		}
		defer func() { _ = conn.Unbind() }()
	}
	// Dummy search to verify the server responds to LDAP search (same path that fails at login if misconfigured).
	searchRequest := ldap.NewSearchRequest(
		Config.Auth.Methods.LdapAuth.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=*)",
		[]string{"1.1"}, // no attributes
		nil,
	)
	_, err = conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("LDAP search test failed (server may require StartTLS or bind before search): %w", err)
	}
	return nil
}

// ValidateJwtAuth checks JWT config and sets defaults. Call when JWT auth is enabled.
func ValidateJwtAuth() error {
	jwtCfg := &Config.Auth.Methods.JwtAuth
	if jwtCfg.Secret == "" {
		return fmt.Errorf("JWT secret is required when JWT auth is enabled")
	}
	if jwtCfg.Header == "" {
		jwtCfg.Header = "X-JWT-Assertion"
	}
	if jwtCfg.Algorithm == "" {
		jwtCfg.Algorithm = "HS256"
	}
	if jwtCfg.GroupsClaim == "" {
		jwtCfg.GroupsClaim = "groups"
	}
	if jwtCfg.UserIdentifier == "" {
		jwtCfg.UserIdentifier = "sub"
	}
	// Validate algorithm
	validAlgos := map[string]bool{
		"HS256": true, "HS384": true, "HS512": true,
		"RS256": true, "RS384": true, "RS512": true,
		"ES256": true, "ES384": true, "ES512": true,
	}
	if !validAlgos[jwtCfg.Algorithm] {
		return fmt.Errorf("unsupported JWT algorithm: %s. Supported: HS256, HS384, HS512, RS256, RS384, RS512, ES256, ES384, ES512", jwtCfg.Algorithm)
	}
	return nil
}
