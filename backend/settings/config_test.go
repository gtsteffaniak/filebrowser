package settings

import (
	"log"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
)

func TestConfigLoadChanged(t *testing.T) {
	yamlData := loadConfigFile("./testingConfig.yaml")
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
	yamlData := loadConfigFile("./testingConfig.yaml")
	// Marshal the YAML data to a more human-readable format
	newConfig := setDefaults()
	GlobalConfiguration := setDefaults()

	err := yaml.Unmarshal(yamlData, &newConfig)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Auth.Method", GlobalConfiguration.Auth.Method, newConfig.Auth.Method},
		{"UserDefaults.HideDotfiles", GlobalConfiguration.UserDefaults.HideDotfiles, newConfig.UserDefaults.HideDotfiles},
		{"Server.Database", GlobalConfiguration.Server.Database, newConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\n\tGlobalConfig.%s: %v \n\tSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}

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

func Test_loadConfigFile(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := loadConfigFile(tt.args.configFile); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadConfigFile() = %v, want %v", got, tt.want)
			}
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
			if got := setDefaults(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}
