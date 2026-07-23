package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiConfigLoad(t *testing.T) {
	// Setup test directory
	testDir := t.TempDir()

	// Create anchors file that will be referenced
	anchorsConfig := `
# Simple anchors for testing
simple_sources: &simple_sources
  - path: "/srv/files"
    name: "Files"
  - path: "/srv/media"  
    name: "Media"

simple_logging: &simple_logging
  - levels: "info|warning|error"
    output: "stdout"

simple_auth: &simple_auth
  methods:
    password:
      enabled: true
      minLength: 8
`
	// Use pattern-based naming: main config is "config.yaml", so anchors must be "*-config.yaml"
	anchorsPath := filepath.Join(testDir, "definitions-config.yaml")
	if err := os.WriteFile(anchorsPath, []byte(anchorsConfig), 0644); err != nil {
		t.Fatalf("Failed to write anchors config: %v", err)
	}

	// Create main config file that references the anchors
	mainConfig := `
http:
  port: 9000
  baseURL: "/test/"
server:
  minSearchLength: 2
  sources: *simple_sources
  logging: *simple_logging

auth:
  tokenExpirationHours: 3
  <<: *simple_auth

frontend:
  name: "Test FileBrowser"

userDefaults:
  ui:
    darkMode: true
    locale: "en"
  listing:
    viewMode: "grid"
`
	mainPath := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	// Test loading the multi-config setup
	// Use generate=true to skip filesystem validation of fake paths
	err := loadConfigWithDefaults(mainPath, true)
	if err != nil {
		t.Fatalf("Failed to load multi-config: %v", err)
	}

	// Verify that values from different files were loaded correctly
	if Config.Http.Port != 9000 {
		t.Errorf("Expected HTTP port 9000, got %d", Config.Http.Port)
	}

	if Config.Http.BaseURL != "/test/" {
		t.Errorf("Expected baseURL '/test/', got '%s'", Config.Http.BaseURL)
	}

	if Config.Server.MinSearchLength != 2 {
		t.Errorf("Expected minSearchLength 2, got %d", Config.Server.MinSearchLength)
	}

	if Config.Auth.TokenExpirationHours != 3 {
		t.Errorf("Expected token expiration 3 hours, got %d", Config.Auth.TokenExpirationHours)
	}

	if !Config.Auth.Methods.PasswordAuth.Enabled {
		t.Error("Expected password auth to be enabled")
	}

	if Config.Auth.Methods.PasswordAuth.MinLength != 8 {
		t.Errorf("Expected password min length 8, got %d", Config.Auth.Methods.PasswordAuth.MinLength)
	}

	if len(Config.Server.Sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(Config.Server.Sources))
	}

	if Config.Frontend.Name != "Test FileBrowser" {
		t.Errorf("Expected frontend name 'Test FileBrowser', got '%s'", Config.Frontend.Name)
	}

	if Config.UserDefaults.Listing.ViewMode != "grid" {
		t.Errorf("Expected view mode 'grid', got '%s'", Config.UserDefaults.Listing.ViewMode)
	}
}

func TestMultiConfigFallback(t *testing.T) {
	// Setup test directory
	testDir := t.TempDir()

	// Create a simple config file (no anchors)
	simpleConfig := `
http:
  port: 8080
  baseURL: "/"
server:
  sources:
    - path: "/srv"

auth:
  tokenExpirationHours: 2

frontend:
  name: "Simple FileBrowser"
`
	configPath := filepath.Join(testDir, "simple-config.yaml")
	if err := os.WriteFile(configPath, []byte(simpleConfig), 0644); err != nil {
		t.Fatalf("Failed to write simple config: %v", err)
	}

	// Test that fallback works for simple configs
	// Use generate=true to skip filesystem validation
	err := loadConfigWithDefaults(configPath, true)
	if err != nil {
		t.Fatalf("Failed to load simple config: %v", err)
	}

	// Verify values were loaded correctly
	if Config.Http.Port != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", Config.Http.Port)
	}
	if Config.Http.BaseURL != "/" {
		t.Errorf("Expected HTTP baseURL '/', got '%s'", Config.Http.BaseURL)
	}

	if Config.Frontend.Name != "Simple FileBrowser" {
		t.Errorf("Expected frontend name 'Simple FileBrowser', got '%s'", Config.Frontend.Name)
	}
}

func TestMultiConfigWithNestedReferences(t *testing.T) {
	// Note: Logger output may appear during tests - this is normal

	// Setup test directory
	testDir := t.TempDir()

	// Create anchors file with nested structure
	anchorsConfig := `
# Base logging configuration
base_logging: &base_logging
  - levels: "info|warning|error"
    output: "stdout"

# Base permissions  
base_permissions: &base_permissions
  share: false
  admin: false
  api: false
`
	// Use pattern-based naming: main config is "config.yaml", so anchors must be "*-config.yaml"
	anchorsPath := filepath.Join(testDir, "definitions-config.yaml")
	if err := os.WriteFile(anchorsPath, []byte(anchorsConfig), 0644); err != nil {
		t.Fatalf("Failed to write anchors config: %v", err)
	}

	// Create main config that uses references
	mainConfig := `
http:
  port: 8080
server:
  sources:
    - path: "/srv"
  logging: *base_logging

auth:
  tokenExpirationHours: 2

userDefaults:
  ui:
    darkMode: true
    locale: "en"
  listing:
    viewMode: "normal"
  account:
    permissions: *base_permissions

frontend:
  name: "Nested Reference Test"
`
	mainPath := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	// Test loading the nested reference setup
	// Use generate=true to skip filesystem validation
	err := loadConfigWithDefaults(mainPath, true)
	if err != nil {
		t.Fatalf("Failed to load nested config: %v", err)
	}

	// Verify nested references work
	if Config.Http.Port != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", Config.Http.Port)
	}

	if len(Config.Server.Logging) == 0 {
		t.Error("Expected logging configuration to be loaded")
	}

	if Config.Server.Logging[0].Output != "stdout" {
		t.Errorf("Expected logging output 'stdout', got '%s'", Config.Server.Logging[0].Output)
	}

	// Verify user defaults
	if Config.UserDefaults.UI.DarkMode == nil || !*Config.UserDefaults.UI.DarkMode {
		t.Errorf("Expected dark mode true, got %v", Config.UserDefaults.UI.DarkMode)
	}

	if Config.UserDefaults.UI.Locale != "en" {
		t.Errorf("Expected locale 'en', got '%s'", Config.UserDefaults.UI.Locale)
	}

	if Config.UserDefaults.Listing.ViewMode != "normal" {
		t.Errorf("Expected viewMode 'normal', got '%s'", Config.UserDefaults.Listing.ViewMode)
	}

	// Verify permissions were loaded from anchor into account.permissions
	if Config.UserDefaults.Account.Permissions.Admin {
		t.Errorf("Expected admin permission false (from base), got %v", Config.UserDefaults.Account.Permissions.Admin)
	}

	if Config.UserDefaults.Account.Permissions.Share {
		t.Errorf("Expected share permission false (from base), got %v", Config.UserDefaults.Account.Permissions.Share)
	}
}
