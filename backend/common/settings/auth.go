package settings

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"`
	Recaptcha            Recaptcha    `json:"recaptcha"`
	Methods              LoginMethods `json:"methods"`
	Signup               bool         `json:"signup"`
	Method               string       `json:"method"`
	Key                  []byte       `json:"key"`
	AdminUsername        string       `json:"adminUsername"`
	AdminPassword        string       `json:"adminPassword"`
	AuthMethods          []string
}

type LoginMethods struct {
	ProxyAuth    ProxyAuthConfig    `json:"proxy"`
	NoAuth       bool               `json:"noauth"`
	PasswordAuth PasswordAuthConfig `json:"password"`
	OidcAuth     OidcConfig         `json:"oidc"`
}

type PasswordAuthConfig struct {
	Enabled   bool `json:"enabled"`
	MinLength int  `json:"minLength"`
}

type ProxyAuthConfig struct {
	Enabled    bool   `json:"enabled"`
	CreateUser bool   `json:"createUser"`
	Header     string `json:"header"`
}

type Recaptcha struct {
	Host   string `json:"host"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// OpenID OAuth2.0
type OidcConfig struct {
	Enabled          bool   `json:"enabled"` // whether to enable OIDC authentication
	CreateUser       bool   `json:"createUser"`
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	AuthorizationUrl string `json:"authorizationUrl"`
	TokenUrl         string `json:"tokenUrl"`
	UserInfoUrl      string `json:"userInfoUrl"`
	Scopes           string `json:"scopes"`         // space separated list of scopes
	UserIdentifier   string `json:"userIdentifier"` // which attribute should be used as the username?
	JwksUrl          string `json:"jwksUrl"`        // currently not used by filebrowser
}
