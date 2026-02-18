package settings

import (
	"strings"
	"testing"
)

func TestParseLdapServer(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		wantScheme string
		wantHost  string
		wantPort  int
		wantErr   bool
	}{
		{
			name:       "ldap with host and port",
			server:     "ldap://localhost:389",
			wantScheme: "ldap",
			wantHost:   "localhost",
			wantPort:   389,
			wantErr:    false,
		},
		{
			name:       "ldaps with host and port",
			server:     "ldaps://ldap.example.com:636",
			wantScheme: "ldaps",
			wantHost:   "ldap.example.com",
			wantPort:   636,
			wantErr:    false,
		},
		{
			name:       "ldap with trailing slash",
			server:     "ldap://host:389/",
			wantScheme: "ldap",
			wantHost:   "host",
			wantPort:   389,
			wantErr:    false,
		},
		{
			name:       "ldap port not provided defaults to 389",
			server:     "ldap://localhost",
			wantScheme: "ldap",
			wantHost:   "localhost",
			wantPort:   389,
			wantErr:    false,
		},
		{
			name:       "ldaps port not provided defaults to 636",
			server:     "ldaps://ldap.example.com",
			wantScheme: "ldaps",
			wantHost:   "ldap.example.com",
			wantPort:   636,
			wantErr:    false,
		},
		{
			name:     "port provided but invalid returns error",
			server:   "ldap://host:bad",
			wantErr:  true,
		},
		{
			name:     "port provided but invalid (ldaps) returns error",
			server:   "ldaps://host:xyz",
			wantErr:  true,
		},
		{
			name:     "port zero returns error",
			server:   "ldap://host:0",
			wantErr:  true,
		},
		{
			name:     "empty server",
			server:   "",
			wantErr:  true,
		},
		{
			name:     "no scheme",
			server:   "localhost:389",
			wantErr:  true,
		},
		{
			name:     "invalid scheme",
			server:   "http://localhost:389",
			wantErr:  true,
		},
		{
			name:     "invalid scheme ftp",
			server:   "ftp://localhost:389",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScheme, gotHost, gotPort, err := parseLdapServer(tt.server)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLdapServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotScheme != tt.wantScheme {
					t.Errorf("parseLdapServer() scheme = %v, want %v", gotScheme, tt.wantScheme)
				}
				if gotHost != tt.wantHost {
					t.Errorf("parseLdapServer() host = %v, want %v", gotHost, tt.wantHost)
				}
				if gotPort != tt.wantPort {
					t.Errorf("parseLdapServer() port = %v, want %v", gotPort, tt.wantPort)
				}
			}
		})
	}
}

func TestVerifyLdapConnection_ValidationOnly(t *testing.T) {
	// Save and restore global config so we don't affect other tests
	saved := Config.Auth.Methods.LdapAuth
	defer func() { Config.Auth.Methods.LdapAuth = saved }()

	t.Run("empty server returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled: true,
			Server:  "",
			BaseDN:  "dc=test,dc=local",
		}
		err := verifyLdapConnection()
		if err == nil {
			t.Fatal("verifyLdapConnection() expected error for empty server")
		}
		if !strings.Contains(err.Error(), "LDAP server is required") {
			t.Errorf("verifyLdapConnection() error = %v, want message containing 'LDAP server is required'", err)
		}
	})

	t.Run("empty baseDN returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled: true,
			Server:  "ldap://localhost:389",
			Scheme:  "ldap",
			Host:    "localhost",
			Port:    389,
			BaseDN:  "",
		}
		err := verifyLdapConnection()
		if err == nil {
			t.Fatal("verifyLdapConnection() expected error for empty baseDN")
		}
		if !strings.Contains(err.Error(), "baseDN is required") {
			t.Errorf("verifyLdapConnection() error = %v, want message containing 'baseDN is required'", err)
		}
	})
}

func TestValidateLdapAuth_ConfigValidation(t *testing.T) {
	saved := Config.Auth.Methods.LdapAuth
	defer func() { Config.Auth.Methods.LdapAuth = saved }()

	t.Run("empty server returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled:  true,
			Server:   "",
			BaseDN:   "dc=test,dc=local",
			UserDN:   "cn=admin,dc=test,dc=local",
		}
		err := ValidateLdapAuth()
		if err == nil {
			t.Fatal("ValidateLdapAuth() expected error for empty server")
		}
		if !strings.Contains(err.Error(), "server is required") {
			t.Errorf("ValidateLdapAuth() error = %v", err)
		}
	})

	t.Run("invalid server URL returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled:  true,
			Server:   "not-a-valid-ldap-url",
			BaseDN:   "dc=test,dc=local",
			UserDN:   "cn=admin,dc=test,dc=local",
		}
		err := ValidateLdapAuth()
		if err == nil {
			t.Fatal("ValidateLdapAuth() expected error for invalid server URL")
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("ValidateLdapAuth() error = %v", err)
		}
	})

	t.Run("empty baseDN returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled:  true,
			Server:   "ldap://localhost:389",
			BaseDN:   "",
			UserDN:   "cn=admin,dc=test,dc=local",
		}
		err := ValidateLdapAuth()
		if err == nil {
			t.Fatal("ValidateLdapAuth() expected error for empty baseDN")
		}
		if !strings.Contains(err.Error(), "baseDN is required") {
			t.Errorf("ValidateLdapAuth() error = %v", err)
		}
	})

	t.Run("empty userDN returns error", func(t *testing.T) {
		Config.Auth.Methods.LdapAuth = LdapConfig{
			Enabled:  true,
			Server:   "ldap://localhost:389",
			BaseDN:   "dc=test,dc=local",
			UserDN:   "",
		}
		err := ValidateLdapAuth()
		if err == nil {
			t.Fatal("ValidateLdapAuth() expected error for empty userDN")
		}
		if !strings.Contains(err.Error(), "userDN") {
			t.Errorf("ValidateLdapAuth() error = %v", err)
		}
	})
}
