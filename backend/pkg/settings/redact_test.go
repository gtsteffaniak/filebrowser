package settings

import (
	"encoding/json"
	"os"
	"reflect"
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

// TestRedactedSettingsCopy_marshalsWithoutPlaintextSecrets is the regression guard for
// GHSA-892f-9h5p-mfc8: GET /api/settings returned cleartext secrets.
func TestRedactedSettingsCopy_marshalsWithoutPlaintextSecrets(t *testing.T) {
	setSecretSentinels(t)

	out, err := json.Marshal(RedactedSettingsCopy(&Config))
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

// TestRedactedSettingsCopy_doesNotMutateLiveConfig is the regression guard for the live-config
// mutation that broke JWT signing and the integrations after the first GET /api/settings.
func TestRedactedSettingsCopy_doesNotMutateLiveConfig(t *testing.T) {
	setSecretSentinels(t)

	_ = RedactedSettingsCopy(&Config)

	if Config.Auth.Key != "SENTINEL-AUTH-KEY" {
		t.Fatalf("live config auth key was mutated: %q", Config.Auth.Key)
	}
	if Config.Auth.TotpSecret != "SENTINEL-TOTP-SECRET" {
		t.Fatalf("live config totp secret was mutated: %q", Config.Auth.TotpSecret)
	}
	if Config.Auth.Methods.PasswordAuth.Recaptcha.Secret != "SENTINEL-RECAPTCHA-SECRET" {
		t.Fatalf("live recaptcha secret was mutated: %q", Config.Auth.Methods.PasswordAuth.Recaptcha.Secret)
	}
	if Config.Auth.Methods.LdapAuth.UserPassword != "SENTINEL-LDAP-PASSWORD" {
		t.Fatalf("live LDAP password was mutated: %q", Config.Auth.Methods.LdapAuth.UserPassword)
	}
	// The deprecated top-level password must survive too, since redaction of the copy must not
	// alias back into the source struct.
	if Config.Auth.AdminPassword != "SENTINEL-LEGACY-ADMIN-PW" {
		t.Fatalf("live legacy admin password was mutated: %q", Config.Auth.AdminPassword)
	}
}

// TestRedactedSettingsCopy_isFaithfulDeepCopy proves the reflection copy reproduces the whole
// config graph (every field type) before redaction, so no field is silently dropped.
func TestRedactedSettingsCopy_isFaithfulDeepCopy(t *testing.T) {
	prev := Config
	t.Cleanup(func() { Config = prev })
	Config = SetDefaults(false)
	Config.Integrations.OnlyOffice.Secret = "" // keep secrets empty so redaction is a no-op
	Config.Auth.Key = ""
	Config.Auth.TotpSecret = ""
	Config.Auth.AdminPassword = ""
	Config.Auth.AdminUsername = ""
	Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	Config.Auth.Methods.PasswordAuth.AdminUsername = ""
	Config.Auth.Methods.OidcAuth.ClientID = ""
	Config.Auth.Methods.OidcAuth.ClientSecret = ""
	Config.Auth.Methods.LdapAuth.UserPassword = ""
	Config.Auth.Methods.JwtAuth.Secret = ""
	Config.Auth.Methods.PasswordAuth.Recaptcha.Secret = ""
	Config.Integrations.Media.Convert.ImagePreview[HEICImagePreview] = boolPtr(false)
	Config.Server.Sources = []*Source{{Path: "/srv", Name: "srv"}}

	cp := RedactedSettingsCopy(&Config)

	if cp == &Config {
		t.Fatal("expected a new Settings, got the same pointer")
	}
	if !reflect.DeepEqual(cp, &Config) {
		t.Fatal("deep copy does not match the source config")
	}
}

// TestRedactedSettingsCopy_isDeepCopy verifies nested maps/slices are copied, not aliased, so a
// caller cannot corrupt the live config by mutating the returned copy.
func TestRedactedSettingsCopy_isDeepCopy(t *testing.T) {
	prev := Config
	t.Cleanup(func() { Config = prev })
	Config = SetDefaults(false)
	Config.Server.Sources = []*Source{{Path: "/srv", Name: "original"}}
	Config.Frontend.Styling.CustomThemes = map[string]CustomTheme{"a": {Description: "original"}}
	Config.Server.SourceMap = map[string]*Source{"/srv": {Path: "/srv", Name: "original"}}

	cp := RedactedSettingsCopy(&Config)

	cp.Server.Sources[0].Name = "changed"
	cp.Server.SourceMap["/srv"].Name = "changed"
	cp.Frontend.Styling.CustomThemes["a"] = CustomTheme{Description: "changed"}

	if Config.Server.Sources[0].Name != "original" {
		t.Fatalf("source slice aliased the live config: %q", Config.Server.Sources[0].Name)
	}
	if Config.Server.SourceMap["/srv"].Name != "original" {
		t.Fatalf("source map aliased the live config: %q", Config.Server.SourceMap["/srv"].Name)
	}
	if Config.Frontend.Styling.CustomThemes["a"].Description != "original" {
		t.Fatalf("custom themes map aliased the live config: %q", Config.Frontend.Styling.CustomThemes["a"].Description)
	}
}

func TestRedactedSettingsCopy_preservesNonSecretValues(t *testing.T) {
	setSecretSentinels(t)
	Config.Frontend.Name = "SENTINEL-NOT-A-SECRET"
	Config.Http.Port = 8123
	Config.Server.SearchResultsLimit = 4242

	out, err := json.Marshal(RedactedSettingsCopy(&Config))
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

// TestRedactedSettingsCopy_emptySecretsStayEmpty ensures an unset secret does not appear to be set.
func TestRedactedSettingsCopy_emptySecretsStayEmpty(t *testing.T) {
	prev := Config
	t.Cleanup(func() { Config = prev })
	Config = SetDefaults(false)
	Config.Auth.Key = ""
	Config.Auth.TotpSecret = ""

	out, err := json.Marshal(RedactedSettingsCopy(&Config))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Settings
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
