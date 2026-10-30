package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const settingsRedactSentinel = "SENTINEL-SETTINGS-SECRET"

// TestSettingsGetHandler_redactsSecrets is the end-to-end regression guard for
// GHSA-892f-9h5p-mfc8: GET /api/settings (and ?property=auth) must not return cleartext secrets.
func TestSettingsGetHandler_redactsSecrets(t *testing.T) {
	setupTestEnv(t)

	prev := settings.Config
	t.Cleanup(func() { settings.Config = prev })
	settings.Config.Auth.Key = settingsRedactSentinel
	settings.Config.Auth.TotpSecret = settingsRedactSentinel
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = settingsRedactSentinel
	settings.Config.Auth.Methods.PasswordAuth.Recaptcha.Secret = settingsRedactSentinel
	settings.Config.Auth.Methods.OidcAuth.ClientSecret = settingsRedactSentinel
	settings.Config.Auth.Methods.LdapAuth.UserPassword = settingsRedactSentinel
	settings.Config.Auth.Methods.JwtAuth.Secret = settingsRedactSentinel
	settings.Config.Integrations.OnlyOffice.Secret = settingsRedactSentinel

	admin := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "settings_redact_admin",
			Permissions: users.Permissions{Admin: true},
		},
	}

	for _, path := range []string{"/api/settings", "/api/settings?property=auth"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			status, err := settingsGetHandler(rec, req, &Context{User: admin})
			if status != http.StatusOK {
				t.Fatalf("status=%d err=%v want 200", status, err)
			}
			body := rec.Body.String()
			if strings.Contains(body, settingsRedactSentinel) {
				t.Fatalf("response for %s leaked a plaintext secret: %s", path, body)
			}
			if !json.Valid([]byte(body)) {
				t.Fatalf("response for %s is not valid JSON: %s", path, body)
			}
		})
	}
}

// TestSettingsGetHandler_redactsAuthKeyOnlyWherePresent confirms non-secret auth fields still
// round-trip so the response stays usable for the UI.
func TestSettingsGetHandler_keepsNonSecretAuthFields(t *testing.T) {
	setupTestEnv(t)

	prev := settings.Config
	t.Cleanup(func() { settings.Config = prev })
	settings.Config.Auth.Key = settingsRedactSentinel
	settings.Config.Auth.TokenExpirationHours = 7

	admin := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "settings_redact_admin2",
			Permissions: users.Permissions{Admin: true},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings?property=auth", nil)
	rec := httptest.NewRecorder()
	status, err := settingsGetHandler(rec, req, &Context{User: admin})
	if status != http.StatusOK {
		t.Fatalf("status=%d err=%v want 200", status, err)
	}

	var got settings.Auth
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal auth: %v", err)
	}
	if got.TokenExpirationHours != 7 {
		t.Fatalf("tokenExpirationHours=%d want 7", got.TokenExpirationHours)
	}
	if got.Key == settingsRedactSentinel {
		t.Fatal("auth key was returned in cleartext")
	}
}
