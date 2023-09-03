package settings

import (
	"log"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
)

func TestConfigLoadChanged(t *testing.T) {
	configYml = "./testingConfig.yaml"
	yamlData := loadConfigFile()
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	GlobalConfiguration := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(newConfig, GlobalConfiguration); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	configYml = "./testingConfig.yaml"
	yamlData := loadConfigFile()
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	GlobalConfiguration := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}

	if GlobalConfiguration.Auth.Method == newConfig.Auth.Method {
		log.Fatalf("Differences should have been found, but were not on Auth method")
	}
	if GlobalConfiguration.UserDefaults.HideDotfiles == newConfig.UserDefaults.HideDotfiles {
		log.Fatalf("Differences should have been found, but were not on Auth method")
	}
}
