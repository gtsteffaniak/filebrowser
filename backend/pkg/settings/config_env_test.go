package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandConfigEnvStrings(t *testing.T) {
	t.Setenv("TEST_EXPAND_ENV_VAR", "from-env")
	in := map[string]interface{}{
		"password": "${TEST_EXPAND_ENV_VAR}",
		"plain":    "no-expansion",
		"nested": map[string]interface{}{
			"value": "$TEST_EXPAND_ENV_VAR",
		},
		"list": []interface{}{"${TEST_EXPAND_ENV_VAR}", "keep"},
		"port": 8080,
		"on":   true,
	}
	expandConfigEnv(in)
	if in["password"] != "from-env" {
		t.Fatalf("password = %#v, want from-env", in["password"])
	}
	if in["plain"] != "no-expansion" {
		t.Fatalf("plain = %#v", in["plain"])
	}
	nested, ok := in["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested = %#v, want map", in["nested"])
	}
	if nested["value"] != "from-env" {
		t.Fatalf("nested.value = %#v", nested["value"])
	}
	list, ok := in["list"].([]interface{})
	if !ok {
		t.Fatalf("list = %#v, want slice", in["list"])
	}
	if list[0] != "from-env" || list[1] != "keep" {
		t.Fatalf("list = %#v", list)
	}
	if in["port"] != 8080 {
		t.Fatalf("port = %#v, want typed int 8080", in["port"])
	}
	if in["on"] != true {
		t.Fatalf("on = %#v, want typed bool true", in["on"])
	}
}

func TestExpandConfigEnvCoercesWholeReference(t *testing.T) {
	t.Setenv("TEST_ENV_PORT", "9090")
	t.Setenv("TEST_ENV_FLAG", "true")
	t.Setenv("TEST_ENV_FLOAT", "1.5")

	in := map[string]interface{}{
		"port": "${TEST_ENV_PORT}",
		"flag": "${TEST_ENV_FLAG}",
		"rate": "${TEST_ENV_FLOAT}",
		// Partial expansion must stay a string (and not coerce "true"-like secrets).
		"msg": "prefix-${TEST_ENV_FLAG}",
	}
	expandConfigEnv(in)

	if in["port"] != int64(9090) {
		t.Fatalf("port = %#v (%T), want int64 9090", in["port"], in["port"])
	}
	if in["flag"] != true {
		t.Fatalf("flag = %#v, want bool true", in["flag"])
	}
	if in["rate"] != 1.5 {
		t.Fatalf("rate = %#v, want float64 1.5", in["rate"])
	}
	if in["msg"] != "prefix-true" {
		t.Fatalf("msg = %#v, want string prefix-true", in["msg"])
	}
}

func TestConfigLoadYamlEnvSubstitution(t *testing.T) {
	testDir := t.TempDir()
	const secret = "ldap-bind-secret"
	t.Setenv("FILEBROWSER_LDAP_USER_PASSWORD", secret)

	configContent := []byte(`
auth:
  methods:
    ldap:
      enabled: true
      server: "ldap://ldap.example.com:389"
      baseDN: "dc=example,dc=com"
      userDN: "cn=admin,dc=example,dc=com"
      userPassword: "${FILEBROWSER_LDAP_USER_PASSWORD}"
server:
  sources:
    - path: "."
`)
	configFile := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(configFile, configContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if err := loadConfigWithDefaults(configFile, true); err != nil {
		t.Fatalf("loadConfigWithDefaults: %v", err)
	}
	if Config.Auth.Methods.LdapAuth.UserPassword != secret {
		t.Fatalf("userPassword = %q, want %q", Config.Auth.Methods.LdapAuth.UserPassword, secret)
	}
}

func TestConfigLoadYamlEnvSpecialCharacters(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{name: "quotes", secret: `p@ss"word'with"quotes`},
		{name: "backslashes", secret: `path\to\secret\and\\more`},
		{name: "multiline", secret: "line1\nline2\nline3"},
		{name: "yaml_special", secret: `a: b # not yaml : [true]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			t.Setenv("FILEBROWSER_LDAP_USER_PASSWORD", tt.secret)

			configContent := []byte(`
auth:
  methods:
    ldap:
      enabled: true
      server: "ldap://ldap.example.com:389"
      baseDN: "dc=example,dc=com"
      userDN: "cn=admin,dc=example,dc=com"
      userPassword: "${FILEBROWSER_LDAP_USER_PASSWORD}"
server:
  sources:
    - path: "."
`)
			configFile := filepath.Join(testDir, "config.yaml")
			if err := os.WriteFile(configFile, configContent, 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			if err := loadConfigWithDefaults(configFile, true); err != nil {
				t.Fatalf("loadConfigWithDefaults: %v", err)
			}
			if Config.Auth.Methods.LdapAuth.UserPassword != tt.secret {
				t.Fatalf("userPassword = %q, want %q", Config.Auth.Methods.LdapAuth.UserPassword, tt.secret)
			}
		})
	}
}

func TestConfigLoadYamlEnvPreservesTypedScalars(t *testing.T) {
	testDir := t.TempDir()
	t.Setenv("TEST_HTTP_PORT", "8443")
	t.Setenv("TEST_TRUST_PROXY", "true")

	configContent := []byte(`
http:
  port: ${TEST_HTTP_PORT}
  trustProxyHeaders: ${TEST_TRUST_PROXY}
auth:
  methods:
    password:
      enabled: true
server:
  sources:
    - path: "."
`)
	configFile := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(configFile, configContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if err := loadConfigWithDefaults(configFile, true); err != nil {
		t.Fatalf("loadConfigWithDefaults: %v", err)
	}
	if Config.Http.Port != 8443 {
		t.Fatalf("http.port = %d, want 8443", Config.Http.Port)
	}
	if !Config.Http.TrustProxyHeaders {
		t.Fatal("expected http.trustProxyHeaders to be true")
	}
}
