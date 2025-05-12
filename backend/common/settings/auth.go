package settings

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
	Enabled          bool   `json:"enabled"`                              // whether to enable OIDC authentication
	ClientID         string `json:"clientId" validate:"required"`         // client id of the OIDC application
	ClientSecret     string `json:"clientSecret" validate:"required"`     // client secret of the OIDC application
	AuthorizationUrl string `json:"authorizationUrl" validate:"required"` // authorization URL of the OIDC provider
	TokenUrl         string `json:"tokenUrl" validate:"required"`         // token URL of the OIDC provider
	UserInfoUrl      string `json:"userInfoUrl" validate:"required"`      // user info URL of the OIDC provider
	Scopes           string `json:"scopes" validate:"required"`           // space separated list of scopes to request from the OIDC provider
	UserIdentifier   string `json:"userIdentifier"`                       // optional: which attribute should be used as the username? options: email, username, name, phone_number, sub
	JwksUrl          string `json:"jwksUrl"`                              // currently not used by filebrowser
}
