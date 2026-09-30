package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandConfigEnv(t *testing.T) {
	in := []byte("password: ${TEST_EXPAND_ENV_VAR}\nplain: no-expansion")
	t.Setenv("TEST_EXPAND_ENV_VAR", "from-env")
	got := expandConfigEnv(in)
	want := []byte("password: from-env\nplain: no-expansion")
	if string(got) != string(want) {
		t.Fatalf("expandConfigEnv() = %q, want %q", got, want)
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
