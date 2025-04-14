package settings

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"`
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key"`
	AdminUsername        string       `json:"adminUsername"`
	AdminPassword        string       `json:"adminPassword"`
	AuthMethods          []string     `json:"-"`
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy" validate:"omitempty"`
	NoAuth       bool               `json:"noauth"`
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
	OidcAuth     OidcConfig         `json:"oidc" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled   bool      `json:"enabled"`
	MinLength int       `json:"minLength" validate:"omitempty,min=5"`
	Signup    bool      `json:"signup" validate:"omitempty"`
	Recaptcha Recaptcha `json:"recaptcha" validate:"omitempty"`
}

type ProxyAuthConfig struct {
	Enabled    bool   `json:"enabled"`
	CreateUser bool   `json:"createUser"`
	Header     string `json:"header"`
}

type Recaptcha struct {
	Host   string `json:"host" validate:"required"`
	Key    string `json:"key" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

// OpenID OAuth2.0
type OidcConfig struct {
	Enabled          bool   `json:"enabled"` // whether to enable OIDC authentication
	ClientID         string `json:"clientId" validate:"required"`
	ClientSecret     string `json:"clientSecret" validate:"required"`
	AuthorizationUrl string `json:"authorizationUrl" validate:"required"`
	TokenUrl         string `json:"tokenUrl" validate:"required"`
	UserInfoUrl      string `json:"userInfoUrl" validate:"required"`
	Scopes           string `json:"scopes" validate:"required"` // space separated list of scopes
	UserIdentifier   string `json:"userIdentifier"`             // which attribute should be used as the username?
	JwksUrl          string `json:"jwksUrl"`                    // currently not used by filebrowser
}
