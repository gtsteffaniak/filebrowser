package settings

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// secretSentinelValues are deliberately unique so a surviving sentinel in the marshaled
// output is unambiguously a leak.
func setSecretSentinels(t *testing.T) {
	t.Helper()
	prev := Config
	t.Cleanup(func() { Config = prev })

	Config.Auth.Key = "SENTINEL-AUTH-KEY"
	Config.Auth.TotpSecret = "SENTINEL-TOTP-SECRET"
	Config.Auth.AdminUsername = "SENTINEL-LEGACY-ADMIN-USER"
	Config.Auth.AdminPassword = "SENTINEL-LEGACY-ADMIN-PW"
	Config.Auth.Methods.PasswordAuth.AdminUsername = "SENTINEL-ADMIN-USER"
	Config.Auth.Methods.PasswordAuth.AdminPassword = "SENTINEL-ADMIN-PW"
	Config.Auth.Methods.PasswordAuth.Recaptcha.Secret = "SENTINEL-RECAPTCHA-SECRET"
	Config.Auth.Methods.OidcAuth.ClientID = "SENTINEL-OIDC-CLIENT-ID"
	Config.Auth.Methods.OidcAuth.ClientSecret = "SENTINEL-OIDC-CLIENT-SECRET"
	Config.Auth.Methods.LdapAuth.UserPassword = "SENTINEL-LDAP-PASSWORD"
	Config.Auth.Methods.JwtAuth.Secret = "SENTINEL-JWT-SECRET"
	Config.Integrations.OnlyOffice.Secret = "SENTINEL-OFFICE-SECRET"
}

// TestRedactSettingsSecrets_marshalsWithoutPlaintextSecrets is the regression guard for
// GHSA-892f-9h5p-mfc8: GET /api/settings returned cleartext secrets.
func TestRedactSettingsSecrets_marshalsWithoutPlaintextSecrets(t *testing.T) {
	setSecretSentinels(t)

	RedactSettingsSecrets(&Config)

	out, err := json.Marshal(&Config)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "SENTINEL-") {
		t.Fatalf("redacted settings still contain a plaintext secret: %s", out)
	}

	// The placeholder must be present so an operator can tell a secret is configured.
	if !strings.Contains(string(out), redactPlaceholder) {
		t.Fatalf("expected redaction placeholder %q in output", redactPlaceholder)
	}
}

func TestRedactSettingsSecrets_preservesNonSecretValues(t *testing.T) {
	setSecretSentinels(t)
	Config.Frontend.Name = "SENTINEL-NOT-A-SECRET"
	Config.Http.Port = 8123
	Config.Server.SearchResultsLimit = 4242

	RedactSettingsSecrets(&Config)

	out, err := json.Marshal(&Config)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Settings
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Frontend.Name != "SENTINEL-NOT-A-SECRET" {
		t.Fatalf("frontend name was redacted: %q", got.Frontend.Name)
	}
	if got.Http.Port != 8123 {
		t.Fatalf("http port was altered: %d", got.Http.Port)
	}
	if got.Server.SearchResultsLimit != 4242 {
		t.Fatalf("search limit was altered: %d", got.Server.SearchResultsLimit)
	}
	// A non-secret field in a struct that also holds secrets must survive: the OIDC/recaptcha
	// containers are the easiest place to over-redact.
	if got.Auth.Methods.OidcAuth.ClientID != redactPlaceholder {
		t.Fatalf("expected OIDC clientId redacted, got %q", got.Auth.Methods.OidcAuth.ClientID)
	}
	if got.Auth.Methods.PasswordAuth.Recaptcha.Host == redactPlaceholder {
		t.Fatal("recaptcha host must not be redacted (it is a public URL)")
	}
}

// TestRedactSettingsSecrets_emptySecretsStayEmpty ensures an unset secret does not appear to be set.
func TestRedactSettingsSecrets_emptySecretsStayEmpty(t *testing.T) {
	prev := Config
	t.Cleanup(func() { Config = prev })
	Config.Auth.Key = ""
	Config.Auth.TotpSecret = ""

	RedactSettingsSecrets(&Config)

	var got Settings
	out, err := json.Marshal(&Config)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Auth.Key != "" || got.Auth.TotpSecret != "" {
		t.Fatalf("empty secrets should stay empty, got key=%q totp=%q", got.Auth.Key, got.Auth.TotpSecret)
	}
}

// TestRedactAuthSecrets_doesNotMutateConfig verifies the by-value helper leaves the live config alone.
func TestRedactAuthSecrets_doesNotMutateConfig(t *testing.T) {
	setSecretSentinels(t)

	redacted := RedactAuthSecrets(Config.Auth)

	if redacted.Key != redactPlaceholder {
		t.Fatalf("expected redacted key, got %q", redacted.Key)
	}
	if Config.Auth.Key != "SENTINEL-AUTH-KEY" {
		t.Fatalf("live config auth key was mutated: %q", Config.Auth.Key)
	}
}

// TestSecretMarkers_drive_yaml_redaction guards the marker gaps fixed for GHSA-892f-9h5p-mfc8:
// the YAML export redacts recaptcha.secret and ldap.userPassword, which previously leaked
// because their Go comments lacked the `secret:` marker the generator looks for.
func TestSecretMarkers_drive_yaml_redaction(t *testing.T) {
	embedded, err := os.ReadFile("../../../frontend/public/config.generated.yaml")
	if err != nil {
		t.Fatalf("read generated yaml: %v", err)
	}

	prev := Config
	t.Cleanup(func() { Config = prev })
	Config = SetDefaults(false)
	Config.Auth.Key = "SENTINEL-AUTH-KEY"
	Config.Auth.TotpSecret = "SENTINEL-TOTP"
	Config.Auth.AdminPassword = "SENTINEL-LEGACY-ADMIN-PW"
	Config.Auth.AdminUsername = "SENTINEL-LEGACY-ADMIN-USER"
	Config.Auth.Methods.PasswordAuth.AdminPassword = "SENTINEL-NESTED-ADMIN-PW"
	Config.Auth.Methods.PasswordAuth.Recaptcha.Secret = "SENTINEL-RECAPTCHA"
	Config.Auth.Methods.OidcAuth.ClientSecret = "SENTINEL-OIDC"
	Config.Auth.Methods.LdapAuth.UserPassword = "SENTINEL-LDAP"
	Config.Auth.Methods.JwtAuth.Secret = "SENTINEL-JWT"
	Config.Integrations.OnlyOffice.Secret = "SENTINEL-OFFICE"

	out, err := GenerateConfigYamlWithEmbedded(&Config, false, true, false, string(embedded))
	if err != nil {
		t.Fatalf("generate yaml: %v", err)
	}
	for _, sentinel := range []string{
		"SENTINEL-AUTH-KEY",
		"SENTINEL-TOTP",
		"SENTINEL-LEGACY-ADMIN-PW",
		"SENTINEL-LEGACY-ADMIN-USER",
		"SENTINEL-NESTED-ADMIN-PW",
		"SENTINEL-RECAPTCHA",
		"SENTINEL-OIDC",
		"SENTINEL-LDAP",
		"SENTINEL-JWT",
		"SENTINEL-OFFICE",
	} {
		if strings.Contains(out, sentinel) {
			t.Errorf("YAML export leaked %s", sentinel)
		}
	}
}

// TestTestSecretFieldNames_covers_yaml_marker_gaps guards the two fields whose Go comments lacked
// the "secret:" marker (recaptcha secret, LDAP bind password), plus the deprecated top-level
// auth fields that the filtered generated YAML omits entirely.
func TestSecretFieldNames_covers_yaml_marker_gaps(t *testing.T) {
	secrets := ActiveSecretFields()
	for _, tc := range []struct{ typ, field string }{
		{"Auth", "Key"},
		{"Auth", "TotpSecret"},
		{"Auth", "AdminPassword"},
		{"PasswordAuthConfig", "AdminPassword"},
		{"Recaptcha", "Secret"},
		{"OidcConfig", "ClientSecret"},
		{"JwtAuthConfig", "Secret"},
		{"LdapConfig", "UserPassword"},
		{"OnlyOffice", "Secret"},
	} {
		if !secrets[tc.typ][tc.field] {
			t.Errorf("%s.%s is not marked secret", tc.typ, tc.field)
		}
	}
}
