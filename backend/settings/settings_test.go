package settings

import (
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
)

func TestConfigLoadChanged(t *testing.T) {
	yamlData, _ := loadConfigFile("./testingConfig.yaml")
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	Config := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Error unmarshaling YAML data: %v", err))
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(newConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	yamlData, _ := loadConfigFile("./testingConfig.yaml")
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	Config := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Error unmarshaling YAML data: %v", err))
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Auth.Method", Config.Auth.Method, newConfig.Auth.Method},
		{"Auth.Method", Config.Auth.Method, newConfig.Auth.Method},
		{"Server.Database", Config.Server.Database, newConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\nConfig.%s: %v \nSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}
