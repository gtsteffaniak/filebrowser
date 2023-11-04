package settings

import (
	"log"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
)

func TestConfigLoadChanged(t *testing.T) {
	yamlData := loadConfigFile("./testingConfig.yaml")
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	GlobalConfig := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(newConfig, GlobalConfig); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	yamlData := loadConfigFile("./testingConfig.yaml")
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	GlobalConfig := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Auth.Method", GlobalConfig.Auth.Method, newConfig.Auth.Method},
		{"Auth.Method", GlobalConfig.Auth.Method, newConfig.Auth.Method},
		{"Frontend.disableExternal", GlobalConfig.Frontend.DisableExternal, newConfig.Frontend.DisableExternal},
		{"UserDefaults.HideDotfiles", GlobalConfig.UserDefaults.HideDotfiles, newConfig.UserDefaults.HideDotfiles},
		{"Server.Database", GlobalConfig.Server.Database, newConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\n\tGlobalConfig.%s: %v \n\tSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}
