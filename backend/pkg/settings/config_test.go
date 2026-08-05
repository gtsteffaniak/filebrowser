package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfigLoadChanged(t *testing.T) {
	// Create isolated test directory
	testDir := t.TempDir()
	validContent, err := os.ReadFile("./validConfig.yaml")
	if err != nil {
		t.Fatalf("failed to read validConfig.yaml: %v", err)
	}
	configFile := filepath.Join(testDir, "config.yaml")
	if err = os.WriteFile(configFile, validContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	defaultConfig := SetDefaults(true)
	err = loadConfigWithDefaults(configFile, true)
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(defaultConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadEnvVars(t *testing.T) {
	// Create isolated test directory
	testDir := t.TempDir()
	validContent, err := os.ReadFile("./validConfig.yaml")
	if err != nil {
		t.Fatalf("failed to read validConfig.yaml: %v", err)
	}
	configFile := filepath.Join(testDir, "config.yaml")
	if err = os.WriteFile(configFile, validContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	defaultConfig := SetDefaults(true)
	expectedKey := "MYKEY"
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", expectedKey)
	err = loadConfigWithDefaults(configFile, true)
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	if Config.Integrations.OnlyOffice.Secret != expectedKey {
		t.Errorf("Expected OnlyOffice.Secret to be '%v', got '%s'", expectedKey, Config.Integrations.OnlyOffice.Secret)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(defaultConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadHttpSection(t *testing.T) {
	testDir := t.TempDir()
	configContent := []byte(`
server:
  sources:
    - path: "."
http:
  trustProxyHeaders: true
  disableRateLimit: true
`)
	configFile := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(configFile, configContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if err := loadConfigWithDefaults(configFile, true); err != nil {
		t.Fatalf("error loading config file: %v", err)
	}

	if !Config.Http.TrustProxyHeaders {
		t.Fatal("expected http.trustProxyHeaders to be true")
	}
	if !Config.Http.DisableRateLimit {
		t.Fatal("expected http.disableRateLimit to be true")
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	// Create isolated test directory
	testDir := t.TempDir()
	validContent, err := os.ReadFile("./validConfig.yaml")
	if err != nil {
		t.Fatalf("failed to read validConfig.yaml: %v", err)
	}
	configFile := filepath.Join(testDir, "config.yaml")
	if err = os.WriteFile(configFile, validContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	defaultConfig := SetDefaults(true)
	err = loadConfigWithDefaults(configFile, true)
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Server.DatabaseV2", Config.Server.DatabaseV2, defaultConfig.Server.DatabaseV2},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\nConfig.%s: %v \nSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}

func TestNeedsSubpathTrustProxyHeadersWarning(t *testing.T) {
	if !needsSubpathTrustProxyHeadersWarning("/files/", false) {
		t.Fatal("expected warning for subpath without trustProxyHeaders")
	}
	if needsSubpathTrustProxyHeadersWarning("/", false) {
		t.Fatal("expected no warning for root baseURL")
	}
	if needsSubpathTrustProxyHeadersWarning("/files/", true) {
		t.Fatal("expected no warning when trustProxyHeaders is true")
	}
}

func TestInvalidConfig(t *testing.T) {
	// Create isolated test directory
	testDir := t.TempDir()
	invalidContent, err := os.ReadFile("./invalidConfig.yaml")
	if err != nil {
		t.Fatalf("failed to read invalidConfig.yaml: %v", err)
	}
	configFile := filepath.Join(testDir, "config.yaml")
	if err = os.WriteFile(configFile, invalidContent, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err = loadConfigWithDefaults(configFile, true)
	// Config loads successfully but validation should catch missing sources
	if err == nil {
		err = ValidateConfig(Config)
		if err == nil {
			t.Fatal("expected validation error for config with missing required sources, got nil")
		}
	}
}
