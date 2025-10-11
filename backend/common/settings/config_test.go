package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInitialize(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Initialize(tt.args.configFile)
		})
	}
}

func Test_setDefaults(t *testing.T) {
	tests := []struct {
		name string
		want Settings
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setDefaults(true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

	defaultConfig := setDefaults(true)
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

	defaultConfig := setDefaults(true)
	expectedKey := "MYKEY"
	// mock environment variables
	os.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", expectedKey)
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

	defaultConfig := setDefaults(true)
	err = loadConfigWithDefaults(configFile, true)
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Server.Database", Config.Server.Database, defaultConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\nConfig.%s: %v \nSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
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
