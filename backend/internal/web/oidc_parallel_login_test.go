package web

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type oidcDiscoveryTransport struct{}

func (oidcDiscoveryTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	const metadata = `{
		"issuer": "https://identity.example.test",
		"authorization_endpoint": "https://identity.example.test/authorize",
		"token_endpoint": "https://identity.example.test/token",
		"jwks_uri": "https://identity.example.test/jwks"
	}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(metadata)),
	}, nil
}

func TestOidcLoginAttemptsDoNotInterfere(t *testing.T) {
	const origin = "https://files.example.test"
	discovery := &http.Client{Transport: oidcDiscoveryTransport{}}
	provider, err := oidc.NewProvider(oidc.ClientContext(context.Background(), discovery), "https://identity.example.test")
	if err != nil {
		t.Fatal(err)
	}
	oldAuth := settings.Config.Auth.Methods.OidcAuth
	oldHTTP := settings.Config.Http
	t.Cleanup(func() {
		settings.Config.Auth.Methods.OidcAuth = oldAuth
		settings.Config.Http = oldHTTP
	})
	settings.Config.Auth.Methods.OidcAuth.Enabled = true
	settings.Config.Auth.Methods.OidcAuth.Provider = provider
	settings.Config.Auth.Methods.OidcAuth.ClientID = "isolated-test"
	settings.Config.Auth.Methods.OidcAuth.Scopes = "openid"
	settings.Config.Http.BaseURL = "/"
	settings.Config.Http.TrustProxyHeaders = false

	for _, order := range []string{"first second", "second first"} {
		t.Run(order, func(t *testing.T) {
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			states := make(map[string]string)
			for _, tab := range []string{"first", "second"} {
				query := url.Values{"redirect": {"/" + tab + "/"}}
				req := httptest.NewRequest(http.MethodGet, origin+"/api/auth/oidc/login?"+query.Encode(), nil)
				res := httptest.NewRecorder()
				if _, err := oidcLoginHandler(res, req, nil); err != nil {
					t.Fatal(err)
				}
				if res.Code != http.StatusFound {
					t.Fatalf("login response = %d", res.Code)
				}
				for _, cookie := range res.Result().Cookies() {
					if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 600 {
						t.Fatalf("unexpected sign-in cookie flags for %s", tab)
					}
				}
				jar.SetCookies(req.URL, res.Result().Cookies())
				location, err := url.Parse(res.Header().Get("Location"))
				if err != nil || location.Query().Get("state") == "" {
					t.Fatal("missing provider state")
				}
				states[tab] = location.Query().Get("state")
			}
			for _, tab := range strings.Fields(order) {
				req := httptest.NewRequest(http.MethodGet, origin+"/api/auth/oidc/callback?state="+url.QueryEscape(states[tab]), nil)
				for _, cookie := range jar.Cookies(req.URL) {
					req.AddCookie(cookie)
				}
				res := httptest.NewRecorder()
				target, status, err := validateOidcCallbackState(req, res)
				if err != nil || target != "/"+tab+"/" {
					t.Fatalf("%s callback status %d, target %q, error %v", tab, status, target, err)
				}
				jar.SetCookies(req.URL, res.Result().Cookies())
			}
		})
	}
}
